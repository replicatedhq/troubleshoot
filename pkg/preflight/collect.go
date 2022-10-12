package preflight

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"
	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
}

func (cr ClusterCollectResult) IsRBACAllowed() bool {
	return cr.isRBACAllowed
}

type HostCollectResult struct {
	AllCollectedData map[string][]byte
	Collectors       []collect.HostCollector
	Spec             *troubleshootv1beta2.HostPreflight
}

func (cr HostCollectResult) IsRBACAllowed() bool {
	return true
}

type RemoteCollectResult struct {
	AllCollectedData map[string][]byte
	Collectors       collect.RemoteCollectors
	Spec             *troubleshootv1beta2.HostPreflight
}

func (cr RemoteCollectResult) IsRBACAllowed() bool {
	return true
}

// CollectHost runs the collection phase of host preflight checks
func CollectHost(opts CollectOpts, p *troubleshootv1beta2.HostPreflight) (CollectResult, error) {
	collectSpecs := make([]*troubleshootv1beta2.HostCollect, 0, 0)
	collectSpecs = append(collectSpecs, p.Spec.Collectors...)

	allCollectedData := make(map[string][]byte)

	var collectors []collect.HostCollector
	for _, desiredCollector := range collectSpecs {
		collector, ok := collect.GetHostCollector(desiredCollector, "")
		if ok {
			collectors = append(collectors, collector)
		}
	}

	collectResult := HostCollectResult{
		Collectors: collectors,
		Spec:       p,
	}

	for _, collector := range collectors {
		isExcluded, _ := collector.IsExcluded()
		if isExcluded {
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
	}

	collectResult.AllCollectedData = allCollectedData

	return collectResult, nil
}

// Collect runs the collection phase of preflight checks
func Collect(opts CollectOpts, p *troubleshootv1beta2.Preflight) (CollectResult, error) {
	collectSpecs := make([]*troubleshootv1beta2.Collect, 0, 0)
	collectSpecs = append(collectSpecs, p.Spec.Collectors...)
	collectSpecs = ensureCollectorInList(collectSpecs, troubleshootv1beta2.Collect{ClusterInfo: &troubleshootv1beta2.ClusterInfo{}})
	collectSpecs = ensureCollectorInList(collectSpecs, troubleshootv1beta2.Collect{ClusterResources: &troubleshootv1beta2.ClusterResources{}})

	var allCollectors []collect.Collector

	allCollectedData := make(map[string][]byte)

	k8sClient, err := kubernetes.NewForConfig(opts.KubernetesRestConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to instantiate Kubernetes client")
	}

	for _, desiredCollector := range collectSpecs {
		if collectorInterface, ok := collect.GetCollector(desiredCollector, "", opts.Namespace, opts.KubernetesRestConfig, k8sClient, nil); ok {
			if collector, ok := collectorInterface.(collect.Collector); ok {
				err := collector.CheckRBAC(context.Background(), collector, desiredCollector, opts.KubernetesRestConfig, opts.Namespace)
				if err != nil {
					return nil, errors.Wrap(err, "failed to check RBAC for collectors")
				}

				if mergeCollector, ok := collectorInterface.(collect.MergeableCollector); ok {
					allCollectors, err = mergeCollector.Merge(allCollectors)
					if err != nil {
						msg := fmt.Sprintf("failed to merge collector: %s: %s", collector.Title(), err)
						opts.ProgressChan <- msg
					}
				} else {
					allCollectors = append(allCollectors, collector)
				}
			}
		}
	}

	collectResult := ClusterCollectResult{
		Collectors: allCollectors,
		Spec:       p,
	}

	foundForbidden := false
	for _, c := range allCollectors {
		for _, e := range c.GetRBACErrors() {
			foundForbidden = true
			opts.ProgressChan <- e
		}
	}

	if foundForbidden && !opts.IgnorePermissionErrors {
		collectResult.isRBACAllowed = false
		return collectResult, errors.New("insufficient permissions to run all collectors")
	}

	// generate a map of all collectors for atomic status messages
	collectorList := map[string]CollectorStatus{}
	for _, collector := range allCollectors {
		collectorList[collector.Title()] = CollectorStatus{
			Status: "pending",
		}
	}

	for i, collector := range allCollectors {
		isExcluded, _ := collector.IsExcluded()
		if isExcluded {
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
	}

	collectResult.AllCollectedData = allCollectedData

	return collectResult, nil
}

// Collect runs the collection phase of preflight checks
func CollectRemote(opts CollectOpts, p *troubleshootv1beta2.HostPreflight) (CollectResult, error) {
	collectSpecs := make([]*troubleshootv1beta2.RemoteCollect, 0, 0)
	collectSpecs = append(collectSpecs, p.Spec.RemoteCollectors...)

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
	}

	collectResult.AllCollectedData = allCollectedData
	return collectResult, nil
}

func ensureCollectorInList(list []*troubleshootv1beta2.Collect, collector troubleshootv1beta2.Collect) []*troubleshootv1beta2.Collect {
	for _, inList := range list {
		if collector.ClusterResources != nil && inList.ClusterResources != nil {
			return list
		}
		if collector.ClusterInfo != nil && inList.ClusterInfo != nil {
			return list
		}
	}

	return append(list, &collector)
}

func ensureHostCollectorInList(list []*troubleshootv1beta2.HostCollect, collector troubleshootv1beta2.HostCollect) []*troubleshootv1beta2.HostCollect {
	for _, inList := range list {
		if collector.CPU != nil && inList.CPU != nil {
			return list
		}
		if collector.Memory != nil && inList.Memory != nil {
			return list
		}
	}

	return append(list, &collector)
}
