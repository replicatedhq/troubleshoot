package analyzer

import (
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_compareSortedSlices(t *testing.T) {
	type args struct {
		actual   []interface{}
		expected []interface{}
	}

	tests := []struct {
		name  string
		args  args
		equal bool
	}{
		{
			name: "empty slices",
			args: args{
				actual:   []interface{}{},
				expected: []interface{}{},
			},
			equal: true,
		},
		{
			name: "same order slices",
			args: args{
				actual:   []interface{}{"a", "b", "c"},
				expected: []interface{}{"a", "b", "c"},
			},
			equal: true,
		},
		{
			name: "unordered slices",
			args: args{
				actual:   []interface{}{"a", "b", "c"},
				expected: []interface{}{"b", "a", "c"},
			},
			equal: true,
		},
		{
			name: "different type and unordered slices",
			args: args{
				actual:   []interface{}{1, "a", "c"},
				expected: []interface{}{"a", 1, "c"},
			},
			equal: true,
		},
		{
			name: "unordered slices with map",
			args: args{
				actual:   []interface{}{map[string]int{"a": 1}, "a", "c"},
				expected: []interface{}{"a", map[string]int{"a": 1}, "c"},
			},
			equal: true,
		},
		{
			name: "unequal slices with duplicates",
			args: args{
				actual:   []interface{}{"a", "a", "a", "c"},
				expected: []interface{}{"a", "a", "c", "c"},
			},
			equal: false,
		},
		{
			name: "unordered slices with boolean and strings",
			args: args{
				actual:   []interface{}{true, "a", false, true},
				expected: []interface{}{"a", true, false, true},
			},
			equal: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareSortedSlices(tt.args.actual, tt.args.expected)
			assert.Equalf(t, tt.equal, got, "compareSlices() = %v, want %v", got, tt.equal)
		})
	}
}

