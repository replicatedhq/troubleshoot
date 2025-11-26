package preflight

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/pkg/errors"
	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/version"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type CollectOpts struct {
	Namespace              string
	IgnorePermissionErrors bool
	KubernetesRestConfig   *rest.Config
	Image                  string
	PullPolicy             string
	LabelSelector          string
	Timeout                time.Duration
	ProgressChan           chan interface{}

	// Optional path to the bundle directory to store the collected data
	BundlePath string
}

type CollectProgress struct {
	CurrentName    string
	CurrentStatus  string
	CompletedCount int
	TotalCount     int
	Collectors     map[string]CollectorStatus
}

type CollectorStatus struct {
	Status string
}

func (cp *CollectProgress) String() string {
	return fmt.Sprintf("name: %-20s status: %-15s completed: %-4d total: %-4d", cp.CurrentName, cp.CurrentStatus, cp.CompletedCount, cp.TotalCount)
}

type CollectResult interface {
	Analyze() []*analyze.AnalyzeResult
	IsRBACAllowed() bool
}

type ClusterCollectResult struct {
	AllCollectedData map[string][]byte
	Collectors       []collect.Collector
	RemoteCollectors collect.RemoteCollectors
	isRBACAllowed    bool
	Spec             *troubleshootv1beta2.Preflight
	Context          context.Context
}

func (cr ClusterCollectResult) IsRBACAllowed() bool {
	return cr.isRBACAllowed
}

type HostCollectResult struct {
	AllCollectedData map[string][]byte
	Collectors       []collect.HostCollector
	Spec             *troubleshootv1beta2.HostPreflight
	Context          context.Context
}

func (cr HostCollectResult) IsRBACAllowed() bool {
	return true
}

type RemoteCollectResult struct {
	AllCollectedData map[string][]byte
	Collectors       collect.RemoteCollectors
	Spec             *troubleshootv1beta2.HostPreflight
	Context          context.Context
}

func (cr RemoteCollectResult) IsRBACAllowed() bool {
	return true
}

// CollectHost runs the collection phase of host preflight checks
func CollectHost(opts CollectOpts, p *troubleshootv1beta2.HostPreflight) (CollectResult, error) {
	return CollectHostWithContext(context.Background(), opts, p)
}

func CollectHostWithContext(
	ctx context.Context, opts CollectOpts, p *troubleshootv1beta2.HostPreflight,
) (CollectResult, error) {
	collectSpecs := make([]*troubleshootv1beta2.HostCollect, 0)
	if p != nil && p.Spec.Collectors != nil {
		collectSpecs = append(collectSpecs, p.Spec.Collectors...)
	}

	allCollectedData := make(map[string][]byte)

	var collectors []collect.HostCollector
	for _, desiredCollector := range collectSpecs {
		collector, ok := collect.GetHostCollector(desiredCollector, opts.BundlePath)
		if ok {
			collectors = append(collectors, collector)
		}
	}

	collectResult := HostCollectResult{
		Collectors: collectors,
		Spec:       p,
		Context:    ctx,
	}

	for _, collector := range collectors {
		_, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, collector.Title())
		span.SetAttributes(attribute.String("type", reflect.TypeOf(collector).String()))

		isExcluded, _ := collector.IsExcluded()
		if isExcluded {
			opts.ProgressChan <- fmt.Sprintf("[%s] Excluding collector", collector.Title())
			span.SetAttributes(attribute.Bool(constants.EXCLUDED, true))
			span.End()
			continue
		}

		opts.ProgressChan <- fmt.Sprintf("[%s] Running collector...", collector.Title())
		result, err := collector.Collect(opts.ProgressChan)
		if err != nil {
			opts.ProgressChan <- errors.Errorf("failed to run collector: %s: %v", collector.Title(), err)
		}
		for k, v := range result {
			allCollectedData[k] = v
		}
		span.End()
	}

	// The values of map entries will contain the collected data in bytes if the data was not stored to disk
	collectResult.AllCollectedData = allCollectedData

	return collectResult, nil
}

// Collect runs the collection phase of preflight checks
func Collect(opts CollectOpts, p *troubleshootv1beta2.Preflight) (CollectResult, error) {
	return CollectWithContext(context.Background(), opts, p)
}

