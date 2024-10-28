package analyzer

import (
	"encoding/json"
	"testing"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func collectorToBytes(collector any) []byte {
	jsonData, _ := json.Marshal(collector)
	return jsonData
}

func TestAnalyzeHostCollectorResults(t *testing.T) {
	tests := []struct {
		name             string
		outcomes         []*troubleshootv1beta2.Outcome
		collectedContent []collectedContent
		expectResult     []*AnalyzeResult
	}{
		{
			name: "pass if ubuntu >= 00.1.2",
			collectedContent: []collectedContent{
				{
					NodeName: "node1",
					Data: collectorToBytes(collect.HostOSInfo{
						Name:            "myhost",
						KernelVersion:   "5.4.0-1034-gcp",
						PlatformVersion: "00.1.2",
						Platform:        "ubuntu",
					}),
				},
			},
			outcomes: []*troubleshootv1beta2.Outcome{
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
			expectResult: []*AnalyzeResult{
				{
					Title:   "Host OS Info - Node node1",
					IsPass:  true,
					Message: "supported distribution matches ubuntu >= 00.1.2",
				},
			},
		},
		{
			name: "fail if ubuntu <= 11.04",
			collectedContent: []collectedContent{
				{
					NodeName: "node1",
					Data: collectorToBytes(collect.HostOSInfo{
						Name:            "myhost",
						KernelVersion:   "5.4.0-1034-gcp",
						PlatformVersion: "11.04",
						Platform:        "ubuntu",
					}),
				},
				{
					NodeName: "node2",
					Data: collectorToBytes(collect.HostOSInfo{
						Name:            "myhost",
						KernelVersion:   "5.4.0-1034-gcp",
						PlatformVersion: "11.04",
						Platform:        "ubuntu",
					}),
				},
			},
			outcomes: []*troubleshootv1beta2.Outcome{
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
			expectResult: []*AnalyzeResult{
				{
					Title:   "Host OS Info - Node node1",
					IsFail:  true,
					Message: "unsupported ubuntu version 11.04",
				},
				{
					Title:   "Host OS Info - Node node2",
					IsFail:  true,
					Message: "unsupported ubuntu version 11.04",
				},
			},
		},
		{
			name: "title does not include node name if empty",
			collectedContent: []collectedContent{
				{
					NodeName: "",
					Data: collectorToBytes(collect.HostOSInfo{
						Name:            "myhost",
						KernelVersion:   "5.4.0-1034-gcp",
						PlatformVersion: "20.04",
						Platform:        "ubuntu",
					}),
				},
			},
			outcomes: []*troubleshootv1beta2.Outcome{
				{
					Pass: &troubleshootv1beta2.SingleOutcome{
						When:    "ubuntu >= 20.04",
						Message: "supported distribution matches ubuntu >= 20.04",
					},
				},
				{
					Fail: &troubleshootv1beta2.SingleOutcome{
						Message: "unsupported distribution",
					},
				},
			},
			expectResult: []*AnalyzeResult{
				{
					Title:   "Host OS Info", // Ensuring the title does not include node name if it's empty
					IsPass:  true,
					Message: "supported distribution matches ubuntu >= 20.04",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a := AnalyzeHostOS{}
			// Call the new analyzeHostCollectorResults function with the test data
			result, err := analyzeHostCollectorResults(
				test.collectedContent,
				test.outcomes,
				a.CheckCondition,
				"Host OS Info",
			)
			require.NoError(t, err)
			assert.Equal(t, test.expectResult, result)
		})
	}
}

func TestEvaluateOutcomes(t *testing.T) {
	tests := []struct {
		name           string
		outcomes       []*troubleshootv1beta2.Outcome
		checkCondition func(string, []byte) (bool, error)
		data           []byte
		expectedResult []*AnalyzeResult
	}{
		{
			name: "fail condition matches",
			outcomes: []*troubleshootv1beta2.Outcome{
				{
					Fail: &troubleshootv1beta2.SingleOutcome{
						When:    "failCondition",
						Message: "failure condition met",
					},
				},
			},
			checkCondition: func(when string, data []byte) (bool, error) {
				// Return true if the condition being checked matches "failCondition"
				return when == "failCondition", nil
			},
			data: []byte("someData"),
			expectedResult: []*AnalyzeResult{
				{
					Title:   "Test Title",
					IsFail:  true,
					Message: "failure condition met",
				},
			},
		},
		{
			name: "warn condition matches",
			outcomes: []*troubleshootv1beta2.Outcome{
				{
					Warn: &troubleshootv1beta2.SingleOutcome{
						When:    "warnCondition",
						Message: "warning condition met",
					},
				},
			},
			checkCondition: func(when string, data []byte) (bool, error) {
				// Return true if the condition being checked matches "warnCondition"
				return when == "warnCondition", nil
			},
			data: []byte("someData"),
			expectedResult: []*AnalyzeResult{
				{
					Title:   "Test Title",
					IsWarn:  true,
					Message: "warning condition met",
				},
			},
		},
		{
			name: "pass condition matches",
			outcomes: []*troubleshootv1beta2.Outcome{
				{
					Pass: &troubleshootv1beta2.SingleOutcome{
						When:    "passCondition",
						Message: "pass condition met",
					},
				},
			},
			checkCondition: func(when string, data []byte) (bool, error) {
				// Return true if the condition being checked matches "passCondition"
				return when == "passCondition", nil
			},
			data: []byte("someData"),
			expectedResult: []*AnalyzeResult{
				{
					Title:   "Test Title",
					IsPass:  true,
					Message: "pass condition met",
				},
			},
		},
		{
			name: "no condition matches",
			outcomes: []*troubleshootv1beta2.Outcome{
				{
					Fail: &troubleshootv1beta2.SingleOutcome{
						When:    "failCondition",
						Message: "failure condition met",
					},
					Warn: &troubleshootv1beta2.SingleOutcome{
						When:    "warnCondition",
						Message: "warning condition met",
					},
					Pass: &troubleshootv1beta2.SingleOutcome{
						When:    "passCondition",
						Message: "pass condition met",
					},
				},
			},
			checkCondition: func(when string, data []byte) (bool, error) {
				// Always return false to simulate no condition matching
				return false, nil
			},
			data:           []byte("someData"),
			expectedResult: nil, // No condition matches, so we expect no results
		},
		{
			name: "error in checkCondition",
			outcomes: []*troubleshootv1beta2.Outcome{
				{
					Fail: &troubleshootv1beta2.SingleOutcome{
						When:    "failCondition",
						Message: "failure condition met",
					},
				},
			},
			checkCondition: func(when string, data []byte) (bool, error) {
				// Simulate an error occurring during condition evaluation
				return false, errors.New("mock error")
			},
			data: []byte("someData"),
			expectedResult: []*AnalyzeResult{
				{
					Title:  "Test Title",
					IsFail: false, // Error occurred, so no success flag
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := evaluateOutcomes(test.outcomes, test.checkCondition, test.data, "Test Title")

			if test.name == "error in checkCondition" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expectedResult, result)
			}
		})
	}
}
