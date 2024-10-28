package analyzer

import (
	"encoding/json"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeHTTPLoadBalancer(t *testing.T) {
	tests := []struct {
		name         string
		info         *collect.NetworkStatusResult
		hostAnalyzer *troubleshootv1beta2.HTTPLoadBalancerAnalyze
		result       []*AnalyzeResult
		expectErr    bool
	}{
		{
			name: "connection refused, fail",
			info: &collect.NetworkStatusResult{
				Status: collect.NetworkStatusConnectionRefused,
			},
			hostAnalyzer: &troubleshootv1beta2.HTTPLoadBalancerAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "connection-refused",
							Message: "Connection was refused",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "connected",
							Message: "Connection was successful",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							Message: "Unexpected HTTP connection status",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "HTTP Load Balancer",
					IsFail:  true,
					Message: "Connection was refused",
				},
			},
		},
		{
			name: "connected, success",
			info: &collect.NetworkStatusResult{
				Status: collect.NetworkStatusConnected,
			},
			hostAnalyzer: &troubleshootv1beta2.HTTPLoadBalancerAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "connection-refused",
							Message: "Connection was refused",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "connected",
							Message: "Connection was successful",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							Message: "Unexpected HTTP connection status",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "HTTP Load Balancer",
					IsPass:  true,
					Message: "Connection was successful",
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

			result, err := (&AnalyzeHostHTTPLoadBalancer{test.hostAnalyzer}).Analyze(getCollectedFileContents, nil)
			if test.expectErr {
				req.Error(err)
			} else {
				req.NoError(err)
			}

			assert.Equal(t, test.result, result)
		})
	}
}
