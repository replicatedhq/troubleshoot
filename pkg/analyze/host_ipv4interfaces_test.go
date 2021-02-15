package analyzer

import (
	"encoding/json"
	"net"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeIPV4Interfaces(t *testing.T) {
	tests := []struct {
		name         string
		interfaces   []net.Interface
		hostAnalyzer *troubleshootv1beta2.IPV4InterfacesAnalyze
		result       *AnalyzeResult
		expectErr    bool
	}{
		{
			name:       "fail when no ipv4 interfaces detected",
			interfaces: nil,
			hostAnalyzer: &troubleshootv1beta2.IPV4InterfacesAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "count > 0",
							Message: "IPv4 interface available",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "count == 0",
							Message: "No IPv4 interfaces detected",
						},
					},
				},
			},
			result: &AnalyzeResult{
				Title:   "IPv4 Interfaces",
				IsFail:  true,
				Message: "No IPv4 interfaces detected",
			},
		},
		{
			name: "pass when ipv4 interfaces detected",
			interfaces: []net.Interface{
				{
					Index:        1,
					MTU:          1460,
					HardwareAddr: net.HardwareAddr("42010a80001d"),
					Name:         "ens4",
				},
			},
			hostAnalyzer: &troubleshootv1beta2.IPV4InterfacesAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "count == 0",
							Message: "No IPv4 interfaces detected",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "count > 0",
							Message: "IPv4 interface available",
						},
					},
				},
			},
			result: &AnalyzeResult{
				Title:   "IPv4 Interfaces",
				IsPass:  true,
				Message: "IPv4 interface available",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)
			b, err := json.Marshal(test.interfaces)
			if err != nil {
				t.Fatal(err)
			}

			getCollectedFileContents := func(filename string) ([]byte, error) {
				return b, nil
			}

			result, err := analyzeHostIPV4Interfaces(test.hostAnalyzer, getCollectedFileContents)
			if test.expectErr {
				req.Error(err)
			} else {
				req.NoError(err)
			}

			assert.Equal(t, test.result, result)
		})
	}
}