func Test_jsonCompare(t *testing.T) {
	tests := []struct {
		name         string
		isError      bool
		analyzer     troubleshootv1beta2.JsonCompare
		expectResult AnalyzeResult
		fileContents []byte
	}{
		{
			name: "basic comparison",
			analyzer: troubleshootv1beta2.JsonCompare{
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
				CollectorName: "json-compare-1",
				FileName:      "json-compare-1.json",
				Value: `{
					"foo": "bar",
					"stuff": {
						"foo": "bar",
						"bar": true
					},
					"morestuff": [
						{
							"foo": {
								"bar": 123
							}
						}
					]
				}`,
			},
			expectResult: AnalyzeResult{
				IsPass:  true,
				IsWarn:  false,
				IsFail:  false,
				Title:   "json-compare-1",
				Message: "pass",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			fileContents: []byte(`{
				"foo": "bar",
				"stuff": {
					"foo": "bar",
					"bar": true
				},
				"morestuff": [
					{
						"foo": {
							"bar": 123
						}
					}
				]
			}`),
		},
		{
			name: "basic comparison, but fail on match",
			analyzer: troubleshootv1beta2.JsonCompare{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "pass",
							When:    "false",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "fail",
							When:    "true",
						},
					},
				},
				CollectorName: "json-compare-1-1",
				FileName:      "json-compare-1-1.json",
				Value: `{
					"foo": "bar",
					"stuff": {
						"foo": "bar",
						"bar": true
					},
					"morestuff": [
						{
							"foo": {
								"bar": 123
							}
						}
					]
				}`,
			},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  false,
				IsFail:  true,
				Title:   "json-compare-1-1",
				Message: "fail",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			fileContents: []byte(`{
				"foo": "bar",
				"stuff": {
					"foo": "bar",
					"bar": true
				},
				"morestuff": [
					{
						"foo": {
							"bar": 123
						}
					}
				]
			}`),
		},
		{
			name: "comparison using path 1",
			analyzer: troubleshootv1beta2.JsonCompare{
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
				CollectorName: "json-compare-2",
				FileName:      "json-compare-2.json",
				Path:          "morestuff",
				Value: `[
					{
						"foo": {
							"bar": 123
						}
					}
				]`,
			},
			expectResult: AnalyzeResult{
				IsPass:  true,
				IsWarn:  false,
				IsFail:  false,
				Title:   "json-compare-2",
				Message: "pass",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			fileContents: []byte(`{
				"foo": "bar",
				"stuff": {
					"foo": "bar",
					"bar": true
				},
				"morestuff": [
					{
						"foo": {
							"bar": 123
						}
					}
				]
			}`),
		},
		{
			name: "comparison using path 2",
			analyzer: troubleshootv1beta2.JsonCompare{
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
				CollectorName: "json-compare-3",
				FileName:      "json-compare-3.json",
				Path:          "morestuff.[0].foo.bar",
				Value:         `123`,
			},
			expectResult: AnalyzeResult{
				IsPass:  true,
				IsWarn:  false,
				IsFail:  false,
				Title:   "json-compare-3",
				Message: "pass",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			fileContents: []byte(`{
				"foo": "bar",
				"stuff": {
					"foo": "bar",
					"bar": true
				},
				"morestuff": [
					{
						"foo": {
							"bar": 123
						}
					}
				]
			}`),
		},
		{
			name: "comparison using path 2, but warn on match",
			analyzer: troubleshootv1beta2.JsonCompare{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "pass",
							When:    "false",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							Message: "warn",
							When:    "true",
						},
					},
				},
				CollectorName: "json-compare-3-1",
				FileName:      "json-compare-3-1.json",
				Path:          "morestuff.[0].foo.bar",
				Value:         `123`,
			},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  true,
				IsFail:  false,
				Title:   "json-compare-3-1",
				Message: "warn",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			fileContents: []byte(`{
				"foo": "bar",
				"stuff": {
					"foo": "bar",
					"bar": true
				},
				"morestuff": [
					{
						"foo": {
							"bar": 123
						}
					}
				]
			}`),
		},
		{
			name: "basic comparison fail",
			analyzer: troubleshootv1beta2.JsonCompare{
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
				CollectorName: "json-compare-4",
				FileName:      "json-compare-4.json",
				Value: `{
					"foo": "bar",
					"stuff": {
						"foo": "bar",
						"bar": true
					},
					"morestuff": [
						{
							"foo": {
								"bar": 123
							}
						}
					]
				}`,
			},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  false,
				IsFail:  true,
				Title:   "json-compare-4",
				Message: "fail",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			fileContents: []byte(`{
				"foo": "bar",
				"stuff": {
					"foo": "bar",
					"bar": true
				},
				"otherstuff": [
					{
						"foo": {
							"bar": 123
						}
					}
				]
			}`),
		},
		{
			name: "comparison using path fail 1",
			analyzer: troubleshootv1beta2.JsonCompare{
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
				CollectorName: "json-compare-5",
				FileName:      "json-compare-5.json",
				Path:          "morestuff",
				Value: `[
					{
						"foo": {
							"bar": 321
						}
					}
				]`,
			},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  false,
				IsFail:  true,
				Title:   "json-compare-5",
				Message: "fail",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			fileContents: []byte(`{
				"foo": "bar",
				"stuff": {
					"foo": "bar",
					"bar": true
				},
				"morestuff": [
					{
						"foo": {
							"bar": 123
						}
					}
				]
			}`),
		},
		{
			name: "comparison using path, but pass when not matching",
			analyzer: troubleshootv1beta2.JsonCompare{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "pass",
							When:    "false",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "fail",
							When:    "true",
						},
					},
				},
				CollectorName: "json-compare-5-1",
				FileName:      "json-compare-5-1.json",
				Path:          "morestuff",
				Value: `[
					{
						"foo": {
							"bar": 321
						}
					}
				]`,
			},
			expectResult: AnalyzeResult{
				IsPass:  true,
				IsWarn:  false,
				IsFail:  false,
				Title:   "json-compare-5-1",
				Message: "pass",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			fileContents: []byte(`{
				"foo": "bar",
				"stuff": {
					"foo": "bar",
					"bar": true
				},
				"morestuff": [
					{
						"foo": {
							"bar": 123
						}
					}
				]
			}`),
		},
		{
			name: "basic comparison warn",
			analyzer: troubleshootv1beta2.JsonCompare{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "pass",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							Message: "warn",
						},
					},
				},
				CollectorName: "json-compare-6",
				FileName:      "json-compare-6.json",
				Value: `{
					"foo": "bar",
					"stuff": {
						"foo": "bar",
						"bar": true
					},
					"morestuff": [
						{
							"foo": {
								"bar": 123
							}
						}
					]
				}`,
			},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  true,
				IsFail:  false,
				Title:   "json-compare-6",
				Message: "warn",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			fileContents: []byte(`{
				"foo": "bar",
				"stuff": {
					"foo": "bar",
					"bar": true
				},
				"otherstuff": [
					{
						"foo": {
							"bar": 123
						}
					}
				]
			}`),
		},
		{
			name:    "invalid json error",
			isError: true,
			analyzer: troubleshootv1beta2.JsonCompare{
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
				CollectorName: "json-compare-7",
				FileName:      "json-compare-7.json",
				Path:          "morestuff",
				Value: `[
					{
						"foo": {
							"bar": 123
						}
					}
				]`,
			},
			fileContents: []byte(`{ "this: - is-invalid: json }`),
		},
		{
			name:    "no json error",
			isError: true,
			analyzer: troubleshootv1beta2.JsonCompare{
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
				CollectorName: "json-compare-8",
				FileName:      "json-compare-8.json",
				Path:          "morestuff",
				Value: `[
					{
						"foo": {
							"bar": 123
						}
					}
				]`,
			},
			fileContents: []byte(``),
		},
		{
			name: "jsonpath comparison",
			analyzer: troubleshootv1beta2.JsonCompare{
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
				CollectorName: "jsonpath-compare-1",
				FileName:      "jsonpath-compare-1.json",
				JsonPath:      "{$.morestuff[0]}",
				Value: `{
					"foo": {
						"bar": 123
					}
				}`,
			},
			expectResult: AnalyzeResult{
				IsPass:  true,
				IsWarn:  false,
				IsFail:  false,
				Title:   "jsonpath-compare-1",
				Message: "pass",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			fileContents: []byte(`{
				"foo": "bar",
				"stuff": {
					"foo": "bar",
					"bar": true
				},
				"morestuff": [
					{
						"foo": {
							"bar": 123
						}
					}
				]
			}`),
		},
		{
			name: "jsonpath comparison, but fail on match",
			analyzer: troubleshootv1beta2.JsonCompare{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "pass",
							When:    "false",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "fail",
							When:    "true",
						},
					},
				},
				CollectorName: "jsonpath-compare-1-1",
				FileName:      "jsonpath-compare-1-1.json",
				JsonPath:      "{$.morestuff[0].foo.bar}",
				Value:         `123`,
			},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  false,
				IsFail:  true,
				Title:   "jsonpath-compare-1-1",
				Message: "fail",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			fileContents: []byte(`{
				"foo": "bar",
				"stuff": {
					"foo": "bar",
					"bar": true
				},
				"morestuff": [
					{
						"foo": {
							"bar": 123
						}
					}
				]
			}`),
		},
		{
			name: "jsonpath comparison, multiple values",
			analyzer: troubleshootv1beta2.JsonCompare{
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
				CollectorName: "jsonpath-compare-2",
				FileName:      "jsonpath-compare-2.json",
				JsonPath:      "{$..bar}",
				Value:         `[true, 123]`,
			},
			expectResult: AnalyzeResult{
				IsPass:  true,
				IsWarn:  false,
				IsFail:  false,
				Title:   "jsonpath-compare-2",
				Message: "pass",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			fileContents: []byte(`{
				"foo": "bar",
				"stuff": {
					"foo": "bar",
					"bar": true
				},
				"morestuff": [
					{
						"foo": {
							"bar": 123
						}
					}
				]
			}`),
		},
		{
			name: "basic comparison with outcome message templated",
			analyzer: troubleshootv1beta2.JsonCompare{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "Status: {{ .morestuff.status }}, Info: {{ .morestuff.info }}",
							When:    "true",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "Status: {{ .morestuff.status }}, Info: {{ .morestuff.info }}",
							When:    "false",
						},
					},
				},
				CollectorName: "json-compare",
				FileName:      "json-compare.json",
				Path:          "morestuff.status",
				Value:         `"ready"`,
			},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  false,
				IsFail:  true,
				Title:   "json-compare",
				Message: "Status: notready, Info: morestuff is not ready",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			fileContents: []byte(`{
			    "stuff": {
			      "status": "ready",
			      "info": "this stuff is ready"
			    },
			    "morestuff": {
			      "status": "notready",
			      "info": "morestuff is not ready"
			    }
			}`),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			getCollectedFileContents := func(n string) ([]byte, error) {
				return test.fileContents, nil
			}

			actual, err := analyzeJsonCompare(&test.analyzer, getCollectedFileContents, "json-compare")
			if !test.isError {
				req.NoError(err)
				req.Equal(test.expectResult, *actual)
			} else {
				req.Error(err)
			}
		})
	}
}
