package analyzer

import (
	"encoding/json"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeSubnetAvailable(t *testing.T) {
	tests := []struct {
		name         string
		info         *collect.SubnetAvailableResult
		hostAnalyzer *troubleshootv1beta2.SubnetAvailableAnalyze
		result       []*AnalyzeResult
		expectedErr  bool
	}{
		{
			name: "subnet available",
			info: &collect.SubnetAvailableResult{
				CIDRRangeAlloc: "10.0.0.0/8",
				DesiredCIDR:    22,
				Status:         collect.SubnetStatusAvailable,
			},
			hostAnalyzer: &troubleshootv1beta2.SubnetAvailableAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "a-subnet-is-available",
							Message: "available /22 subnet found",
						},
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "no-subnet-available",
							Message: "failed to find available subnet",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Subnet Available",
					IsPass:  true,
					Message: "available /22 subnet found",
				},
			},
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

			result, err := (&AnalyzeHostSubnetAvailable{test.hostAnalyzer}).Analyze(getCollectedFileContents, nil)
			if test.expectedErr {
				req.Error(err)
			} else {
				req.NoError(err)
			}

			assert.Equal(t, test.result, result)
		})
	}
}
