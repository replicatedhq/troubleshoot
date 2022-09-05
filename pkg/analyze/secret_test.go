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

func Test_analyzeSecret(t *testing.T) {
	tests := []struct {
		name      string
		analyzer  *troubleshootv1beta2.AnalyzeSecret
		mockFiles map[string][]byte
		want      *AnalyzeResult
		wantErr   bool
	}{
		{
			name: "found",
			analyzer: &troubleshootv1beta2.AnalyzeSecret{
				Namespace:  "test-namespace",
				SecretName: "test-secret",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "Not found",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "Found",
						},
					},
				},
			},
			mockFiles: map[string][]byte{
				"secrets/test-namespace/test-secret.json": mustJSONMarshalIndent(t, collect.SecretOutput{
					Namespace:    "test-namespace",
					Name:         "test-secret",
					SecretExists: true,
				}),
			},
			want: &AnalyzeResult{
				IsPass:  true,
				Message: "Found",
				Title:   "Secret test-secret",
				IconKey: "kubernetes_analyze_secret",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/secret.svg?w=13&h=16",
			},
		},
		{
			name: "not found",
			analyzer: &troubleshootv1beta2.AnalyzeSecret{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "test secret analyzer",
				},
				Namespace:  "test-namespace",
				SecretName: "test-secret",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "Not found",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "Found",
						},
					},
				},
			},
			mockFiles: map[string][]byte{
				"secrets/test-namespace/test-secret.json": mustJSONMarshalIndent(t, collect.SecretOutput{
					Namespace:    "test-namespace",
					Name:         "test-secret",
					SecretExists: false,
				}),
			},
			want: &AnalyzeResult{
				IsFail:  true,
				Message: "Not found",
				Title:   "test secret analyzer",
				IconKey: "kubernetes_analyze_secret",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/secret.svg?w=13&h=16",
			},
		},
		{
			name: "key found",
			analyzer: &troubleshootv1beta2.AnalyzeSecret{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "test secret analyzer",
				},
				Namespace:  "test-namespace",
				SecretName: "test-secret",
				Key:        "test-key",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "Key not found",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "Key found",
						},
					},
				},
			},
			mockFiles: map[string][]byte{
				"secrets/test-namespace/test-secret/test-key.json": mustJSONMarshalIndent(t, collect.SecretOutput{
					Namespace:    "test-namespace",
					Name:         "test-secret",
					Key:          "test-key",
					SecretExists: true,
					KeyExists:    true,
				}),
			},
			want: &AnalyzeResult{
				IsPass:  true,
				Message: "Key found",
				Title:   "test secret analyzer",
				IconKey: "kubernetes_analyze_secret",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/secret.svg?w=13&h=16",
			},
		},
		{
			name: "key not found",
			analyzer: &troubleshootv1beta2.AnalyzeSecret{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "test secret analyzer",
				},
				Namespace:  "test-namespace",
				SecretName: "test-secret",
				Key:        "test-key",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "Key not found",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "Key found",
						},
					},
				},
			},
			mockFiles: map[string][]byte{
				"secrets/test-namespace/test-secret/test-key.json": mustJSONMarshalIndent(t, collect.SecretOutput{
					Namespace:    "test-namespace",
					Name:         "test-secret",
					Key:          "test-key",
					SecretExists: true,
					KeyExists:    false,
				}),
			},
			want: &AnalyzeResult{
				IsFail:  true,
				Message: "Key not found",
				Title:   "test secret analyzer",
				IconKey: "kubernetes_analyze_secret",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/secret.svg?w=13&h=16",
			},
		},
		{
			name: "key not found secret not found",
			analyzer: &troubleshootv1beta2.AnalyzeSecret{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "test secret analyzer",
				},
				Namespace:  "test-namespace",
				SecretName: "test-secret",
				Key:        "test-key",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "Key not found",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "Key found",
						},
					},
				},
			},
			wantErr: true, // TODO: should this be a not found error? This will not work with selectors.
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getCollectedFileContents := func(fileName string) ([]byte, error) {
				contents, ok := tt.mockFiles[fileName]
				if !ok {
					return nil, errors.Errorf("file %s was not collected", fileName)
				}
				return contents, nil
			}
			got, err := analyzeSecret(tt.analyzer, getCollectedFileContents)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func mustJSONMarshalIndent(t *testing.T, v interface{}) []byte {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return b
}
