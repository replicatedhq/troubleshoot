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
			name: "pass if ubuntu >= 0.1.2",
			hostInfo: collect.HostOSInfo{
				Name:            "myhost",
				KernelVersion:   "5.4.0-1034-gcp",
				PlatformVersion: "00.1.2",
				Platform:        "ubuntu",
			},
			hostAnalyzer: &troubleshootv1beta2.HostOSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "ubuntu >= 00.1.2",
							Message: "supported distribution matches ubuntu >= 00.1.2",
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
					Message: "supported distribution matches ubuntu >= 00.1.2",
				},
			},
		},

		{
			name: "pass if ubuntu >= 1.0.2",
			hostInfo: collect.HostOSInfo{
				Name:            "myhost",
				KernelVersion:   "5.4.0-1034-gcp",
				PlatformVersion: "1.0.2",
				Platform:        "ubuntu",
			},
			hostAnalyzer: &troubleshootv1beta2.HostOSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "ubuntu >= 1.0.2",
							Message: "supported distribution matches ubuntu >= 1.0.2",
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
					Message: "supported distribution matches ubuntu >= 1.0.2",
				},
			},
		},
		{
			name: "pass if ubuntu >= 1.2.0",
			hostInfo: collect.HostOSInfo{
				Name:            "myhost",
				KernelVersion:   "5.4.0-1034-gcp",
				PlatformVersion: "1.2.0",
				Platform:        "ubuntu",
			},
			hostAnalyzer: &troubleshootv1beta2.HostOSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "ubuntu >= 1.0.2",
							Message: "supported distribution matches ubuntu >= 1.2.0",
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
					Message: "supported distribution matches ubuntu >= 1.2.0",
				},
			},
		},
		{
			name: "pass if ubuntu-1.2.0-kernel >= 1.2.0",
			hostInfo: collect.HostOSInfo{
				Name:            "myhost",
				KernelVersion:   "1.2.0-1034-gcp",
				PlatformVersion: "1.2.0",
				Platform:        "centos",
			},
			hostAnalyzer: &troubleshootv1beta2.HostOSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "centos-1.2.0-kernel >= 1.2.0",
							Message: "supported kernel matches centos-1.2.0-kernel >= 1.2.0",
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
					Message: "supported kernel matches centos-1.2.0-kernel >= 1.2.0",
				},
			},
		},
		{
			name: "pass if ubuntu-0.1.2-kernel >= 0.1.2",
			hostInfo: collect.HostOSInfo{
				Name:            "myhost",
				KernelVersion:   "0.01.2-1034-gcp",
				PlatformVersion: "8.2",
				Platform:        "centos",
			},
			hostAnalyzer: &troubleshootv1beta2.HostOSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "centos-8.2-kernel >= 0.01.2",
							Message: "supported kernel matches centos-8.2-kernel >= 0.01.2",
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
					Message: "supported kernel matches centos-8.2-kernel >= 0.01.2",
				},
			},
		},

		{
			name: "fail if ubuntu <= 11.04",
			hostInfo: collect.HostOSInfo{
				Name:            "myhost",
				KernelVersion:   "5.4.0-1034-gcp",
				PlatformVersion: "11.04",
				Platform:        "ubuntu",
			},
			hostAnalyzer: &troubleshootv1beta2.HostOSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "ubuntu <= 11.04",
							Message: "unsupported ubuntu version 11.04",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "supported distribution",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Host OS Info",
					IsFail:  true,
					Message: "unsupported ubuntu version 11.04",
				},
			},
		},
		{
			name: "fail if none of the kernel distribution versions match",
			hostInfo: collect.HostOSInfo{
				Name:            "my-host",
				KernelVersion:   "4.4",
				PlatformVersion: "18.04",
				Platform:        "ubuntu",
			},
			hostAnalyzer: &troubleshootv1beta2.HostOSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "centos-18.04-kernel > 4.15",
							Message: "supported distribution matches centos-18.04-kernel >= 4.15",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "ubuntu-18.04-kernel > 4.15",
							Message: "supported distribution matches ubuntu-18.04-kernel >= 4.15",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "ubuntu-16.04-kernel == 4.15",
							Message: "supported distribution matches ubuntu-16.04-kernel == 4.15 ",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "None matched, centos-18.04-kernel >= 4.15,ubuntu-18.04-kernel >= 4.15, supported distribution",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Host OS Info",
					IsFail:  true,
					Message: "None matched, centos-18.04-kernel >= 4.15,ubuntu-18.04-kernel >= 4.15, supported distribution",
				},
			},
		},
		/*
			{
				name: "test if centos kernel > 4.15",
				hostInfo: collect.HostOSInfo{
					Name:            "my-host",
					KernelVersion:   "4.15",
					PlatformVersion: "18.04",
					Platform:        "centos",
				},
				hostAnalyzer: &troubleshootv1beta2.HostOSAnalyze{
					Outcomes: []*troubleshootv1beta2.Outcome{
						{
							Pass: &troubleshootv1beta2.SingleOutcome{
								When:    "centos-18.04-kernel >= 4.15",
								Message: "supported distribution matches centos-18.04-kernel >= 4.15",
							},
						},
						{
							Pass: &troubleshootv1beta2.SingleOutcome{
								When:    "ubuntu-18.04-kernel > 4.15",
								Message: "supported distribution matches ubuntu-18.04-kernel >= 4.15",
							},
						},
						{
							Warn: &troubleshootv1beta2.SingleOutcome{
								When:    "ubuntu-16.04-kernel == 4.15",
								Message: "supported distribution matches ubuntu-16.04-kernel == 4.15 ",
							},
						},
						{
							Fail: &troubleshootv1beta2.SingleOutcome{
								Message: "None matched, centos-18.04-kernel >= 4.15,ubuntu-18.04-kernel >= 4.15, supported distribution",
							},
						},
					},
				},
				result: []*AnalyzeResult{
					{
						Title:   "Host OS Info",
						IsPass:  true,
						Message: "supported distribution matches centos-18.04-kernel >= 4.15",
					},
				},
			},
			{
				name: "test ubuntu 16 kernel >= 4.15-abc",
				hostInfo: collect.HostOSInfo{
					Name:            "my-host",
					KernelVersion:   "4.14-abc",
					PlatformVersion: "16.04",
					Platform:        "ubuntu",
				},
				hostAnalyzer: &troubleshootv1beta2.HostOSAnalyze{
					Outcomes: []*troubleshootv1beta2.Outcome{
						{
							Pass: &troubleshootv1beta2.SingleOutcome{
								When:    "ubuntu-16.04-kernel >= 4.14",
								Message: "supported distribution match 4.14",
							},
						},
					},
				},

				result: []*AnalyzeResult{
					{
						Title:   "Host OS Info",
						IsPass:  true,
						Message: "supported distribution match 4.14",
					},
				},
			},
		*/
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
