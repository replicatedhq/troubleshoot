package analyzer

import (
	"testing"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_deploymentStatus(t *testing.T) {
	tests := []struct {
		name         string
		analyzer     troubleshootv1beta1.DeploymentStatus
		expectResult AnalyzeResult
		files        map[string][]byte
	}{
		{
			name: "1/1, = 1",
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
				Title:   "",
				Message: "pass",
			},
			files: map[string][]byte{
				"cluster-resources/deployments/default.json": []byte(collectedDeployments),
			},
		},
		{
			name: "1/1, = 2",
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
				Title:   "",
				Message: "fail",
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

			actual, err := deploymentStatus(&test.analyzer, getFiles)
			req.NoError(err)

			assert.Equal(t, &test.expectResult, actual)

		})
	}
}
