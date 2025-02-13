package analyzer

import (
	"encoding/json"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeSubnetContainsIP(t *testing.T) {
	tests := []struct {
		name         string
		info         *collect.SubnetContainsIPResult
		hostAnalyzer *troubleshootv1beta2.SubnetContainsIPAnalyze
		result       []*AnalyzeResult
		expectErr    bool
	}{
		{
			name: "ip is in subnet",
			info: &collect.SubnetContainsIPResult{
				CIDR:     "10.0.0.0/8",
				IP:       "10.0.0.5",
				Contains: true,
			},
			hostAnalyzer: &troubleshootv1beta2.SubnetContainsIPAnalyze{
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
			info: &collect.SubnetContainsIPResult{
				CIDR:     "10.0.0.0/8",
				IP:       "192.168.1.1",
				Contains: false,
			},
			hostAnalyzer: &troubleshootv1beta2.SubnetContainsIPAnalyze{
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
			name: "invalid condition",
			info: &collect.SubnetContainsIPResult{
				CIDR:     "10.0.0.0/8",
				IP:       "10.0.0.5",
				Contains: true,
			},
			hostAnalyzer: &troubleshootv1beta2.SubnetContainsIPAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "invalid",
							Message: "this should error",
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
			b, err := json.Marshal(test.info)
			if err != nil {
				t.Fatal(err)
			}

			getCollectedFileContents := func(filename string) ([]byte, error) {
				return b, nil
			}

			result, err := (&AnalyzeHostSubnetContainsIP{test.hostAnalyzer}).Analyze(getCollectedFileContents, nil)
			if test.expectErr {
				req.Error(err)
			} else {
				req.NoError(err)
			}

			assert.Equal(t, test.result, result)
		})
	}
}
