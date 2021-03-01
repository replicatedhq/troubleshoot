package analyzer

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_textAnalyze(t *testing.T) {
	tests := []struct {
		name         string
		analyzer     troubleshootv1beta2.TextAnalyze
		expectResult []AnalyzeResult
		files        map[string][]byte
	}{
		{
			name: "success case 1",
			analyzer: troubleshootv1beta2.TextAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "pass",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "fail",
						},
					},
				},
				CollectorName: "text-collector-1",
				FileName:      "cfile-1.txt",
				RegexPattern:  "succeeded",
			},
			expectResult: []AnalyzeResult{
				{
					IsPass:  true,
					IsWarn:  false,
					IsFail:  false,
					Title:   "text-collector-1",
					Message: "pass",
					IconKey: "kubernetes_text_analyze",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
				},
			},
			files: map[string][]byte{
				"text-collector-1/cfile-1.txt": []byte("Yes it all succeeded"),
			},
		},
		{
			name: "failure case 1",
			analyzer: troubleshootv1beta2.TextAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "success",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "fail",
						},
					},
				},
				CollectorName: "text-collector-2",
				FileName:      "cfile-2.txt",
				RegexPattern:  "succeeded",
			},
			expectResult: []AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "text-collector-2",
					Message: "fail",
					IconKey: "kubernetes_text_analyze",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
				},
			},
			files: map[string][]byte{
				"text-collector-2/cfile-2.txt": []byte(""),
			},
		},
		{
			name: "success case 2",
			analyzer: troubleshootv1beta2.TextAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "success",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "fail",
						},
					},
				},
				CollectorName: "text-collector-3",
				FileName:      "cfile-3.txt",
				RegexPattern:  "",
			},
			expectResult: []AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "text-collector-3",
					Message: "Invalid analyzer",
					IconKey: "kubernetes_text_analyze",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
				},
			},
			files: map[string][]byte{
				"text-collector-3/cfile-3.txt": []byte("Connection to service succeeded"),
			},
		},
		{
			name: "success case 3",
			analyzer: troubleshootv1beta2.TextAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "success",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "fail",
						},
					},
				},
				CollectorName: "text-collector-5",
				FileName:      "cfile-5.txt",
				RegexPattern:  "([a-zA-Z0-9\\-_:*\\s])*succe([a-zA-Z0-9\\-_:*\\s!])*",
			},
			expectResult: []AnalyzeResult{
				{
					IsPass:  true,
					IsWarn:  false,
					IsFail:  false,
					Title:   "text-collector-5",
					Message: "success",
					IconKey: "kubernetes_text_analyze",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
				},
			},
			files: map[string][]byte{
				"text-collector-5/cfile-5.txt": []byte("Connection to service succeeded!"),
			},
		},
		{
			name: "failure case 3",
			analyzer: troubleshootv1beta2.TextAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "success",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "fail",
						},
					},
				},
				CollectorName: "text-collector-4",
				FileName:      "cfile-4.txt",
				RegexPattern:  "succeeded",
			},
			expectResult: []AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "text-collector-4",
					Message: "fail",
					IconKey: "kubernetes_text_analyze",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
				},
			},
			files: map[string][]byte{
				"text-collector-4/cfile-4.txt": []byte("A different message"),
			},
		},
		{
			name: "failure case 4",
			analyzer: troubleshootv1beta2.TextAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "success",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "fail",
						},
					},
				},
				CollectorName: "text-collector-6",
				FileName:      "cfile-6.txt",
				RegexPattern:  "([a-zA-Z0-9\\-_:*\\s])*succe([a-zA-Z0-9\\-_:*\\s!])*",
			},
			expectResult: []AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "text-collector-6",
					Message: "fail",
					IconKey: "kubernetes_text_analyze",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
				},
			},
			files: map[string][]byte{
				"text-collector-6/cfile-6.txt": []byte("A different message"),
			},
		},
		{
			name: "multiple results case 1",
			analyzer: troubleshootv1beta2.TextAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "pass",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "fail",
						},
					},
				},
				CollectorName: "text-collector-1",
				FileName:      "cfile",
				RegexPattern:  "succeeded",
			},
			expectResult: []AnalyzeResult{
				{
					IsPass:  true,
					IsWarn:  false,
					IsFail:  false,
					Title:   "text-collector-1",
					Message: "pass",
					IconKey: "kubernetes_text_analyze",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
				},
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "text-collector-1",
					Message: "fail",
					IconKey: "kubernetes_text_analyze",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
				},
			},
			files: map[string][]byte{
				"text-collector-1/cfile-1.txt": []byte("Yes it all succeeded"),
				"text-collector-1/cfile-2.txt": []byte("no success here"),
				"text-collector-2/cfile-3.txt": []byte("Yes it all succeeded"),
			},
		},
		{
			name: "multiple results case 2 globbing",
			analyzer: troubleshootv1beta2.TextAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "pass",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "fail",
						},
					},
				},
				CollectorName: "text-collector-1",
				FileName:      "cfile*.log",
				RegexPattern:  "succeeded",
			},
			expectResult: []AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "text-collector-1",
					Message: "fail",
					IconKey: "kubernetes_text_analyze",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
				},
			},
			files: map[string][]byte{
				"text-collector-1/cfile-1.txt": []byte("Yes it all succeeded"),
				"text-collector-1/cfile-2.log": []byte("no success here"),
				"text-collector-2/cfile-3.txt": []byte("Yes it all succeeded"),
			},
		},
		{
			name: "case insensitive failure case 1", // regexes are not case insensitive by default
			analyzer: troubleshootv1beta2.TextAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "pass",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "fail",
						},
					},
				},
				CollectorName: "text-collector-1",
				FileName:      "cfile-1.txt",
				RegexPattern:  "succeeded",
			},
			expectResult: []AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "text-collector-1",
					Message: "fail",
					IconKey: "kubernetes_text_analyze",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
				},
			},
			files: map[string][]byte{
				"text-collector-1/cfile-1.txt": []byte("Yes it all SUCCEEDED"),
			},
		},
		{
			name: "case insensitive success case 1",
			analyzer: troubleshootv1beta2.TextAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "pass",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "fail",
						},
					},
				},
				CollectorName: "text-collector-1",
				FileName:      "cfile-1.txt",
				RegexPattern:  "(?i)succeeded",
			},
			expectResult: []AnalyzeResult{
				{
					IsPass:  true,
					IsWarn:  false,
					IsFail:  false,
					Title:   "text-collector-1",
					Message: "pass",
					IconKey: "kubernetes_text_analyze",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
				},
			},
			files: map[string][]byte{
				"text-collector-1/cfile-1.txt": []byte("Yes it all SUCCEEDED"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			getFiles := func(n string) (map[string][]byte, error) {
				matching := make(map[string][]byte)
				for k, v := range test.files {
					if strings.HasPrefix(k, n) {
						matching[k] = v
					}
				}

				for k, v := range test.files {
					if ok, _ := filepath.Match(n, k); ok {
						matching[k] = v
					}
				}

				if len(matching) == 0 {
					return nil, fmt.Errorf("File not found: %s", n)
				}
				return matching, nil
			}

			actual, err := analyzeTextAnalyze(&test.analyzer, getFiles)
			req.NoError(err)

			unPointered := []AnalyzeResult{}
			for _, v := range actual {
				unPointered = append(unPointered, *v)
			}
			assert.ElementsMatch(t, test.expectResult, unPointered)
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
			req := require.New(t)

			actual, err := compareRegex(test.conditional, test.foundMatches)
			req.NoError(err)

			assert.Equal(t, test.expected, actual)
		})
	}
}
