package supportbundle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"time"

	"github.com/pkg/errors"
	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/convert"
	"github.com/replicatedhq/troubleshoot/pkg/version"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"k8s.io/client-go/kubernetes"
)

type FilteredCollector struct {
	Spec      troubleshootv1beta2.HostCollect
	Collector collect.HostCollector
}

func runHostCollectors(ctx context.Context, hostCollectors []*troubleshootv1beta2.HostCollect, additionalRedactors *troubleshootv1beta2.Redactor, bundlePath string, opts SupportBundleCreateOpts) (collect.CollectorResult, error) {
	collectSpecs := append([]*troubleshootv1beta2.HostCollect{}, hostCollectors...)
	collectedData := make(map[string][]byte)

	// Filter out excluded collectors
	filteredCollectors, err := filterHostCollectors(ctx, collectSpecs, bundlePath, opts)
	if err != nil {
		return nil, err
	}

	if opts.RunHostCollectorsInPod {
		if err := checkRemoteCollectorRBAC(ctx, opts.KubernetesRestConfig, "Remote Host Collectors", opts.Namespace); err != nil {
			if rbacErr, ok := err.(*RBACPermissionError); ok {
				for _, forbiddenErr := range rbacErr.Forbidden {
					opts.ProgressChan <- forbiddenErr
				}

				if !opts.CollectWithoutPermissions {
					return nil, collect.ErrInsufficientPermissionsToRun
				}
			} else {
				return nil, err
			}
		}
		if err := collectRemoteHost(ctx, filteredCollectors, bundlePath, opts, collectedData); err != nil {
			return nil, err
		}
	} else {
		if err := collectHost(ctx, filteredCollectors, opts, collectedData); err != nil {
			return nil, err
		}
	}

	if opts.Redact {
		globalRedactors := getGlobalRedactors(additionalRedactors)
		if err := redactResults(ctx, bundlePath, collectedData, globalRedactors); err != nil {
			return collectedData, err
		}
	}

	return collectedData, nil
}

