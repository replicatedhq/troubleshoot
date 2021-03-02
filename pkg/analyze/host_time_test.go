package analyzer

import (
	"encoding/json"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeHostTime(t *testing.T) {
	tests := []struct {
		name         string
		timeInfo     *collect.TimeInfo
		hostAnalyzer *troubleshootv1beta2.TimeAnalyze
		result       *AnalyzeResult
		expectErr    bool
	}{
		{
			name: "ntp == synchronized+active",
			timeInfo: &collect.TimeInfo{
				Timezone:        "UTC",
				NTPSynchronized: true,
				NTPActive:       true,
			},
			hostAnalyzer: &troubleshootv1beta2.TimeAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "ntp == unsynchronized+inactive",
							Message: "System clock not synchronized",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "ntp == synchronized+active",
							Message: "System clock synchronized and NTP is active",
						},
					},
				},
			},
			result: &AnalyzeResult{
				Title:   "Time",
				IsPass:  true,
				Message: "System clock synchronized and NTP is active",
			},
		},
		{
			name: "ntp == unsynchronized+inactive",
			timeInfo: &collect.TimeInfo{
				Timezone:        "UTC",
				NTPSynchronized: false,
				NTPActive:       false,
			},
			hostAnalyzer: &troubleshootv1beta2.TimeAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "ntp == unsynchronized+inactive",
							Message: "System clock not synchronized",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "ntp == synchronized+active",
							Message: "System clock synchronized and NTP is active",
						},
					},
				},
			},
			result: &AnalyzeResult{
				Title:   "Time",
				IsFail:  true,
				Message: "System clock not synchronized",
			},
		},
		{
			name: "ntp == unsynchronized+active",
			timeInfo: &collect.TimeInfo{
				Timezone:        "UTC",
				NTPSynchronized: false,
				NTPActive:       true,
			},
			hostAnalyzer: &troubleshootv1beta2.TimeAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "ntp == unsynchronized+inactive",
							Message: "System clock not synchronized",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "ntp == unsynchronized+active",
							Message: "System clock not yet synchronized",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "ntp == synchronized+active",
							Message: "System clock synchronized and NTP is active",
						},
					},
				},
			},
			result: &AnalyzeResult{
				Title:   "Time",
				IsWarn:  true,
				Message: "System clock not yet synchronized",
			},
		},
		{
			name: "ntp == synchronized+inactive",
			timeInfo: &collect.TimeInfo{
				Timezone:        "UTC",
				NTPSynchronized: true,
				NTPActive:       false,
			},
			hostAnalyzer: &troubleshootv1beta2.TimeAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "ntp == unsynchronized+inactive",
							Message: "System clock not synchronized",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "ntp == unsynchronized+active",
							Message: "System clock not yet synchronized",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "ntp == synchronized+inactive",
							Message: "System clock synchronized for now",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "ntp == synchronized+active",
							Message: "System clock synchronized and NTP is active",
						},
					},
				},
			},
			result: &AnalyzeResult{
				Title:   "Time",
				IsWarn:  true,
				Message: "System clock synchronized for now",
			},
		},
		{
			name: "timezone",
			timeInfo: &collect.TimeInfo{
				Timezone:        "UTC",
				NTPSynchronized: true,
				NTPActive:       true,
			},
			hostAnalyzer: &troubleshootv1beta2.TimeAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "timezone == UTC",
							Message: "Timezone is set to UTC",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "timezone not set to UTC",
						},
					},
				},
			},
			result: &AnalyzeResult{
				Title:   "Time",
				IsPass:  true,
				Message: "Timezone is set to UTC",
			},
		},
		{
			name: "timezone is not UTC",
			timeInfo: &collect.TimeInfo{
				Timezone:        "PST",
				NTPSynchronized: true,
				NTPActive:       true,
			},
			hostAnalyzer: &troubleshootv1beta2.TimeAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "timezone != UTC",
							Message: "Timezone is not set to UTC",
						},
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "Timezone is set to UTC",
						},
					},
				},
			},
			result: &AnalyzeResult{
				Title:   "Time",
				IsFail:  true,
				Message: "Timezone is not set to UTC",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)
			b, err := json.Marshal(test.timeInfo)
			if err != nil {
				t.Fatal(err)
			}

			getCollectedFileContents := func(filename string) ([]byte, error) {
				return b, nil
			}

			result, err := (&AnalyzeHostTime{test.hostAnalyzer}).Analyze(getCollectedFileContents)
			if test.expectErr {
				req.Error(err)
			} else {
				req.NoError(err)
			}

			assert.Equal(t, test.result, result)
		})
	}
}
