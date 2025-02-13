package analyzer

import (
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeSubnetContainsIP(t *testing.T) {
	tests := []struct {
		name         string
		hostAnalyzer *troubleshootv1beta2.SubnetContainsIPAnalyze
		result       []*AnalyzeResult
		expectErr    bool
	}{
		{
			name: "ip is in subnet",
			hostAnalyzer: &troubleshootv1beta2.SubnetContainsIPAnalyze{
				CIDR: "10.0.0.0/8",
				IP:   "10.0.0.5",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "true",
							Message: "IP address is in subnet",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "false",
							Message: "IP address is not in subnet",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Subnet Contains IP",
					IsPass:  true,
					Message: "IP address is in subnet",
				},
			},
		},
		{
			name: "ip is not in subnet",
			hostAnalyzer: &troubleshootv1beta2.SubnetContainsIPAnalyze{
				CIDR: "10.0.0.0/8",
				IP:   "192.168.1.1",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "true",
							Message: "IP address is in subnet",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "false",
							Message: "IP address is not in subnet",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Subnet Contains IP",
					IsFail:  true,
					Message: "IP address is not in subnet",
				},
			},
		},
		{
			name: "invalid CIDR",
			hostAnalyzer: &troubleshootv1beta2.SubnetContainsIPAnalyze{
				CIDR: "invalid",
				IP:   "10.0.0.5",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "true",
							Message: "IP address is in subnet",
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "invalid IP",
			hostAnalyzer: &troubleshootv1beta2.SubnetContainsIPAnalyze{
				CIDR: "10.0.0.0/8",
				IP:   "invalid",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "true",
							Message: "IP address is in subnet",
						},
					},
				},
			},
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			result, err := (&AnalyzeHostSubnetContainsIP{test.hostAnalyzer}).Analyze(nil, nil)
			if test.expectErr {
				req.Error(err)
				return
			}
			req.NoError(err)

			assert.Equal(t, test.result, result)
		})
	}
}