func runCollectors(ctx context.Context, collectors []*troubleshootv1beta2.Collect, additionalRedactors *troubleshootv1beta2.Redactor, bundlePath string, opts SupportBundleCreateOpts) (collect.CollectorResult, error) {
	var allCollectors []collect.Collector
	var foundForbidden bool

	collectSpecs := make([]*troubleshootv1beta2.Collect, 0)
	collectSpecs = append(collectSpecs, collectors...)
	collectSpecs = collect.EnsureCollectorInList(collectSpecs, troubleshootv1beta2.Collect{ClusterInfo: &troubleshootv1beta2.ClusterInfo{}})
	collectSpecs = collect.EnsureCollectorInList(collectSpecs, troubleshootv1beta2.Collect{ClusterResources: &troubleshootv1beta2.ClusterResources{}})
	collectSpecs = collect.DedupCollectors(collectSpecs)
	collectSpecs = collect.EnsureClusterResourcesFirst(collectSpecs)

	opts.KubernetesRestConfig.QPS = constants.DEFAULT_CLIENT_QPS
	opts.KubernetesRestConfig.Burst = constants.DEFAULT_CLIENT_BURST
	opts.KubernetesRestConfig.UserAgent = fmt.Sprintf("%s/%s", constants.DEFAULT_CLIENT_USER_AGENT, version.Version())

	k8sClient, err := kubernetes.NewForConfig(opts.KubernetesRestConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to instantiate Kubernetes client")
	}

	allCollectorsMap := make(map[reflect.Type][]collect.Collector)
	allCollectedData := make(map[string][]byte)

	for _, desiredCollector := range collectSpecs {
		if collectorInterface, ok := collect.GetCollector(desiredCollector, bundlePath, opts.Namespace, opts.KubernetesRestConfig, k8sClient, opts.SinceTime); ok {
			if collector, ok := collectorInterface.(collect.Collector); ok {
				err := collector.CheckRBAC(ctx, collector, desiredCollector, opts.KubernetesRestConfig, opts.Namespace)
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
				opts.CollectorProgressCallback(opts.ProgressChan, msg)
			}
			allCollectors = append(allCollectors, mergedCollectors...)
		} else {
			allCollectors = append(allCollectors, collectors...)
		}

		foundForbidden = false
		for _, collector := range collectors {
			for _, e := range collector.GetRBACErrors() {
				foundForbidden = true
				opts.ProgressChan <- e
			}
		}
	}

	if foundForbidden && !opts.CollectWithoutPermissions {
		return nil, collect.ErrInsufficientPermissionsToRun
	}

	for _, collector := range allCollectors {
		_, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, collector.Title())
		span.SetAttributes(attribute.String("type", reflect.TypeOf(collector).String()))

		isExcluded, _ := collector.IsExcluded()
		if isExcluded {
			msg := fmt.Sprintf("excluding %q collector", collector.Title())
			opts.CollectorProgressCallback(opts.ProgressChan, msg)
			span.SetAttributes(attribute.Bool(constants.EXCLUDED, true))
			span.End()
			continue
		}

		// skip collectors with RBAC errors unless its the ClusterResources collector
		if collector.HasRBACErrors() {
			if _, ok := collector.(*collect.CollectClusterResources); !ok {
				msg := fmt.Sprintf("skipping collector %q with insufficient RBAC permissions", collector.Title())
				opts.CollectorProgressCallback(opts.ProgressChan, msg)
				span.SetStatus(codes.Error, "skipping collector, insufficient RBAC permissions")
				span.End()
				continue
			}
		}
		opts.CollectorProgressCallback(opts.ProgressChan, collector.Title())
		result, err := collector.Collect(opts.ProgressChan)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			opts.ProgressChan <- errors.Errorf("failed to run collector: %s: %v", collector.Title(), err)
		}

		for k, v := range result {
			allCollectedData[k] = v
		}
		span.End()
	}

	collectResult := allCollectedData

	globalRedactors := []*troubleshootv1beta2.Redact{}
	if additionalRedactors != nil {
		globalRedactors = additionalRedactors.Spec.Redactors
	}

	if opts.Redact {
		// TODO: Should we record how long each redactor takes?
		_, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, "In-cluster collectors")
		span.SetAttributes(attribute.String("type", "Redactors"))
		err := collect.RedactResult(bundlePath, collectResult, globalRedactors)
		if err != nil {
			err := errors.Wrap(err, "failed to redact in cluster collector results")
			span.SetStatus(codes.Error, err.Error())
			span.End()
			return collectResult, err
		}
		span.End()
	}

	return collectResult, nil
}

func findFileName(basename, extension string) (string, error) {
	n := 1
	name := basename
	for {
		filename := name + "." + extension
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			return filename, nil
		} else if err != nil {
			return "", errors.Wrap(err, "check file exists")
		}

		name = fmt.Sprintf("%s (%d)", basename, n)
		n = n + 1
	}
}

func getAnalysisFile(analyzeResults []*analyze.AnalyzeResult) (io.Reader, error) {
	data := convert.FromAnalyzerResult(analyzeResults)
	analysis, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal analysis")
	}

	return bytes.NewBuffer(analysis), nil
}