func CollectWithContext(ctx context.Context, opts CollectOpts, p *troubleshootv1beta2.Preflight) (CollectResult, error) {
	var allCollectors []collect.Collector
	var foundForbidden bool

	collectSpecs := make([]*troubleshootv1beta2.Collect, 0)
	if p != nil && p.Spec.Collectors != nil {
		collectSpecs = append(collectSpecs, p.Spec.Collectors...)
	}
	collectSpecs = collect.EnsureCollectorInList(
		collectSpecs, troubleshootv1beta2.Collect{ClusterInfo: &troubleshootv1beta2.ClusterInfo{}},
	)
	collectSpecs = collect.EnsureCollectorInList(
		collectSpecs, troubleshootv1beta2.Collect{ClusterResources: &troubleshootv1beta2.ClusterResources{}},
	)
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
	// Track the order in which we first see each collector type
	collectorTypeOrder := []reflect.Type{}
	seenTypes := make(map[reflect.Type]bool)

	for _, desiredCollector := range collectSpecs {
		if collectorInterface, ok := collect.GetCollector(desiredCollector, opts.BundlePath, opts.Namespace, opts.KubernetesRestConfig, k8sClient, nil); ok {
			if collector, ok := collectorInterface.(collect.Collector); ok {
				err := collector.CheckRBAC(ctx, collector, desiredCollector, opts.KubernetesRestConfig, opts.Namespace)
				if err != nil {
					return nil, errors.Wrap(err, "failed to check RBAC for collectors")
				}
				collectorType := reflect.TypeOf(collector)
				allCollectorsMap[collectorType] = append(allCollectorsMap[collectorType], collector)

				// Track first appearance of each type to preserve order
				if !seenTypes[collectorType] {
					collectorTypeOrder = append(collectorTypeOrder, collectorType)
					seenTypes[collectorType] = true
				}
			}
		}
	}

	collectorList := map[string]CollectorStatus{}

	// Iterate in the order collectors were first seen (preserves EnsureClusterResourcesFirst ordering)
	for _, collectorType := range collectorTypeOrder {
		collectors := allCollectorsMap[collectorType]
		if mergeCollector, ok := collectors[0].(collect.MergeableCollector); ok {
			mergedCollectors, err := mergeCollector.Merge(collectors)
			if err != nil {
				msg := fmt.Sprintf("failed to merge collector: %s: %s", mergeCollector.Title(), err)
				opts.ProgressChan <- msg
			}
			allCollectors = append(allCollectors, mergedCollectors...)
		} else {
			allCollectors = append(allCollectors, collectors...)
		}

		for _, collector := range collectors {
			for _, e := range collector.GetRBACErrors() {
				foundForbidden = true
				opts.ProgressChan <- e
			}

			// generate a map of all collectors for atomic status messages
			collectorList[collector.Title()] = CollectorStatus{
				Status: "pending",
			}
		}
	}

	collectResult := ClusterCollectResult{
		Collectors: allCollectors,
		Spec:       p,
		Context:    ctx,
	}

	if foundForbidden && !opts.IgnorePermissionErrors {
		collectResult.isRBACAllowed = false
		return collectResult, collect.ErrInsufficientPermissionsToRun
	}

	// move Copy Collectors if any to the end of the execution list
	allCollectors = collect.EnsureCopyLast(allCollectors)

	for i, collector := range allCollectors {
		_, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, collector.Title())
		span.SetAttributes(attribute.String("type", reflect.TypeOf(collector).String()))

		isExcluded, _ := collector.IsExcluded()
		if isExcluded {
			klog.V(1).Infof("excluding %q collector", collector.Title())
			span.SetAttributes(attribute.Bool(constants.EXCLUDED, true))
			span.End()
			continue
		}

		// skip collectors with RBAC errors unless its the ClusterResources collector
		if collector.HasRBACErrors() {
			if _, ok := collector.(*collect.CollectClusterResources); !ok {
				opts.ProgressChan <- fmt.Sprintf("skipping collector %s with insufficient RBAC permissions", collector.Title())
				opts.ProgressChan <- CollectProgress{
					CurrentName:    collector.Title(),
					CurrentStatus:  "skipped",
					CompletedCount: i + 1,
					TotalCount:     len(allCollectors),
					Collectors:     collectorList,
				}
				span.SetStatus(codes.Error, "skipping collector, insufficient RBAC permissions")
				span.End()
				continue
			}
		}

		collectorList[collector.Title()] = CollectorStatus{
			Status: "running",
		}
		opts.ProgressChan <- CollectProgress{
			CurrentName:    collector.Title(),
			CurrentStatus:  "running",
			CompletedCount: i,
			TotalCount:     len(allCollectors),
			Collectors:     collectorList,
		}

		result, err := collector.Collect(opts.ProgressChan)
		if err != nil {
			collectorList[collector.Title()] = CollectorStatus{
				Status: "failed",
			}
			opts.ProgressChan <- errors.Errorf("failed to run collector: %s: %v", collector.Title(), err)
			opts.ProgressChan <- CollectProgress{
				CurrentName:    collector.Title(),
				CurrentStatus:  "failed",
				CompletedCount: i + 1,
				TotalCount:     len(allCollectors),
				Collectors:     collectorList,
			}
			span.SetStatus(codes.Error, err.Error())
			span.End()
			continue
		}

		collectorList[collector.Title()] = CollectorStatus{
			Status: "completed",
		}
		opts.ProgressChan <- CollectProgress{
			CurrentName:    collector.Title(),
			CurrentStatus:  "completed",
			CompletedCount: i + 1,
			TotalCount:     len(allCollectors),
			Collectors:     collectorList,
		}

		for k, v := range result {
			allCollectedData[k] = v
		}
		span.End()
	}

	// The values of map entries will contain the collected data in bytes if the data was not stored to disk
	collectResult.AllCollectedData = allCollectedData

	return collectResult, nil
}

