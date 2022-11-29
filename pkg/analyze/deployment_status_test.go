package analyzer

import (
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_deploymentStatus(t *testing.T) {
	tests := []struct {
		name         string
		analyzer     troubleshootv1beta2.DeploymentStatus
		expectResult []*AnalyzeResult
		files        map[string][]byte
	}{
		{
			name: "1/1, fail when absent",
			analyzer: troubleshootv1beta2.DeploymentStatus{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "absent",
							Message: "fail",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "= 1",
							Message: "pass",
						},
					},
				},
				Namespace: "default",
				Name:      "nonexistant-deployment",
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "nonexistant-deployment Status",
					Message: "fail",
					IconKey: "kubernetes_deployment_status",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17",
				},
			},
			files: map[string][]byte{
				"cluster-resources/deployments/default.json":     []byte(defaultDeployments),
				"cluster-resources/deployments/monitoring.json":  []byte(monitoringDeployments),
				"cluster-resources/deployments/kube-system.json": []byte(kubeSystemDeployments),
			},
		},
		{
			name: "1/1, pass when = 1",
			analyzer: troubleshootv1beta2.DeploymentStatus{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "absent",
							Message: "fail",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "= 1",
							Message: "pass",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "fail",
						},
					},
				},
				Namespace: "default",
				Name:      "kotsadm-api",
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  true,
					IsWarn:  false,
					IsFail:  false,
					Title:   "kotsadm-api Status",
					Message: "pass",
					IconKey: "kubernetes_deployment_status",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17",
				},
			},
			files: map[string][]byte{
				"cluster-resources/deployments/default.json":     []byte(defaultDeployments),
				"cluster-resources/deployments/monitoring.json":  []byte(monitoringDeployments),
				"cluster-resources/deployments/kube-system.json": []byte(kubeSystemDeployments),
			},
		},
		{
			name: "1/1, pass when = 2",
			analyzer: troubleshootv1beta2.DeploymentStatus{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "absent",
							Message: "fail",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "= 2",
							Message: "pass",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "fail",
						},
					},
				},
				Namespace: "default",
				Name:      "kotsadm-api",
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "kotsadm-api Status",
					Message: "fail",
					IconKey: "kubernetes_deployment_status",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17",
				},
			},
			files: map[string][]byte{
				"cluster-resources/deployments/default.json":     []byte(defaultDeployments),
				"cluster-resources/deployments/monitoring.json":  []byte(monitoringDeployments),
				"cluster-resources/deployments/kube-system.json": []byte(kubeSystemDeployments),
			},
		},
		{
			name: "1/1, pass when >= 2, warn when = 1, fail when 0",
			analyzer: troubleshootv1beta2.DeploymentStatus{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "absent",
							Message: "fail",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    ">= 2",
							Message: "pass",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "= 1",
							Message: "warn",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "fail",
						},
					},
				},
				Namespace: "default",
				Name:      "kotsadm-api",
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  true,
					IsFail:  false,
					Title:   "kotsadm-api Status",
					Message: "warn",
					IconKey: "kubernetes_deployment_status",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17",
				},
			},
			files: map[string][]byte{
				"cluster-resources/deployments/default.json":     []byte(defaultDeployments),
				"cluster-resources/deployments/monitoring.json":  []byte(monitoringDeployments),
				"cluster-resources/deployments/kube-system.json": []byte(kubeSystemDeployments),
			},
		},
		{
			name: "multiple namespaces, 2/3",
			analyzer: troubleshootv1beta2.DeploymentStatus{
				Namespaces: []string{"default", "monitoring"},
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "default/kotsadm-web Deployment Status",
					Message: "The deployment default/kotsadm-web has 1/2 replicas",
					IconKey: "kubernetes_deployment_status",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17",
				},
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "monitoring/prometheus-operator Deployment Status",
					Message: "The deployment monitoring/prometheus-operator has 1/2 replicas",
					IconKey: "kubernetes_deployment_status",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17",
				},
			},
			files: map[string][]byte{
				"cluster-resources/deployments/default.json":     []byte(defaultDeployments),
				"cluster-resources/deployments/monitoring.json":  []byte(monitoringDeployments),
				"cluster-resources/deployments/kube-system.json": []byte(kubeSystemDeployments),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			getFiles := func(n string, _ []string) (map[string][]byte, error) {
				if file, ok := test.files[n]; ok {
					return map[string][]byte{n: file}, nil
				}
				return test.files, nil
			}

			actual, err := analyzeDeploymentStatus(&test.analyzer, getFiles)
			req.NoError(err)

			req.Equal(len(test.expectResult), len(actual))
			for _, a := range actual {
				assert.Contains(t, test.expectResult, a)
			}
		})
	}
}