// collectRemoteHost runs remote host collectors sequentially
func collectRemoteHost(ctx context.Context, filteredCollectors []FilteredCollector, bundlePath string, opts SupportBundleCreateOpts, collectedData map[string][]byte) error {
	opts.KubernetesRestConfig.QPS = constants.DEFAULT_CLIENT_QPS
	opts.KubernetesRestConfig.Burst = constants.DEFAULT_CLIENT_BURST
	opts.KubernetesRestConfig.UserAgent = fmt.Sprintf("%s/%s", constants.DEFAULT_CLIENT_USER_AGENT, version.Version())

	// Run remote collectors sequentially
	for _, c := range filteredCollectors {
		collector := c.Collector
		spec := c.Spec

		// Send progress event: starting the collector
		opts.ProgressChan <- fmt.Sprintf("[%s] Running host collector...", collector.Title())

		// Start a span for tracing
		_, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, collector.Title())
		span.SetAttributes(attribute.String("type", reflect.TypeOf(collector).String()))

		// Parameters for remote collection
		params := &collect.RemoteCollectParams{
			ProgressChan:  opts.ProgressChan,
			HostCollector: &spec,
			BundlePath:    bundlePath,
			ClientConfig:  opts.KubernetesRestConfig,
			Image:         "replicated/troubleshoot:latest",
			PullPolicy:    "IfNotPresent",
			Timeout:       time.Duration(60 * time.Second),
			LabelSelector: "",
			NamePrefix:    "host-remote",
			Namespace:     "default",
			Title:         collector.Title(),
		}

		// Perform the collection
		result, err := collect.RemoteHostCollect(ctx, *params)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			opts.ProgressChan <- fmt.Sprintf("[%s] Error: %v", collector.Title(), err)
			return errors.Wrap(err, "failed to run remote host collector")
		}

		// Send progress event: completed successfully
		opts.ProgressChan <- fmt.Sprintf("[%s] Completed host collector", collector.Title())

		// Aggregate the results
		for k, v := range result {
			collectedData[k] = v
		}

		span.End()
	}
	return nil
}

// collectHost runs host collectors sequentially
func collectHost(ctx context.Context, filteredCollectors []FilteredCollector, opts SupportBundleCreateOpts, collectedData map[string][]byte) error {
	// Run local collectors sequentially
	for _, c := range filteredCollectors {
		collector := c.Collector

		// Send progress event: starting the collector
		opts.ProgressChan <- fmt.Sprintf("[%s] Running host collector...", collector.Title())

		// Start a span for tracing
		_, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, collector.Title())
		span.SetAttributes(attribute.String("type", reflect.TypeOf(collector).String()))

		// Run local collector sequentially
		result, err := collector.Collect(opts.ProgressChan)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			opts.ProgressChan <- fmt.Sprintf("[%s] Error: %v", collector.Title(), err)
			return errors.Wrap(err, "failed to run host collector")
		}

		// Send progress event: completed successfully
		opts.ProgressChan <- fmt.Sprintf("[%s] Completed host collector", collector.Title())

		// Aggregate the results
		for k, v := range result {
			collectedData[k] = v
		}

		span.End()
	}
	return nil
}

func redactResults(ctx context.Context, bundlePath string, collectedData collect.CollectorResult, redactors []*troubleshootv1beta2.Redact) error {
	_, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, "Host collectors")
	defer span.End()

	err := collect.RedactResult(bundlePath, collectedData, redactors)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return errors.Wrap(err, "failed to redact host collector results")
	}
	return nil
}

// getGlobalRedactors returns the global redactors from the support bundle spec
func getGlobalRedactors(additionalRedactors *troubleshootv1beta2.Redactor) []*troubleshootv1beta2.Redact {
	if additionalRedactors != nil {
		return additionalRedactors.Spec.Redactors
	}
	return []*troubleshootv1beta2.Redact{}
}

// filterHostCollectors filters out excluded collectors and returns a list of collectors to run
func filterHostCollectors(ctx context.Context, collectSpecs []*troubleshootv1beta2.HostCollect, bundlePath string, opts SupportBundleCreateOpts) ([]FilteredCollector, error) {
	var filteredCollectors []FilteredCollector

	for _, desiredCollector := range collectSpecs {
		collector, ok := collect.GetHostCollector(desiredCollector, bundlePath)
		if collector == nil {
			continue
		}
		_, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, collector.Title())
		span.SetAttributes(attribute.String("type", reflect.TypeOf(collector).String()))

		if !ok {
			return nil, collect.ErrHostCollectorNotFound
		}

		isExcluded, _ := collector.IsExcluded()
		if isExcluded {
			opts.ProgressChan <- fmt.Sprintf("[%s] Excluding host collector", collector.Title())
			span.SetAttributes(attribute.Bool(constants.EXCLUDED, true))
			span.End()
			continue
		}

		filteredCollectors = append(filteredCollectors, FilteredCollector{
			Spec:      *desiredCollector,
			Collector: collector,
		})
	}

	return filteredCollectors, nil
}
