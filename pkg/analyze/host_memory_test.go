package analyzer

import (
	"encoding/json"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_doCompareHostMemory(t *testing.T) {
	tests := []struct {
		name        string
		conditional string
		actual      uint64
		expected    bool
	}{
		{
			name:        "< 16Gi when actual is 8Gi",
			conditional: "< 16Gi",
			actual:      8 * 1024 * 1024 * 1024,
			expected:    true,
		},
		{
			name:        "< 8Gi when actual is 8Gi",
			conditional: "< 8Gi",
			actual:      8 * 1024 * 1024 * 1024,
			expected:    false,
		},
		{
			name:        "<= 8Gi when actual is 8Gi",
			conditional: "<= 8Gi",
			actual:      8 * 1024 * 1024 * 1024,
			expected:    true,
		},
		{
			name:        "<= 8Gi when actual is 16Gi",
			conditional: "<= 8Gi",
			actual:      16 * 1024 * 1024 * 1024,
			expected:    false,
		},
		{
			name:        "== 8Gi when actual is 16Gi",
			conditional: "== 8Gi",
			actual:      16 * 1024 * 1024 * 1024,
			expected:    false,
		},
		{
			name:        "== 8Gi when actual is 8Gi",
			conditional: "== 8Gi",
			actual:      8 * 1024 * 1024 * 1024,
			expected:    true,
		},
		{
			name:        "== 8000000000 when actual is 8000000000",
			conditional: "== 8000000000",
			actual:      8 * 1000 * 1000 * 1000,
			expected:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			actual, err := compareHostMemoryConditionalToActual(test.conditional, test.actual)
			req.NoError(err)

			assert.Equal(t, test.expected, actual)

		})
	}
}

func TestAnalyzeHostMemory(t *testing.T) {
	tests := []struct {
		name         string
		memoryInfo   *collect.MemoryInfo
		hostAnalyzer *troubleshootv1beta2.MemoryAnalyze
		result       *AnalyzeResult
		expectErr    bool
	}{
		{
			name: "Pass on memory available",
			memoryInfo: &collect.MemoryInfo{
				Total: 8 * 1024 * 1024 * 1024,
			},
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
			result: &AnalyzeResult{
				Title:   "Amount of Memory",
				IsPass:  true,
				Message: "System has at least 4Gi of memory",
			},
		},
		{
			name: "Fail on memory available",
			memoryInfo: &collect.MemoryInfo{
				Total: 8 * 1024 * 1024 * 1024,
			},
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
			result: &AnalyzeResult{
				Title:   "Amount of Memory",
				IsFail:  true,
				Message: "System requires at least 16Gi of memory",
			},
		},
		{
			name: "Warn on memory available",
			memoryInfo: &collect.MemoryInfo{
				Total: 8 * 1024 * 1024 * 1024,
			},
			hostAnalyzer: &troubleshootv1beta2.MemoryAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "< 4Gi",
							Message: "System requires at least 4Gi of memory",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "<= 8Gi",
							Message: "System performs best with more than 8Gi of memory",
						},
					},
				},
			},
			result: &AnalyzeResult{
				Title:   "Amount of Memory",
				IsWarn:  true,
				Message: "System performs best with more than 8Gi of memory",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)
			b, err := json.Marshal(test.memoryInfo)
			if err != nil {
				t.Fatal(err)
			}

			getCollectedFileContents := func(filename string) ([]byte, error) {
				return b, nil
			}

			result, err := (&AnalyzeHostMemory{test.hostAnalyzer}).Analyze(getCollectedFileContents)
			if test.expectErr {
				req.Error(err)
			} else {
				req.NoError(err)
			}

			assert.Equal(t, test.result, result)
		})
	}
}