// Collect runs the collection phase of preflight checks
func CollectRemote(opts CollectOpts, p *troubleshootv1beta2.HostPreflight) (CollectResult, error) {
	return CollectRemoteWithContext(context.Background(), opts, p)
}

func CollectRemoteWithContext(ctx context.Context, opts CollectOpts, p *troubleshootv1beta2.HostPreflight) (CollectResult, error) {
	collectSpecs := make([]*troubleshootv1beta2.RemoteCollect, 0, 0)
	if p != nil && p.Spec.RemoteCollectors != nil {
		collectSpecs = append(collectSpecs, p.Spec.RemoteCollectors...)
	}

	allCollectedData := make(map[string][]byte)

	var collectors collect.RemoteCollectors
	for _, desiredCollector := range collectSpecs {
		collector := collect.RemoteCollector{
			Redact:        true,
			Collect:       desiredCollector,
			ClientConfig:  opts.KubernetesRestConfig,
			Image:         opts.Image,
			PullPolicy:    opts.PullPolicy,
			LabelSelector: opts.LabelSelector,
			Namespace:     opts.Namespace,
			Timeout:       opts.Timeout,
		}
		collectors = append(collectors, &collector)
	}

	collectResult := RemoteCollectResult{
		Collectors: collectors,
		Spec:       p,
		Context:    ctx,
	}

	// generate a map of all collectors for atomic status messages
	collectorList := map[string]CollectorStatus{}
	for _, collector := range collectors {
		collectorList[collector.GetDisplayName()] = CollectorStatus{
			Status: "pending",
		}
	}

	// Run preflights collectors synchronously
	for i, collector := range collectors {
		_, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, collector.GetDisplayName())
		span.SetAttributes(attribute.String("type", reflect.TypeOf(collector).String()))

		collectorList[collector.GetDisplayName()] = CollectorStatus{
			Status: "running",
		}

		opts.ProgressChan <- CollectProgress{
			CurrentName:    collector.GetDisplayName(),
			CurrentStatus:  "running",
			CompletedCount: i,
			TotalCount:     len(collectors),
			Collectors:     collectorList,
		}

		result, err := collector.RunCollectorSync(nil)
		if err != nil {
			collectorList[collector.GetDisplayName()] = CollectorStatus{
				Status: "failed",
			}

			opts.ProgressChan <- errors.Errorf("failed to run collector %s: %v\n", collector.GetDisplayName(), err)
			opts.ProgressChan <- CollectProgress{
				CurrentName:    collector.GetDisplayName(),
				CurrentStatus:  "failed",
				CompletedCount: i + 1,
				TotalCount:     len(collectors),
				Collectors:     collectorList,
			}

			// Save collector error to bundle (write to disk if bundlePath exists)
			errorInfo := map[string]string{
				"collector": collector.GetDisplayName(),
				"error":     err.Error(),
				"timestamp": time.Now().Format(time.RFC3339),
			}
			if errorJSON, marshalErr := json.Marshal(errorInfo); marshalErr == nil {
				errorPath := fmt.Sprintf("collector-errors/%s-error.json", collector.GetDisplayName())
				// Always store bytes in-memory for consistency
				allCollectedData[errorPath] = errorJSON
				// Best-effort write to disk if bundlePath provided
				if opts.BundlePath != "" {
					if writeErr := os.MkdirAll(filepath.Join(opts.BundlePath, "collector-errors"), 0755); writeErr == nil {
						_ = os.WriteFile(filepath.Join(opts.BundlePath, errorPath), errorJSON, 0644)
					}
				}
			}

			span.SetStatus(codes.Error, err.Error())
			span.End()
			continue
		}

		collectorList[collector.GetDisplayName()] = CollectorStatus{
			Status: "completed",
		}

		opts.ProgressChan <- CollectProgress{
			CurrentName:    collector.GetDisplayName(),
			CurrentStatus:  "completed",
			CompletedCount: i + 1,
			TotalCount:     len(collectors),
			Collectors:     collectorList,
		}

		for k, v := range result {
			if curBytes, ok := allCollectedData[k]; ok {
				var curResults map[string]string
				if err := json.Unmarshal(curBytes, &curResults); err != nil {
					opts.ProgressChan <- errors.Errorf("failed to read existing results for collector %s: %v\n", collector.GetDisplayName(), err)
					continue
				}
				var newResults map[string]string
				if err := json.Unmarshal(v, &newResults); err != nil {
					opts.ProgressChan <- errors.Errorf("failed to read new results for collector %s: %v\n", collector.GetDisplayName(), err)
					continue
				}
				for file, data := range newResults {
					curResults[file] = data
				}
				combinedResults, err := json.Marshal(curResults)
				if err != nil {
					opts.ProgressChan <- errors.Errorf("failed to combine results for collector %s: %v\n", collector.GetDisplayName(), err)
					continue
				}
				allCollectedData[k] = combinedResults
			} else {
				allCollectedData[k] = v
			}

		}

		span.End()
	}

	collectResult.AllCollectedData = allCollectedData
	return collectResult, nil
}
