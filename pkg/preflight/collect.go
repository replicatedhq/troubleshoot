package preflight

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"k8s.io/client-go/rest"
)

type CollectOpts struct {
	Namespace              string
	IgnorePermissionErrors bool
	KubernetesRestConfig   *rest.Config
	ProgressChan           chan interface{}
}

type CollectResult struct {
	AllCollectedData map[string][]byte
	Collectors       collect.Collectors
	IsRBACAllowed    bool
	Spec             *troubleshootv1beta2.Preflight
}

// Collect runs the collection phase of preflight checks
func Collect(opts CollectOpts, p *troubleshootv1beta2.Preflight) (CollectResult, map[string][]byte, error) {
	var result map[string][]byte
	var protected map[string][]byte
	var err error
	collectSpecs := make([]*troubleshootv1beta2.Collect, 0, 0)
	collectSpecs = append(collectSpecs, p.Spec.Collectors...)
	collectSpecs = ensureCollectorInList(collectSpecs, troubleshootv1beta2.Collect{ClusterInfo: &troubleshootv1beta2.ClusterInfo{}})
	collectSpecs = ensureCollectorInList(collectSpecs, troubleshootv1beta2.Collect{ClusterResources: &troubleshootv1beta2.ClusterResources{}})

	allCollectedData := make(map[string][]byte)

	var collectors collect.Collectors
	for _, desiredCollector := range collectSpecs {
		collector := collect.Collector{
			Redact:       true,
			Collect:      desiredCollector,
			ClientConfig: opts.KubernetesRestConfig,
			Namespace:    opts.Namespace,
		}
		collectors = append(collectors, &collector)
	}

	collectResult := CollectResult{
		Collectors: collectors,
		Spec:       p,
	}

	if err := collectors.CheckRBAC(context.Background()); err != nil {
		return collectResult, nil, errors.Wrap(err, "failed to check RBAC for collectors")
	}

	foundForbidden := false
	for _, c := range collectors {
		for _, e := range c.RBACErrors {
			foundForbidden = true
			opts.ProgressChan <- e
		}
	}

	if foundForbidden && !opts.IgnorePermissionErrors {
		collectResult.IsRBACAllowed = false
		return collectResult, nil, errors.New("insufficient permissions to run all collectors")
	}

	// Run preflights collectors synchronously
	for _, collector := range collectors {
		if len(collector.RBACErrors) > 0 {
			// don't skip clusterResources collector due to RBAC issues
			if collector.Collect.ClusterResources == nil {
				collectResult.IsRBACAllowed = false // not failing, but going to report this
				opts.ProgressChan <- fmt.Sprintf("skipping collector %s with insufficient RBAC permissions", collector.GetDisplayName())
				continue
			}
		}

		result, protected, err = collector.RunCollectorSync(nil, p.Spec.Analyzers)
		if err != nil {
			opts.ProgressChan <- errors.Errorf("failed to run collector %s: %v\n", collector.GetDisplayName(), err)
			continue
		}

		if result != nil {
			for k, v := range result {
				allCollectedData[k] = v
			}
		}
	}

	collectResult.AllCollectedData = allCollectedData
	return collectResult, protected, nil
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
