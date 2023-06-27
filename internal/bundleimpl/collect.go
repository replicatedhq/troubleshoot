package bundleimpl

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/bundle"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/supportbundle"
	"github.com/replicatedhq/troubleshoot/pkg/version"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"k8s.io/client-go/kubernetes"
)

func (b *Bundle) doCollect(
	ctx context.Context, opt bundle.CollectOptions,
) (collect.CollectorResult, error) {
	ctxWrap, root := otel.Tracer(constants.LIB_TRACER_NAME).Start(
		ctx, constants.TROUBLESHOOT_ROOT_SPAN_NAME,
	)
	defer func() {
		// If this function returns an error, root.End() may not be called.
		// We want to ensure this happens, so we defer it. It is safe to call
		// root.End() multiple times.
		root.End()
	}()

	allResults := collect.NewResult()

	sbSpec := concatSpecs(opt.Specs.SupportBundlesV1Beta2...)

	collectedResults, err := b.collectFromHost(ctxWrap, sbSpec.Spec.HostCollectors, opt.BundleDir, b.progressChan)
	if err != nil {
		return nil, errors.Wrap(err, "failed to collect bundle from host")
	}
	allResults.AddResult(collectedResults)

	collectedResults, err = b.collectFromCluster(ctxWrap, sbSpec.Spec.Collectors, opt.BundleDir, opt.Namespace)
	if err != nil {
		return nil, errors.Wrap(err, "failed to collect bundle from cluster")
	}
	allResults.AddResult(collectedResults)

	return collect.CollectorResult{}, nil
}

func (b *Bundle) collectFromCluster(
	ctx context.Context, collectors []*troubleshootv1beta2.Collect, bundlePath string, namespace string,
) (collect.CollectorResult, error) {
	restConfig, err := k8sutil.GetRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	// TODO: Duplicate of runCollectors from pkg/supportbundle/collect.go. DRY me up!
	var allCollectors []collect.Collector
	var foundForbidden bool

	collectSpecs := make([]*troubleshootv1beta2.Collect, 0)
	collectSpecs = append(collectSpecs, collectors...)
	collectSpecs = collect.EnsureCollectorInList(collectSpecs, troubleshootv1beta2.Collect{ClusterInfo: &troubleshootv1beta2.ClusterInfo{}})
	collectSpecs = collect.EnsureCollectorInList(collectSpecs, troubleshootv1beta2.Collect{ClusterResources: &troubleshootv1beta2.ClusterResources{}})
	collectSpecs = collect.DedupCollectors(collectSpecs)
	collectSpecs = collect.EnsureClusterResourcesFirst(collectSpecs)

	restConfig.QPS = constants.DEFAULT_CLIENT_QPS
	restConfig.Burst = constants.DEFAULT_CLIENT_BURST
	restConfig.UserAgent = fmt.Sprintf("%s/%s", constants.DEFAULT_CLIENT_USER_AGENT, version.Version())

	k8sClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to instantiate Kubernetes client")
	}

	allCollectorsMap := make(map[reflect.Type][]collect.Collector)
	collectResult := collect.NewResult()

	// TODO: How should we handle passing configuration options (cli flags, env vars) to collectors?
	// Using configuration to drive decisions is not a public API concern, its an implementators one.
	withoutPerms := viper.GetBool("collect-without-permissions")
	var sinceTime *time.Time // This is a pod logs only option. It shouldn't pollute the public API.
	if viper.GetString("since-time") != "" || viper.GetString("since") != "" {
		sinceTime, err = parseTimeFlags(viper.GetViper())
		if err != nil {
			return nil, errors.Wrap(err, "failed parse since time")
		}
	}

	for _, desiredCollector := range collectSpecs {
		if collectorInterface, ok := collect.GetCollector(desiredCollector, bundlePath, namespace, restConfig, k8sClient, sinceTime); ok {
			if collector, ok := collectorInterface.(collect.Collector); ok {
				err := collector.CheckRBAC(ctx, collector, desiredCollector, restConfig, namespace)
				if err != nil {
					return nil, errors.Wrap(err, "failed to check RBAC for collectors")
				}
				collectorType := reflect.TypeOf(collector)
				allCollectorsMap[collectorType] = append(allCollectorsMap[collectorType], collector)
			}
		}
	}

	for _, collectors := range allCollectorsMap {
		if mergeCollector, ok := collectors[0].(collect.MergeableCollector); ok {
			mergedCollectors, err := mergeCollector.Merge(collectors)
			if err != nil {
				msg := fmt.Sprintf("failed to merge collector: %s: %s", mergeCollector.Title(), err)
				b.progressChan <- msg
			}
			allCollectors = append(allCollectors, mergedCollectors...)
		} else {
			allCollectors = append(allCollectors, collectors...)
		}

		foundForbidden = false
		for _, collector := range collectors {
			for _, e := range collector.GetRBACErrors() {
				foundForbidden = true
				b.progressChan <- e
			}
		}
	}

	// if foundForbidden && !opts.CollectWithoutPermissions {
	if foundForbidden && !withoutPerms {
		return nil, errors.New("insufficient permissions to run all collectors")
	}

	for _, collector := range allCollectors {
		_, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, collector.Title())
		span.SetAttributes(attribute.String("type", reflect.TypeOf(collector).String()))

		isExcluded, _ := collector.IsExcluded()
		if isExcluded {
			msg := fmt.Sprintf("excluding %q collector", collector.Title())
			b.progressChan <- msg
			span.SetAttributes(attribute.Bool(constants.EXCLUDED, true))
			span.End()
			continue
		}

		// skip collectors with RBAC errors unless its the ClusterResources collector
		if collector.HasRBACErrors() {
			if _, ok := collector.(*collect.CollectClusterResources); !ok {
				msg := fmt.Sprintf("skipping collector %q with insufficient RBAC permissions", collector.Title())
				b.progressChan <- msg
				span.SetStatus(codes.Error, "skipping collector, insufficient RBAC permissions")
				span.End()
				continue
			}
		}

		b.progressChan <- collector.Title()
		result, err := collector.Collect(b.progressChan)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			b.progressChan <- errors.Errorf("failed to run collector: %s: %v", collector.Title(), err)
		}

		for k, v := range result {
			collectResult[k] = v
		}
		span.End()
	}

	return collectResult, nil
}

