package analyzer

import (
	"encoding/json"
	"fmt"
	"path"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	appsv1 "k8s.io/api/apps/v1"
)

func analyzeStatefulsetStatus(analyzer *troubleshootv1beta2.StatefulsetStatus, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	collected, err := getCollectedFileContents(path.Join("cluster-resources", "statefulsets", fmt.Sprintf("%s.json", analyzer.Namespace)))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read collected statefulsets from namespace")
	}

	var statefulsets []appsv1.StatefulSet
	if err := json.Unmarshal(collected, &statefulsets); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal statefulset list")
	}

	var status *appsv1.StatefulSetStatus
	for _, statefulset := range statefulsets {
		if statefulset.Name == analyzer.Name {
			status = &statefulset.Status
		}
	}

	if status == nil {
		// there's not an error, but maybe the requested statefulset is not even deployed
		return &AnalyzeResult{
			Title:   fmt.Sprintf("%s Statefulset Status", analyzer.Name),
			IconKey: "kubernetes_statefulset_status",
			IconURI: "https://troubleshoot.sh/images/analyzer-icons/statefulset-status.svg?w=23&h=14",
			IsFail:  true,
			Message: fmt.Sprintf("The statefulset %q was not found", analyzer.Name),
		}, nil
	}

	return commonStatus(analyzer.Outcomes, fmt.Sprintf("%s Status", analyzer.Name), "kubernetes_statefulset_status", "https://troubleshoot.sh/images/analyzer-icons/statefulset-status.svg?w=23&h=14", int(status.ReadyReplicas))
}
