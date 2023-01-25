package analyzer

import (
	"encoding/json"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeUDPPortStatus(t *testing.T) {
	tt := []struct {
		name      string
		collected collect.NetworkStatusResult
		analyzer  troubleshootv1beta2.UDPPortStatusAnalyze
		results   []*AnalyzeResult
		wantErr   bool
	}{
		{
			name: "pass expect single result",
			collected: collect.NetworkStatusResult{
				Status: "connected",
			},
			analyzer: troubleshootv1beta2.UDPPortStatusAnalyze{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "Kubernetes API UDP Port Status",
				},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "connected",
							Message: "Port 6443 is open",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							Message: "Unexpected port status",
						},
					},
				},
			},
			results: []*AnalyzeResult{
				{
					Title:   "Kubernetes API UDP Port Status",
					IsPass:  true,
					Message: "Port 6443 is open",
				},
			},
		},

		{
			name: "get warn if no match",
			collected: collect.NetworkStatusResult{
				Status: "connected",
			},
			analyzer: troubleshootv1beta2.UDPPortStatusAnalyze{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "Kubernetes API UDP Port Status",
				},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "foo",
							Message: "Port 6443 is open",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							Message: "Unexpected port status",
						},
					},
				},
			},
			results: []*AnalyzeResult{
				{
					Title:   "Kubernetes API UDP Port Status",
					IsWarn:  true,
					Message: "Unexpected port status",
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			fn := func(_ string) ([]byte, error) {
				return json.Marshal(&tc.collected)
			}
			analyzer := AnalyzeHostUDPPortStatus{
				hostAnalyzer: &tc.analyzer,
			}
			results, err := analyzer.Analyze(fn)
			if tc.wantErr {
				require.NotNil(t, err)
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.results, results)
		})
	}

}
