package analyzer

import (
	"encoding/json"
	"fmt"
	"path"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	appsv1 "k8s.io/api/apps/v1"
)

func analyzeDeploymentStatus(analyzer *troubleshootv1beta2.DeploymentStatus, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	collected, err := getCollectedFileContents(path.Join("cluster-resources", "deployments", fmt.Sprintf("%s.json", analyzer.Namespace)))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read collected deployments from namespace")
	}

	var deployments []appsv1.Deployment
	if err := json.Unmarshal(collected, &deployments); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal deployment list")
	}

	var status *appsv1.DeploymentStatus
	for _, deployment := range deployments {
		if deployment.Name == analyzer.Name {
			status = &deployment.Status
		}
	}

	if status == nil {
		// there's not an error, but maybe the requested deployment is not even deployed
		return &AnalyzeResult{
			Title:   fmt.Sprintf("%s Deployment Status", analyzer.Name),
			IconKey: "kubernetes_deployment_status",
			IconURI: "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17",
			IsFail:  true,
			Message: fmt.Sprintf("The deployment %q was not found", analyzer.Name),
		}, nil
	}

	return commonStatus(analyzer.Outcomes, fmt.Sprintf("%s Status", analyzer.Name), "kubernetes_deployment_status", "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17", int(status.ReadyReplicas))
}
