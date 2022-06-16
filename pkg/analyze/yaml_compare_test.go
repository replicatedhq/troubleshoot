package analyzer

import (
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/require"
)

func Test_yamlCompare(t *testing.T) {
	tests := []struct {
		name         string
		isError      bool
		analyzer     troubleshootv1beta2.YamlCompare
		expectResult AnalyzeResult
		fileContents []byte
	}{
		{
			name: "basic comparison",
			analyzer: troubleshootv1beta2.YamlCompare{
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
				CollectorName: "yaml-compare-1",
				FileName:      "yaml-compare-1.yaml",
				Value: `foo: bar
stuff:
  foo: bar
  bar: foo
morestuff:
- foo:
    bar: baz`,
			},
			expectResult: AnalyzeResult{
				IsPass:  true,
				IsWarn:  false,
				IsFail:  false,
				Title:   "yaml-compare-1",
				Message: "pass",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			fileContents: []byte(`foo: bar
stuff:
  foo: bar
  bar: foo
morestuff:
- foo:
    bar: baz`),
		},
		{
			name: "basic comparison, but fail on match",
			analyzer: troubleshootv1beta2.YamlCompare{
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
				CollectorName: "yaml-compare-1-1",
				FileName:      "yaml-compare-1-1.yaml",
				Value: `foo: bar
stuff:
  foo: bar
  bar: foo
morestuff:
- foo:
    bar: baz`,
			},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  false,
				IsFail:  true,
				Title:   "yaml-compare-1-1",
				Message: "fail",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			fileContents: []byte(`foo: bar
stuff:
  foo: bar
  bar: foo
morestuff:
- foo:
    bar: baz`),
		},
		{
			name: "comparison using path 1",
			analyzer: troubleshootv1beta2.YamlCompare{
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
				CollectorName: "yaml-compare-2",
				FileName:      "yaml-compare-2.yaml",
				Path:          "morestuff",
				Value: `- foo:
    bar: baz`,
			},
			expectResult: AnalyzeResult{
				IsPass:  true,
				IsWarn:  false,
				IsFail:  false,
				Title:   "yaml-compare-2",
				Message: "pass",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			fileContents: []byte(`foo: bar
stuff:
  foo: bar
  bar: foo
morestuff:
- foo:
    bar: baz`),
		},
		{
			name: "comparison using path, but warn when matching",
			analyzer: troubleshootv1beta2.YamlCompare{
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
				CollectorName: "yaml-compare-2-1",
				FileName:      "yaml-compare-2-1.yaml",
				Path:          "morestuff",
				Value: `- foo:
    bar: baz`,
			},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  true,
				IsFail:  false,
				Title:   "yaml-compare-2-1",
				Message: "warn",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			fileContents: []byte(`foo: bar
stuff:
  foo: bar
  bar: foo
morestuff:
- foo:
    bar: baz`),
		},
		{
			name: "comparison using path 2",
			analyzer: troubleshootv1beta2.YamlCompare{
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
				CollectorName: "yaml-compare-3",
				FileName:      "yaml-compare-3.yaml",
				Path:          "morestuff.[0].foo",
				Value:         `bar: baz`,
			},
			expectResult: AnalyzeResult{
				IsPass:  true,
				IsWarn:  false,
				IsFail:  false,
				Title:   "yaml-compare-3",
				Message: "pass",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			fileContents: []byte(`foo: bar
stuff:
  foo: bar
  bar: foo
morestuff:
- foo:
    bar: baz`),
		},
		{
			name: "basic comparison fail",
			analyzer: troubleshootv1beta2.YamlCompare{
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
				CollectorName: "yaml-compare-4",
				FileName:      "yaml-compare-4.yaml",
				Value: `foo: bar
stuff:
  foo: bar
  bar: foo
morestuff:
- foo:
    bar: baz`,
			},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  false,
				IsFail:  true,
				Title:   "yaml-compare-4",
				Message: "fail",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			fileContents: []byte(`foo: bar
stuff:
  foo: bar
  bar: foo
otherstuff:
- foo:
    bar: baz`),
		},
		{
			name: "basic comparison pass when not matching",
			analyzer: troubleshootv1beta2.YamlCompare{
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
				CollectorName: "yaml-compare-4-1",
				FileName:      "yaml-compare-4-1.yaml",
				Value: `foo: bar
stuff:
  foo: bar
  bar: foo
morestuff:
- foo:
    bar: baz`,
			},
			expectResult: AnalyzeResult{
				IsPass:  true,
				IsWarn:  false,
				IsFail:  false,
				Title:   "yaml-compare-4-1",
				Message: "pass",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			fileContents: []byte(`foo: bar
stuff:
  foo: bar
  bar: foo
otherstuff:
- foo:
    bar: baz`),
		},
		{
			name: "comparison using path fail 1",
			analyzer: troubleshootv1beta2.YamlCompare{
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
				CollectorName: "yaml-compare-5",
				FileName:      "yaml-compare-5.yaml",
				Path:          "morestuff",
				Value: `- bar:
    foo: baz`,
			},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  false,
				IsFail:  true,
				Title:   "yaml-compare-5",
				Message: "fail",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			fileContents: []byte(`foo: bar
stuff:
  foo: bar
  bar: foo
morestuff:
- foo:
    bar: baz`),
		},
		{
			name: "basic comparison warn",
			analyzer: troubleshootv1beta2.YamlCompare{
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
				CollectorName: "yaml-compare-6",
				FileName:      "yaml-compare-6.yaml",
				Value: `foo: bar
stuff:
  foo: bar
  bar: foo
morestuff:
- foo:
    bar: baz`,
			},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  true,
				IsFail:  false,
				Title:   "yaml-compare-6",
				Message: "warn",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
			fileContents: []byte(`foo: bar
stuff:
  foo: bar
  bar: foo
otherstuff:
- foo:
    bar: baz`),
		},
		{
			name:    "invalid yaml error",
			isError: true,
			analyzer: troubleshootv1beta2.YamlCompare{
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
				CollectorName: "yaml-compare-7",
				FileName:      "yaml-compare-7.yaml",
				Path:          "morestuff",
				Value: `- foo:
    bar: baz`,
			},
			fileContents: []byte(`{ "this: - is-invalid: yaml }`),
		},
		{
			name:    "no yaml error",
			isError: true,
			analyzer: troubleshootv1beta2.YamlCompare{
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
				CollectorName: "yaml-compare-8",
				FileName:      "yaml-compare-8.yaml",
				Path:          "morestuff",
				Value: `- foo:
    bar: baz`,
			},
			fileContents: []byte(``),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			getCollectedFileContents := func(n string) ([]byte, error) {
				return test.fileContents, nil
			}

			actual, err := analyzeYamlCompare(&test.analyzer, getCollectedFileContents)
			if !test.isError {
				req.NoError(err)
				req.Equal(test.expectResult, *actual)
			} else {
				req.Error(err)
			}
		})
	}
}
