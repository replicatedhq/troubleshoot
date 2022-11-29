package analyzer

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func analyzeReplicaSetStatus(analyzer *troubleshootv1beta2.ReplicaSetStatus, getFileContents getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	if analyzer.Name == "" {
		return analyzeAllReplicaSetStatuses(analyzer, getFileContents)
	} else {
		return analyzeOneReplicaSetStatus(analyzer, getFileContents)
	}
}

func analyzeOneReplicaSetStatus(analyzer *troubleshootv1beta2.ReplicaSetStatus, getFileContents getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	excludeFiles := []string{}
	files, err := getFileContents(filepath.Join("cluster-resources", "replicasets", fmt.Sprintf("%s.json", analyzer.Namespace)), excludeFiles)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read collected replicasets from namespace")
	}

	var result *AnalyzeResult
	for _, collected := range files { // only 1 file here
		var replicasets appsv1.ReplicaSetList
		if err := json.Unmarshal(collected, &replicasets); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal deployment list")
		}

		var replicaset *appsv1.ReplicaSet
		for _, r := range replicasets.Items {
			if r.Name == analyzer.Name {
				replicaset = r.DeepCopy()
				break
			}
		}

		if replicaset == nil {
			// there's not an error, but maybe the requested deployment is not even deployed
			result = &AnalyzeResult{
				Title:   fmt.Sprintf("%s ReplicaSet Status", analyzer.Name),
				IconKey: "kubernetes_deployment_status",                                                  // TODO: need new icon
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17", // TODO: need new icon
				IsFail:  true,
				Message: fmt.Sprintf("The replicaset %q was not found", analyzer.Name),
			}
		} else if len(analyzer.Outcomes) > 0 {
			result, err = replicasetStatus(analyzer.Outcomes, replicaset)
			if err != nil {
				return nil, errors.Wrap(err, "failed to process status")
			}
		} else {
			result = getDefaultReplicaSetResult(replicaset)
		}
	}

	return []*AnalyzeResult{result}, nil
}

func analyzeAllReplicaSetStatuses(analyzer *troubleshootv1beta2.ReplicaSetStatus, getFileContents getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	fileNames := make([]string, 0)
	if analyzer.Namespace != "" {
		fileNames = append(fileNames, filepath.Join("cluster-resources", "replicasets", fmt.Sprintf("%s.json", analyzer.Namespace)))
	}
	for _, ns := range analyzer.Namespaces {
		fileNames = append(fileNames, filepath.Join("cluster-resources", "replicasets", fmt.Sprintf("%s.json", ns)))
	}

	if len(fileNames) == 0 {
		fileNames = append(fileNames, filepath.Join("cluster-resources", "replicasets", "*.json"))
	}

	results := []*AnalyzeResult{}
	excludeFiles := []string{}
	for _, fileName := range fileNames {
		files, err := getFileContents(fileName, excludeFiles)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read collected replicaset from file")
		}

		labelSelector, err := labels.Parse(strings.Join(analyzer.Selector, ","))
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse selector")
		}

		for _, collected := range files {
			var replicasets appsv1.ReplicaSetList
			if err := json.Unmarshal(collected, &replicasets); err != nil {
				return nil, errors.Wrap(err, "failed to unmarshal replicaset list")
			}

			for _, replicaset := range replicasets.Items {
				if !labelSelector.Matches(labels.Set(replicaset.Labels)) {
					continue
				}

				var result *AnalyzeResult
				if len(analyzer.Outcomes) > 0 {
					result, err = replicasetStatus(analyzer.Outcomes, &replicaset)
					if err != nil {
						return nil, errors.Wrap(err, "failed to process status")
					}
				} else {
					result = getDefaultReplicaSetResult(&replicaset)
				}

				if result != nil {
					results = append(results, result)
				}
			}
		}
	}

	return results, nil
}

func replicasetStatus(outcomes []*troubleshootv1beta2.Outcome, replicaset *appsv1.ReplicaSet) (*AnalyzeResult, error) {
	result := &AnalyzeResult{
		Title:   fmt.Sprintf("%s Status", replicaset.Name),
		IconKey: "kubernetes_deployment_status",                                                  // TODO: needs new icon
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17", // TODO: needs new icon
	}

	// ordering from the spec is important, the first one that matches returns
	for _, outcome := range outcomes {
		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}

			match, err := compareReplicaSetStatusToWhen(outcome.Fail.When, replicaset)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse fail range")
			}

			if match {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}
		} else if outcome.Warn != nil {
			if outcome.Warn.When == "" {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return result, nil
			}

			match, err := compareReplicaSetStatusToWhen(outcome.Warn.When, replicaset)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse warn range")
			}

			if match {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return result, nil
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return result, nil
			}

			match, err := compareReplicaSetStatusToWhen(outcome.Pass.When, replicaset)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse pass range")
			}

			if match {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return result, nil
			}
		}
	}

	return result, nil
}

func getDefaultReplicaSetResult(replicaset *appsv1.ReplicaSet) *AnalyzeResult {
	if replicaset.Spec.Replicas == nil && replicaset.Status.AvailableReplicas == 1 { // default is 1
		return nil
	}
	if replicaset.Spec.Replicas != nil && *replicaset.Spec.Replicas == replicaset.Status.AvailableReplicas {
		return nil
	}

	return &AnalyzeResult{
		Title:   fmt.Sprintf("%s/%s ReplicaSet Status", replicaset.Namespace, replicaset.Name),
		IconKey: "kubernetes_deployment_status",                                                  // TODO: need new icon
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17", // TODO: need new icon
		IsFail:  true,
		Message: fmt.Sprintf("The replicaset %s/%s is not ready", replicaset.Namespace, replicaset.Name),
	}
}

func compareReplicaSetStatusToWhen(when string, job *appsv1.ReplicaSet) (bool, error) {
	parts := strings.Split(strings.TrimSpace(when), " ")

	// we can make this a lot more flexible
	if len(parts) != 3 {
		return false, errors.Errorf("unable to parse when range: %s", when)
	}

	value, err := strconv.Atoi(parts[2])
	if err != nil {
		return false, errors.Wrapf(err, "failed to parse when value: %s", parts[2])
	}

	var actual int32
	switch parts[0] {
	case "ready":
		actual = job.Status.ReadyReplicas
	case "available":
		actual = job.Status.AvailableReplicas
	default:
		return false, errors.Errorf("unknown when value: %s", parts[0])
	}

	switch parts[1] {
	case "=":
		fallthrough
	case "==":
		fallthrough
	case "===":
		return actual == int32(value), nil

	case "<":
		return actual < int32(value), nil

	case ">":
		return actual > int32(value), nil

	case "<=":
		return actual <= int32(value), nil

	case ">=":
		return actual >= int32(value), nil
	}

	return false, errors.Errorf("unknown comparator: %q", parts[1])
}
