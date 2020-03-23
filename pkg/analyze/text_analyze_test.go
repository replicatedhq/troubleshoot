package analyzer

import (
	"fmt"
	"testing"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.undefinedlabs.com/scopeagent"
)

func Test_textAnalyze(t *testing.T) {
	tests := []struct {
		name         string
		analyzer     troubleshootv1beta1.TextAnalyze
		expectResult AnalyzeResult
		files        map[string][]byte
	}{
		{
			name: "success case 1",
			analyzer: troubleshootv1beta1.TextAnalyze{
				Outcomes: []*troubleshootv1beta1.Outcome{
					{
						Pass: &troubleshootv1beta1.SingleOutcome{
							Message: "pass",
						},
					},
					{
						Fail: &troubleshootv1beta1.SingleOutcome{
							Message: "fail",
						},
					},
				},
				CollectorName: "text-collector-1",
				FileName:      "cfile-1.txt",
				RegexPattern:  "succeeded",
			},
			expectResult: AnalyzeResult{
				IsPass:  true,
				IsWarn:  false,
				IsFail:  false,
				Title:   "text-collector-1",
				Message: "pass",
			},
			files: map[string][]byte{
				"text-collector-1/cfile-1.txt": []byte("Yes it all succeeded"),
			},
		},
		{
			name: "failure case 1",
			analyzer: troubleshootv1beta1.TextAnalyze{
				Outcomes: []*troubleshootv1beta1.Outcome{
					{
						Pass: &troubleshootv1beta1.SingleOutcome{
							Message: "success",
						},
					},
					{
						Fail: &troubleshootv1beta1.SingleOutcome{
							Message: "fail",
						},
					},
				},
				CollectorName: "text-collector-2",
				FileName:      "cfile-2.txt",
				RegexPattern:  "succeeded",
			},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  false,
				IsFail:  true,
				Title:   "text-collector-2",
				Message: "fail",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			files: map[string][]byte{
				"text-collector-2/cfile-2.txt": []byte(""),
			},
		},
		{
			name: "success case 2",
			analyzer: troubleshootv1beta1.TextAnalyze{
				Outcomes: []*troubleshootv1beta1.Outcome{
					{
						Pass: &troubleshootv1beta1.SingleOutcome{
							Message: "success",
						},
					},
					{
						Fail: &troubleshootv1beta1.SingleOutcome{
							Message: "fail",
						},
					},
				},
				CollectorName: "text-collector-3",
				FileName:      "cfile-3.txt",
				RegexPattern:  "",
			},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  false,
				IsFail:  true,
				Title:   "text-collector-3",
				Message: "Invalid analyzer",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			files: map[string][]byte{
				"text-collector-3/cfile-3.txt": []byte("Connection to service succeeded"),
			},
		},
		{
			name: "success case 3",
			analyzer: troubleshootv1beta1.TextAnalyze{
				Outcomes: []*troubleshootv1beta1.Outcome{
					{
						Pass: &troubleshootv1beta1.SingleOutcome{
							Message: "success",
						},
					},
					{
						Fail: &troubleshootv1beta1.SingleOutcome{
							Message: "fail",
						},
					},
				},
				CollectorName: "text-collector-5",
				FileName:      "cfile-5.txt",
				RegexPattern:  "([a-zA-Z0-9\\-_:*\\s])*succe([a-zA-Z0-9\\-_:*\\s!])*",
			},
			expectResult: AnalyzeResult{
				IsPass:  true,
				IsWarn:  false,
				IsFail:  false,
				Title:   "text-collector-5",
				Message: "success",
			},
			files: map[string][]byte{
				"text-collector-5/cfile-5.txt": []byte("Connection to service succeeded!"),
			},
		},
		{
			name: "failure case 3",
			analyzer: troubleshootv1beta1.TextAnalyze{
				Outcomes: []*troubleshootv1beta1.Outcome{
					{
						Pass: &troubleshootv1beta1.SingleOutcome{
							Message: "success",
						},
					},
					{
						Fail: &troubleshootv1beta1.SingleOutcome{
							Message: "fail",
						},
					},
				},
				CollectorName: "text-collector-4",
				FileName:      "cfile-4.txt",
				RegexPattern:  "succeeded",
			},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  false,
				IsFail:  true,
				Title:   "text-collector-4",
				Message: "fail",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			files: map[string][]byte{
				"text-collector-4/cfile-4.txt": []byte("A different message"),
			},
		},
		{
			name: "failure case 4",
			analyzer: troubleshootv1beta1.TextAnalyze{
				Outcomes: []*troubleshootv1beta1.Outcome{
					{
						Pass: &troubleshootv1beta1.SingleOutcome{
							Message: "success",
						},
					},
					{
						Fail: &troubleshootv1beta1.SingleOutcome{
							Message: "fail",
						},
					},
				},
				CollectorName: "text-collector-6",
				FileName:      "cfile-6.txt",
				RegexPattern:  "([a-zA-Z0-9\\-_:*\\s])*succe([a-zA-Z0-9\\-_:*\\s!])*",
			},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  false,
				IsFail:  true,
				Title:   "text-collector-6",
				Message: "fail",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			files: map[string][]byte{
				"text-collector-6/cfile-6.txt": []byte("A different message"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scopetest := scopeagent.StartTest(t)
			defer scopetest.End()
			req := require.New(t)

			getFiles := func(n string) ([]byte, error) {
				val, ok := test.files[n]
				if !ok {
					return nil, fmt.Errorf("File not found: %s", n)
				}
				return val, nil
			}

			actual, err := analyzeTextAnalyze(&test.analyzer, getFiles)
			req.NoError(err)
			assert.Equal(t, &test.expectResult, actual)
		})
	}
}

func Test_compareRegex(t *testing.T) {
	tests := []struct {
		name         string
		conditional  string
		foundMatches map[string]string
		expected     bool
	}{
		{
			name:        "Loss < 5",
			conditional: "Loss < 5",
			foundMatches: map[string]string{
				"Transmitted": "5",
				"Received":    "4",
				"Loss":        "20",
			},
			expected: false,
		},
		{
			name:        "Hostname = icecream",
			conditional: "Hostname = icecream",
			foundMatches: map[string]string{
				"Something": "5",
				"Hostname":  "icecream",
			},
			expected: true,
		},
		{
			name:        "Day >= 23",
			conditional: "Day >= 23",
			foundMatches: map[string]string{
				"day": "5",
				"Day": "24",
			},
			expected: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scopetest := scopeagent.StartTest(t)
			defer scopetest.End()
			req := require.New(t)

			actual, err := compareRegex(test.conditional, test.foundMatches)
			req.NoError(err)

			assert.Equal(t, test.expected, actual)
		})
	}
}
