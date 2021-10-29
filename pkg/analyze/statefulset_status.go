package analyzer

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	appsv1 "k8s.io/api/apps/v1"
)

func analyzeStatefulsetStatus(analyzer *troubleshootv1beta2.StatefulsetStatus, getFileContents func(string) (map[string][]byte, error)) ([]*AnalyzeResult, error) {
	if analyzer.Name == "" {
		return analyzeAllStatefulsetStatuses(analyzer, getFileContents)
	} else {
		return analyzeOneStatefulsetStatus(analyzer, getFileContents)
	}
}

func analyzeOneStatefulsetStatus(analyzer *troubleshootv1beta2.StatefulsetStatus, getFileContents func(string) (map[string][]byte, error)) ([]*AnalyzeResult, error) {
	files, err := getFileContents(filepath.Join("cluster-resources", "statefulsets", fmt.Sprintf("%s.json", analyzer.Namespace)))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read collected statefulsets from namespace")
	}

	var result *AnalyzeResult
	for _, collected := range files { // only 1 file here
		var statefulsets []appsv1.StatefulSet
		if err := json.Unmarshal(collected, &statefulsets); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal statefulset list")
		}

		var statefulset *appsv1.StatefulSet
		for _, s := range statefulsets {
			if s.Name == analyzer.Name {
				statefulset = s.DeepCopy()
				break
			}
		}

		if statefulset == nil {
			result = &AnalyzeResult{
				Title:   fmt.Sprintf("%s Statefulset Status", analyzer.Name),
				IconKey: "kubernetes_statefulset_status",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/statefulset-status.svg?w=23&h=14",
				IsFail:  true,
				Message: fmt.Sprintf("The statefulset %q was not found", analyzer.Name),
			}
		} else if len(analyzer.Outcomes) > 0 {
			result, err = commonStatus(analyzer.Outcomes, fmt.Sprintf("%s Status", analyzer.Name), "kubernetes_statefulset_status", "https://troubleshoot.sh/images/analyzer-icons/statefulset-status.svg?w=23&h=14", int(statefulset.Status.ReadyReplicas))
			if err != nil {
				return nil, errors.Wrap(err, "failed to process status")
			}
		} else {
			result = getDefaultStatefulSetResult(statefulset)
		}
	}

	if result == nil {
		return nil, nil
	}

	return []*AnalyzeResult{result}, nil
}

func analyzeAllStatefulsetStatuses(analyzer *troubleshootv1beta2.StatefulsetStatus, getFileContents func(string) (map[string][]byte, error)) ([]*AnalyzeResult, error) {
	var fileName string
	if analyzer.Namespace != "" {
		fileName = filepath.Join("cluster-resources", "statefulsets", fmt.Sprintf("%s.json", analyzer.Namespace))
	} else {
		fileName = filepath.Join("cluster-resources", "statefulsets", "*.json")
	}

	files, err := getFileContents(fileName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read collected statefulsets from namespace")
	}

	results := []*AnalyzeResult{}
	for _, collected := range files {
		var statefulsets []appsv1.StatefulSet
		if err := json.Unmarshal(collected, &statefulsets); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal statefulset list")
		}

		for _, statefulset := range statefulsets {
			result := getDefaultStatefulSetResult(&statefulset)
			if result != nil {
				results = append(results, result)
			}
		}
	}

	return results, nil
}

func getDefaultStatefulSetResult(statefulset *appsv1.StatefulSet) *AnalyzeResult {
	if statefulset.Status.Replicas == statefulset.Status.ReadyReplicas {
		return nil
	}

	return &AnalyzeResult{
		Title:   fmt.Sprintf("%s/%s Statefulset Status", statefulset.Namespace, statefulset.Name),
		IconKey: "kubernetes_statefulset_status",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/statefulset-status.svg?w=23&h=14",
		IsFail:  true,
		Message: fmt.Sprintf("The statefulset %s/%s has %d/%d replicas", statefulset.Namespace, statefulset.Name, statefulset.Status.ReadyReplicas, statefulset.Status.Replicas),
	}
}
