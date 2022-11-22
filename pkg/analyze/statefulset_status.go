package analyzer

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	appsv1 "k8s.io/api/apps/v1"
)

func analyzeStatefulsetStatus(analyzer *troubleshootv1beta2.StatefulsetStatus, getFileContents func(string, []string) (map[string][]byte, error)) ([]*AnalyzeResult, error) {
	if analyzer.Name == "" {
		return analyzeAllStatefulsetStatuses(analyzer, getFileContents)
	} else {
		return analyzeOneStatefulsetStatus(analyzer, getFileContents)
	}
}

func analyzeOneStatefulsetStatus(analyzer *troubleshootv1beta2.StatefulsetStatus, getFileContents func(string, []string) (map[string][]byte, error)) ([]*AnalyzeResult, error) {
	excludeFiles := []string{}
	files, err := getFileContents(filepath.Join("cluster-resources", "statefulsets", fmt.Sprintf("%s.json", analyzer.Namespace)), excludeFiles)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read collected statefulsets from namespace")
	}

	var result *AnalyzeResult
	for _, collected := range files { // only 1 file here
		var exists bool = true
		var readyReplicas int

		var statefulsets appsv1.StatefulSetList
		if err := json.Unmarshal(collected, &statefulsets); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal statefulset list")
		}

		var statefulset *appsv1.StatefulSet
		for _, s := range statefulsets.Items {
			if s.Name == analyzer.Name {
				statefulset = s.DeepCopy()
				break
			}
		}

		if statefulset == nil {
			exists = false
			readyReplicas = 0
		} else {
			readyReplicas = int(statefulset.Status.ReadyReplicas)
		}
		if len(analyzer.Outcomes) > 0 {
			result, err = commonStatus(analyzer.Outcomes, analyzer.Name, "kubernetes_statefulset_status", "https://troubleshoot.sh/images/analyzer-icons/statefulset-status.svg?w=23&h=14", readyReplicas, exists, "statefulset")
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

func analyzeAllStatefulsetStatuses(analyzer *troubleshootv1beta2.StatefulsetStatus, getFileContents func(string, []string) (map[string][]byte, error)) ([]*AnalyzeResult, error) {
	fileNames := make([]string, 0)
	if analyzer.Namespace != "" {
		fileNames = append(fileNames, filepath.Join("cluster-resources", "statefulsets", fmt.Sprintf("%s.json", analyzer.Namespace)))
	}
	for _, ns := range analyzer.Namespaces {
		fileNames = append(fileNames, filepath.Join("cluster-resources", "statefulsets", fmt.Sprintf("%s.json", ns)))
	}

	// no namespace specified, so we need to analyze all statefulsets
	if len(fileNames) == 0 {
		fileNames = append(fileNames, filepath.Join("cluster-resources", "statefulsets", "*.json"))
	}

	excludeFiles := []string{}
	results := []*AnalyzeResult{}
	for _, fileName := range fileNames {
		files, err := getFileContents(fileName, excludeFiles)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read collected statefulsets from namespace")
		}

		for _, collected := range files {
			var statefulsets appsv1.StatefulSetList
			if err := json.Unmarshal(collected, &statefulsets); err != nil {
				return nil, errors.Wrap(err, "failed to unmarshal statefulset list")
			}

			for _, statefulset := range statefulsets.Items {
				result := getDefaultStatefulSetResult(&statefulset)
				if result != nil {
					results = append(results, result)
				}
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
