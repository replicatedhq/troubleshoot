package analyzer

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
)

type AnalyzeDeploymentStatus struct {
	analyzer *troubleshootv1beta2.DeploymentStatus
}

func (a *AnalyzeDeploymentStatus) Title() string {
	if a.analyzer.CheckName != "" {
		return a.analyzer.CheckName
	}

	if a.analyzer.Name != "" && a.analyzer.Namespace != "" {
		return fmt.Sprintf("%s/%s Deployment Status", a.analyzer.Namespace, a.analyzer.Name)
	}

	if a.analyzer.Name != "" {
		return fmt.Sprintf("%s Deployment Status", a.analyzer.Name)
	}
	if a.analyzer.Namespace != "" {
		return fmt.Sprintf("%s Deployment Status", a.analyzer.Namespace)
	}

	return "Deployment Status"
}

func (a *AnalyzeDeploymentStatus) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeDeploymentStatus) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	results, err := analyzeDeploymentStatus(a.analyzer, findFiles)
	if err != nil {
		return nil, err
	}
	for i := range results {
		results[i].Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	}
	return results, nil
}

func analyzeDeploymentStatus(analyzer *troubleshootv1beta2.DeploymentStatus, getFileContents getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	if analyzer.Name == "" {
		return analyzeAllDeploymentStatuses(analyzer, getFileContents)
	} else {
		return analyzeOneDeploymentStatus(analyzer, getFileContents)
	}
}

func analyzeOneDeploymentStatus(analyzer *troubleshootv1beta2.DeploymentStatus, getFileContents getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	excludeFiles := []string{}
	files, err := getFileContents(filepath.Join(constants.CLUSTER_RESOURCES_DIR, constants.CLUSTER_RESOURCES_DEPLOYMENTS, fmt.Sprintf("%s.json", analyzer.Namespace)), excludeFiles)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read collected deployments from namespace")
	}

	var result *AnalyzeResult
	for _, collected := range files { // only 1 file here
		var exists bool = true
		var readyReplicas int

		var deployments appsv1.DeploymentList
		if err := json.Unmarshal(collected, &deployments); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal deployment list")
		}

		var status *appsv1.DeploymentStatus
		for _, deployment := range deployments.Items {
			if deployment.Name == analyzer.Name {
				status = deployment.Status.DeepCopy()
			}
		}

		if status == nil {
			exists = false
			readyReplicas = 0
		} else {
			readyReplicas = int(status.ReadyReplicas)
		}

		result, err = commonStatus(analyzer.Outcomes, analyzer.Name, "kubernetes_deployment_status", "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17", readyReplicas, exists, "deployment")
		if err != nil {
			return nil, errors.Wrap(err, "failed to process status")
		}
	}

	if result == nil {
		return nil, nil
	}

	return []*AnalyzeResult{result}, nil
}

func analyzeAllDeploymentStatuses(analyzer *troubleshootv1beta2.DeploymentStatus, getFileContents getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	excludeFiles := []string{}
	fileNames := make([]string, 0)
	if analyzer.Namespace != "" {
		fileNames = append(fileNames, filepath.Join(constants.CLUSTER_RESOURCES_DIR, constants.CLUSTER_RESOURCES_DEPLOYMENTS, fmt.Sprintf("%s.json", analyzer.Namespace)))
	}
	for _, ns := range analyzer.Namespaces {
		fileNames = append(fileNames, filepath.Join(constants.CLUSTER_RESOURCES_DIR, constants.CLUSTER_RESOURCES_DEPLOYMENTS, fmt.Sprintf("%s.json", ns)))
	}

	// no namespace specified, so we need to analyze all deployments
	if len(fileNames) == 0 {
		fileNames = append(fileNames, filepath.Join(constants.CLUSTER_RESOURCES_DIR, constants.CLUSTER_RESOURCES_DEPLOYMENTS, "*.json"))
	}

	results := []*AnalyzeResult{}
	for _, fileName := range fileNames {
		files, err := getFileContents(fileName, excludeFiles)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read collected deployments from file")
		}

		for _, collected := range files {
			var deployments appsv1.DeploymentList
			if err := json.Unmarshal(collected, &deployments); err != nil {
				return nil, errors.Wrap(err, "failed to unmarshal deployment list")
			}

			for _, deployment := range deployments.Items {
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
	}

	return results, nil
}