func (b *Bundle) collectFromHost(
	ctx context.Context, collectSpecs []*troubleshootv1beta2.HostCollect, bundlePath string, progressChan chan any,
) (collect.CollectorResult, error) {
	// TODO: Duplicate of runHostCollectors from pkg/supportbundle/collect.go. DRY me up!
	collectResult := collect.NewResult()

	var collectors []collect.HostCollector
	for _, desiredCollector := range collectSpecs {
		collector, ok := collect.GetHostCollector(desiredCollector, bundlePath)
		if ok {
			collectors = append(collectors, collector)
		}
	}

	for _, collector := range collectors {
		// TODO: Add context to host collectors
		_, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, collector.Title())
		span.SetAttributes(attribute.String("type", reflect.TypeOf(collector).String()))

		isExcluded, _ := collector.IsExcluded()
		if isExcluded {
			progressChan <- fmt.Sprintf("[%s] Excluding host collector", collector.Title())
			span.SetAttributes(attribute.Bool(constants.EXCLUDED, true))
			span.End()
			continue
		}

		progressChan <- fmt.Sprintf("[%s] Running host collector...", collector.Title())
		// TODO: Convert return type to CollectorResult
		result, err := collector.Collect(progressChan)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			progressChan <- errors.Errorf("failed to run host collector: %s: %v", collector.Title(), err)
		}
		span.End()
		for k, v := range result {
			collectResult[k] = v
		}
	}

	return collectResult, nil
}

func concatSpecs(specs ...troubleshootv1beta2.SupportBundle) *troubleshootv1beta2.SupportBundle {
	target := &troubleshootv1beta2.SupportBundle{}
	for _, s := range specs {
		target = supportbundle.ConcatSpec(target, &s)
	}
	return target
}

func parseTimeFlags(v *viper.Viper) (*time.Time, error) {
	// TODO: Copied from cmd/troubleshoot/cli/run.go. DRY me up!
	var (
		sinceTime time.Time
		err       error
	)
	if v.GetString("since-time") != "" {
		if v.GetString("since") != "" {
			return nil, errors.Errorf("at most one of `sinceTime` or `since` may be specified")
		}
		sinceTime, err = time.Parse(time.RFC3339, v.GetString("since-time"))
		if err != nil {
			return nil, errors.Wrap(err, "unable to parse --since-time flag")
		}
	} else {
		parsedDuration, err := time.ParseDuration(v.GetString("since"))
		if err != nil {
			return nil, errors.Wrap(err, "unable to parse --since flag")
		}
		now := time.Now()
		sinceTime = now.Add(0 - parsedDuration)
	}

	return &sinceTime, nil
}
