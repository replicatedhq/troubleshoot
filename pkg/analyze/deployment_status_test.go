package analyzer

import (
	"testing"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.undefinedlabs.com/scopeagent"
)

func Test_deploymentStatus(t *testing.T) {
	test := scopeagent.StartTest(t)
	defer test.End()
	tests := []struct {
		name         string
		analyzer     troubleshootv1beta1.DeploymentStatus
		expectResult AnalyzeResult
		files        map[string][]byte
	}{
		{
			name: "1/1, pass when = 1",
			analyzer: troubleshootv1beta1.DeploymentStatus{
				Outcomes: []*troubleshootv1beta1.Outcome{
					{
						Pass: &troubleshootv1beta1.SingleOutcome{
							When:    "= 1",
							Message: "pass",
						},
					},
					{
						Fail: &troubleshootv1beta1.SingleOutcome{
							Message: "fail",
						},
					},
				},
				Namespace: "default",
				Name:      "kotsadm-api",
			},
			expectResult: AnalyzeResult{
				IsPass:  true,
				IsWarn:  false,
				IsFail:  false,
				Title:   "kotsadm-api Status",
				Message: "pass",
			},
			files: map[string][]byte{
				"cluster-resources/deployments/default.json": []byte(collectedDeployments),
			},
		},
		{
			name: "1/1, pass when = 2",
			analyzer: troubleshootv1beta1.DeploymentStatus{
				Outcomes: []*troubleshootv1beta1.Outcome{
					{
						Pass: &troubleshootv1beta1.SingleOutcome{
							When:    "= 2",
							Message: "pass",
						},
					},
					{
						Fail: &troubleshootv1beta1.SingleOutcome{
							Message: "fail",
						},
					},
				},
				Namespace: "default",
				Name:      "kotsadm-api",
			},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  false,
				IsFail:  true,
				Title:   "kotsadm-api Status",
				Message: "fail",
			},
			files: map[string][]byte{
				"cluster-resources/deployments/default.json": []byte(collectedDeployments),
			},
		},
		{
			name: "1/1, pass when >= 2, warn when = 1, fail when 0",
			analyzer: troubleshootv1beta1.DeploymentStatus{
				Outcomes: []*troubleshootv1beta1.Outcome{
					{
						Pass: &troubleshootv1beta1.SingleOutcome{
							When:    ">= 2",
							Message: "pass",
						},
					},
					{
						Warn: &troubleshootv1beta1.SingleOutcome{
							When:    "= 1",
							Message: "warn",
						},
					},
					{
						Fail: &troubleshootv1beta1.SingleOutcome{
							Message: "fail",
						},
					},
				},
				Namespace: "default",
				Name:      "kotsadm-api",
			},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  true,
				IsFail:  false,
				Title:   "kotsadm-api Status",
				Message: "warn",
			},
			files: map[string][]byte{
				"cluster-resources/deployments/default.json": []byte(collectedDeployments),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			getFiles := func(n string) ([]byte, error) {
				return test.files[n], nil
			}

			actual, err := analyzeDeploymentStatus(&test.analyzer, getFiles)
			req.NoError(err)

			assert.Equal(t, &test.expectResult, actual)

		})
	}
}
