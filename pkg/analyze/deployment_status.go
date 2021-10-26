package analyzer

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	appsv1 "k8s.io/api/apps/v1"
)

func analyzeDeploymentStatus(analyzer *troubleshootv1beta2.DeploymentStatus, getFileContents func(string) (map[string][]byte, error)) ([]*AnalyzeResult, error) {
	if analyzer.Name == "" {
		return analyzeAllDeploymentStatuses(analyzer, getFileContents)
	} else {
		return analyzeOneDeploymentStatus(analyzer, getFileContents)
	}
}

func analyzeOneDeploymentStatus(analyzer *troubleshootv1beta2.DeploymentStatus, getFileContents func(string) (map[string][]byte, error)) ([]*AnalyzeResult, error) {
	files, err := getFileContents(filepath.Join("cluster-resources", "deployments", fmt.Sprintf("%s.json", analyzer.Namespace)))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read collected deployments from namespace")
	}

	var result *AnalyzeResult
	for _, collected := range files { // only 1 file here
		var deployments []appsv1.Deployment
		if err := json.Unmarshal(collected, &deployments); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal deployment list")
		}

		var status *appsv1.DeploymentStatus
		for _, deployment := range deployments {
			if deployment.Name == analyzer.Name {
				status = deployment.Status.DeepCopy()
			}
		}

		if status == nil {
			// there's not an error, but maybe the requested deployment is not even deployed
			result = &AnalyzeResult{
				Title:   fmt.Sprintf("%s Deployment Status", analyzer.Name),
				IconKey: "kubernetes_deployment_status",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17",
				IsFail:  true,
				Message: fmt.Sprintf("The deployment %q was not found", analyzer.Name),
			}
		} else {
			result, err = commonStatus(analyzer.Outcomes, fmt.Sprintf("%s Status", analyzer.Name), "kubernetes_deployment_status", "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17", int(status.ReadyReplicas))
			if err != nil {
				return nil, errors.Wrap(err, "failed to process status")
			}
		}
	}

	return []*AnalyzeResult{result}, nil
}

func analyzeAllDeploymentStatuses(analyzer *troubleshootv1beta2.DeploymentStatus, getFileContents func(string) (map[string][]byte, error)) ([]*AnalyzeResult, error) {
	var fileName string
	if analyzer.Namespace != "" {
		fileName = filepath.Join("cluster-resources", "deployments", fmt.Sprintf("%s.json", analyzer.Namespace))
	} else {
		fileName = filepath.Join("cluster-resources", "deployments", "*.json")
	}

	files, err := getFileContents(fileName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read collected deployments from file")
	}

	results := []*AnalyzeResult{}
	for _, collected := range files {
		var deployments []appsv1.Deployment
		if err := json.Unmarshal(collected, &deployments); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal deployment list")
		}

		for _, deployment := range deployments {
			if deployment.Status.Replicas == deployment.Status.AvailableReplicas {
				continue
			}

			result := &AnalyzeResult{
				Title:   fmt.Sprintf("%s/%s Deployment Status", deployment.Namespace, deployment.Name),
				IconKey: "kubernetes_deployment_status",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17",
				IsFail:  true,
				Message: fmt.Sprintf("The deployment %s/%s has %d/%d replicas", deployment.Namespace, deployment.Name, deployment.Status.ReadyReplicas, deployment.Status.Replicas),
			}

			results = append(results, result)
		}
	}

	return results, nil
}
