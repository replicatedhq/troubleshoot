package preflight

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
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
	Spec             *troubleshootv1beta1.Preflight
}

// Collect runs the collection phase of preflight checks
func Collect(opts CollectOpts, p *troubleshootv1beta1.Preflight) (CollectResult, error) {
	collectSpecs := make([]*troubleshootv1beta1.Collect, 0, 0)
	collectSpecs = append(collectSpecs, p.Spec.Collectors...)
	collectSpecs = ensureCollectorInList(collectSpecs, troubleshootv1beta1.Collect{ClusterInfo: &troubleshootv1beta1.ClusterInfo{}})
	collectSpecs = ensureCollectorInList(collectSpecs, troubleshootv1beta1.Collect{ClusterResources: &troubleshootv1beta1.ClusterResources{}})

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

	if err := collectors.CheckRBAC(); err != nil {
		return collectResult, errors.Wrap(err, "failed to check RBAC for collectors")
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
		return collectResult, errors.New("insufficient permissions to run all collectors")
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

		result, err := collector.RunCollectorSync()
		if err != nil {
			opts.ProgressChan <- errors.Errorf("failed to run collector %s: %v\n", collector.GetDisplayName(), err)
			continue
		}

		if result != nil {
			output, err := parseCollectorOutput(string(result))
			if err != nil {
				opts.ProgressChan <- errors.Errorf("failed to parse collector output %s: %v\n", collector.GetDisplayName(), err)
				continue
			}
			for k, v := range output {
				allCollectedData[k] = v
			}
		}
	}

	collectResult.AllCollectedData = allCollectedData
	return collectResult, nil
}

func parseCollectorOutput(output string) (map[string][]byte, error) {
	input := make(map[string]interface{})
	files := make(map[string][]byte)
	if err := json.Unmarshal([]byte(output), &input); err != nil {
		return nil, err
	}

	for filename, maybeContents := range input {
		fileDir, fileName := filepath.Split(filename)

		switch maybeContents.(type) {
		case string:
			decoded, err := base64.StdEncoding.DecodeString(maybeContents.(string))
			if err != nil {
				return nil, err
			}
			files[filepath.Join(fileDir, fileName)] = decoded

		case map[string]interface{}:
			for k, v := range maybeContents.(map[string]interface{}) {
				decoded, err := base64.StdEncoding.DecodeString(v.(string))
				if err != nil {
					return nil, err
				}
				files[filepath.Join(fileDir, fileName, k)] = decoded
			}
		}
	}

	return files, nil
}

func ensureCollectorInList(list []*troubleshootv1beta1.Collect, collector troubleshootv1beta1.Collect) []*troubleshootv1beta1.Collect {
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
