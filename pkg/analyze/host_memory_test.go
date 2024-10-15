package analyzer

import (
	"encoding/json"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeHostMemoryCheckCondition(t *testing.T) {
	tests := []struct {
		name        string
		conditional string
		actual      uint64
		expected    bool
		expectErr   bool
	}{
		{
			name:        "< 16Gi when actual is 8Gi",
			conditional: "< 16Gi",
			actual:      8 * 1024 * 1024 * 1024, // 8GiB
			expected:    true,
			expectErr:   false,
		},
		{
			name:        "< 8Gi when actual is 8Gi",
			conditional: "< 8Gi",
			actual:      8 * 1024 * 1024 * 1024, // 8GiB
			expected:    false,
			expectErr:   false,
		},
		{
			name:        "<= 8Gi when actual is 8Gi",
			conditional: "<= 8Gi",
			actual:      8 * 1024 * 1024 * 1024, // 8GiB
			expected:    true,
			expectErr:   false,
		},
		{
			name:        "<= 8Gi when actual is 16Gi",
			conditional: "<= 8Gi",
			actual:      16 * 1024 * 1024 * 1024, // 16GiB
			expected:    false,
			expectErr:   false,
		},
		{
			name:        "== 8Gi when actual is 16Gi",
			conditional: "== 8Gi",
			actual:      16 * 1024 * 1024 * 1024, // 16GiB
			expected:    false,
			expectErr:   false,
		},
		{
			name:        "== 8Gi when actual is 8Gi",
			conditional: "== 8Gi",
			actual:      8 * 1024 * 1024 * 1024, // 8GiB
			expected:    true,
			expectErr:   false,
		},
		{
			name:        "== 8000000000 when actual is 8000000000",
			conditional: "== 8000000000",
			actual:      8 * 1000 * 1000 * 1000, // 8GB in decimal
			expected:    true,
			expectErr:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create the AnalyzeHostMemory object
			analyzeHostMemory := AnalyzeHostMemory{}

			// Simulate the memory info as JSON-encoded data
			memInfo := collect.MemoryInfo{
				Total: test.actual,
			}
			rawData, err := json.Marshal(memInfo)
			require.NoError(t, err)

			// Call the CheckCondition method
			result, err := analyzeHostMemory.CheckCondition(test.conditional, rawData)
			if test.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expected, result)
			}
		})
	}
}

func TestAnalyzeHostMemory(t *testing.T) {
	tests := []struct {
		name                     string
		hostAnalyzer             *troubleshootv1beta2.MemoryAnalyze
		getCollectedFileContents func(string) ([]byte, error)
		expectedResults          []*AnalyzeResult
		expectedError            string
	}{
		{
			name: "Pass on memory available",
			hostAnalyzer: &troubleshootv1beta2.MemoryAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    ">= 4Gi",
							Message: "System has at least 4Gi of memory",
						},
					},
				},
			},
			getCollectedFileContents: func(filename string) ([]byte, error) {
				memoryInfo := collect.MemoryInfo{
					Total: 8 * 1024 * 1024 * 1024, // 8GiB
				}
				return json.Marshal(memoryInfo)
			},
			expectedResults: []*AnalyzeResult{
				{
					Title:   "Amount of Memory",
					IsPass:  true,
					Message: "System has at least 4Gi of memory",
				},
			},
			expectedError: "",
		},
		{
			name: "Fail on memory available",
			hostAnalyzer: &troubleshootv1beta2.MemoryAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "< 16Gi",
							Message: "System requires at least 16Gi of memory",
						},
					},
				},
			},
			getCollectedFileContents: func(filename string) ([]byte, error) {
				memoryInfo := collect.MemoryInfo{
					Total: 8 * 1024 * 1024 * 1024, // 8GiB
				}
				return json.Marshal(memoryInfo)
			},
			expectedResults: []*AnalyzeResult{
				{
					Title:   "Amount of Memory",
					IsFail:  true,
					Message: "System requires at least 16Gi of memory",
				},
			},
			expectedError: "",
		},
		{
			name: "Warn on memory available",
			hostAnalyzer: &troubleshootv1beta2.MemoryAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "<= 8Gi",
							Message: "System performs best with more than 8Gi of memory",
						},
					},
				},
			},
			getCollectedFileContents: func(filename string) ([]byte, error) {
				memoryInfo := collect.MemoryInfo{
					Total: 8 * 1024 * 1024 * 1024, // 8GiB
				}
				return json.Marshal(memoryInfo)
			},
			expectedResults: []*AnalyzeResult{
				{
					Title:   "Amount of Memory",
					IsWarn:  true,
					Message: "System performs best with more than 8Gi of memory",
				},
			},
			expectedError: "",
		},
		{
			name: "Pass on empty pass predicate",
			hostAnalyzer: &troubleshootv1beta2.MemoryAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "< 8Gi",
							Message: "System requires at least 8Gi of memory",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "",
							Message: "Memory is sufficient",
						},
					},
				},
			},
			getCollectedFileContents: func(filename string) ([]byte, error) {
				memoryInfo := collect.MemoryInfo{
					Total: 16 * 1024 * 1024 * 1024, // 16GiB
				}
				return json.Marshal(memoryInfo)
			},
			expectedResults: []*AnalyzeResult{
				{
					Title:   "Amount of Memory",
					IsPass:  true,
					Message: "Memory is sufficient",
				},
			},
			expectedError: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Set up the AnalyzeHostMemory object
			analyzeHostMemory := AnalyzeHostMemory{
				hostAnalyzer: test.hostAnalyzer,
			}

			// Call the Analyze function
			results, err := analyzeHostMemory.Analyze(test.getCollectedFileContents, nil)

			// Check for errors and compare results
			if test.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expectedResults, results)
			}
		})
	}
}
