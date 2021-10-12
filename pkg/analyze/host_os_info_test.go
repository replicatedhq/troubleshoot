package analyzer

import (
	"encoding/json"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeHostOS(t *testing.T) {
	tests := []struct {
		name         string
		hostInfo     collect.HostOSInfo
		hostAnalyzer *troubleshootv1beta2.HostOSAnalyze
		result       []*AnalyzeResult
		expectErr    bool
	}{
		{
			name: "platform == expected distribution",
			hostInfo: collect.HostOSInfo{
				Name:            "myhost",
				KernelVersion:   "5.4.0-1034-gcp",
				PlatformVersion: "18.04",
				Platform:        "ubuntu",
			},
			hostAnalyzer: &troubleshootv1beta2.HostOSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "ubuntu == 18.04",
							Message: "supported distribution",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "unsupported distribution",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Host OS Info",
					IsPass:  true,
					Message: "supported distribution",
				},
			},
		},
		{
			name: "platform == expected distribution",
			hostInfo: collect.HostOSInfo{
				Name:            "myhost",
				KernelVersion:   "5.4.0-1034-gcp",
				PlatformVersion: "20.04",
				Platform:        "ubuntu",
			},
			hostAnalyzer: &troubleshootv1beta2.HostOSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "ubuntu == 20.04",
							Message: "supported distribution",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "unsupported distribution",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Host OS Info",
					IsPass:  true,
					Message: "supported distribution",
				},
			},
		},
		{
			name: "platform == unsupported but distribution",
			hostInfo: collect.HostOSInfo{
				Name:            "myhost",
				KernelVersion:   "5.4.0-1034-gcp",
				PlatformVersion: "18.04",
				Platform:        "ubuntu",
			},
			hostAnalyzer: &troubleshootv1beta2.HostOSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "ubuntu == 11.04",
							Message: "unsupported distribution",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "unsupported distribution",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Host OS Info",
					IsFail:  true,
					Message: "unsupported distribution",
				},
			},
		},
		{
			name: "test ubuntu 18 kernel >= 4.15",
			hostInfo: collect.HostOSInfo{
				Name:            "my-host",
				KernelVersion:   "5.4",
				PlatformVersion: "18.04",
				Platform:        "ubuntu",
			},
			hostAnalyzer: &troubleshootv1beta2.HostOSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "ubuntu-18.04-kernel >= 4.15",
							Message: "supported distribution",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Host OS Info",
					IsPass:  true,
					Message: "supported distribution",
				},
			},
		},
		{
			expectErr: true,
			name:      "test ubuntu 16 kernel < 4.15",
			hostInfo: collect.HostOSInfo{
				Name:            "my-host",
				KernelVersion:   "4",
				PlatformVersion: "16.04",
				Platform:        "ubuntu",
			},
			hostAnalyzer: &troubleshootv1beta2.HostOSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "ubuntu-16.04-kernel < 4.15",
							Message: "unsupported distribution",
						},
					},
				},
			},

			result: []*AnalyzeResult{
				{
					Title:   "Host OS Info",
					IsFail:  true,
					Message: "unsupported distribution",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)
			b, err := json.Marshal(test.hostInfo)
			if err != nil {
				t.Fatal(err)
			}

			getCollectedFileContents := func(filename string) ([]byte, error) {
				return b, nil
			}

			result, err := (&AnalyzeHostOS{test.hostAnalyzer}).Analyze(getCollectedFileContents)
			if test.expectErr {
				req.Error(err)
			} else {
				req.NoError(err)
			}

			assert.Equal(t, test.result, result)
		})
	}
}
