package collect

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/client-go/rest"
)

type CollectorRunOpts struct {
	Namespace                 string
	CollectWithoutPermissions bool
	HttpClient                *http.Client
	KubernetesRestConfig      *rest.Config
	Image                     string
	PullPolicy                string
	LabelSelector             string
	Timeout                   time.Duration
	ProgressChan              chan interface{}
}

type CollectProgress struct {
	CurrentName    string
	CurrentStatus  string
	CompletedCount int
	TotalCount     int
}

type HostCollectResult struct {
	AllCollectedData map[string][]byte
	Collectors       []HostCollector
	Spec             *troubleshootv1beta2.HostCollector
}

type RemoteCollectResult struct {
	AllCollectedData map[string][]byte
	Collectors       RemoteCollectors
	Spec             *troubleshootv1beta2.RemoteCollector
	isRBACAllowed    bool
}

// CollectHost runs the collection phase for a local collector.
func CollectHost(c *troubleshootv1beta2.HostCollector, additionalRedactors *troubleshootv1beta2.Redactor, opts CollectorRunOpts) (*HostCollectResult, error) {
	allCollectedData := make(map[string][]byte)

	var collectors []HostCollector
	for _, desiredCollector := range c.Spec.Collectors {
		collector, ok := GetHostCollector(desiredCollector)
		if ok {
			collectors = append(collectors, collector)
		}
	}

	collectResult := &HostCollectResult{
		Collectors: collectors,
		Spec:       c,
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

// CollectRemote runs the collection phase for a remote collector.
func CollectRemote(c *troubleshootv1beta2.RemoteCollector, additionalRedactors *troubleshootv1beta2.Redactor, opts CollectorRunOpts) (*RemoteCollectResult, error) {
	allCollectedData := make(map[string][]byte)

	var collectors RemoteCollectors
	for _, desiredCollector := range c.Spec.Collectors {
		collector := RemoteCollector{
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

	collectResult := &RemoteCollectResult{
		Collectors: collectors,
		Spec:       c,
	}

	if err := collectors.CheckRBAC(context.Background()); err != nil {
		return collectResult, errors.Wrap(err, "failed to check RBAC for collectors")
	}

	foundForbidden := false
	for _, c := range collectors {
		for _, e := range c.RBACErrors {
			foundForbidden = true
			opts.ProgressChan <- e
		}
	}

	if foundForbidden && !opts.CollectWithoutPermissions {
		collectResult.isRBACAllowed = false
		return collectResult, errors.New("insufficient permissions to run all collectors")
	}

	// Run collectors synchronously.
	for i, collector := range collectors {
		opts.ProgressChan <- CollectProgress{
			CurrentName:    collector.GetDisplayName(),
			CurrentStatus:  "running",
			CompletedCount: i,
			TotalCount:     len(collectors),
		}

		result, err := collector.RunCollectorSync(nil)
		if err != nil {
			opts.ProgressChan <- errors.Errorf("failed to run collector %s: %v\n", collector.GetDisplayName(), err)
			opts.ProgressChan <- CollectProgress{
				CurrentName:    collector.GetDisplayName(),
				CurrentStatus:  "failed",
				CompletedCount: i + 1,
				TotalCount:     len(collectors),
			}
			continue
		}

		opts.ProgressChan <- CollectProgress{
			CurrentName:    collector.GetDisplayName(),
			CurrentStatus:  "completed",
			CompletedCount: i + 1,
			TotalCount:     len(collectors),
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
