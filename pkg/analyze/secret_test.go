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
			name: "not found with no fail outcome leaves the message empty instead of fabricating one",
			analyzer: &troubleshootv1beta2.AnalyzeSecret{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "Optional Secret",
				},
				Namespace:  "default",
				SecretName: "does-not-exist",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "secret found",
						},
					},
				},
			},
			mockFiles: map[string][]byte{
				"secrets/default/does-not-exist.json": mustJSONMarshalIndent(t, collect.SecretOutput{
					Namespace:    "default",
					Name:         "does-not-exist",
					SecretExists: false,
				}),
			},
			want: &AnalyzeResult{
				IsFail:  true,
				Message: "",
				Title:   "Optional Secret",
				IconKey: "kubernetes_analyze_secret",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/secret.svg?w=13&h=16",
			},
		},
		{
			name: "key not found with no fail outcome leaves the message empty instead of fabricating one",
			analyzer: &troubleshootv1beta2.AnalyzeSecret{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "Optional Secret Key",
				},
				Namespace:  "test-namespace",
				SecretName: "test-secret",
				Key:        "missing-key",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "key found",
						},
					},
				},
			},
			mockFiles: map[string][]byte{
				"secrets/test-namespace/test-secret/missing-key.json": mustJSONMarshalIndent(t, collect.SecretOutput{
					Namespace:    "test-namespace",
					Name:         "test-secret",
					Key:          "missing-key",
					SecretExists: true,
					KeyExists:    false,
				}),
			},
			want: &AnalyzeResult{
				IsFail:  true,
				Message: "",
				Title:   "Optional Secret Key",
				IconKey: "kubernetes_analyze_secret",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/secret.svg?w=13&h=16",
			},
		},
		{
			name: "found with only fail outcome configured leaves the message empty, not the fail outcome's message",
			analyzer: &troubleshootv1beta2.AnalyzeSecret{
				Namespace:  "test-namespace",
				SecretName: "test-secret",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "Not found",
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
				Message: "",
				Title:   "Secret test-secret",
				IconKey: "kubernetes_analyze_secret",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/secret.svg?w=13&h=16",
			},
		},
		{
			name: "spec with neither fail nor pass outcome returns an error so the framework surfaces the misconfiguration",
			analyzer: &troubleshootv1beta2.AnalyzeSecret{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "Misconfigured",
				},
				Namespace:  "default",
				SecretName: "does-not-exist",
				Outcomes:   []*troubleshootv1beta2.Outcome{},
			},
			mockFiles: map[string][]byte{
				"secrets/default/does-not-exist.json": mustJSONMarshalIndent(t, collect.SecretOutput{
					Namespace:    "default",
					Name:         "does-not-exist",
					SecretExists: false,
				}),
			},
			wantErr: true,
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
		{
			name: "combined fail and pass in a single outcome, secret found uses the pass outcome",
			analyzer: &troubleshootv1beta2.AnalyzeSecret{
				Namespace:  "test-namespace",
				SecretName: "test-secret",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "Not found",
						},
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "Found",
							URI:     "https://example.com/found",
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
				URI:     "https://example.com/found",
				Title:   "Secret test-secret",
				IconKey: "kubernetes_analyze_secret",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/secret.svg?w=13&h=16",
			},
		},
		{
			name: "combined fail and pass in a single outcome, secret not found uses the fail outcome",
			analyzer: &troubleshootv1beta2.AnalyzeSecret{
				Namespace:  "test-namespace",
				SecretName: "test-secret",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "Not found",
						},
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
				Title:   "Secret test-secret",
				IconKey: "kubernetes_analyze_secret",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/secret.svg?w=13&h=16",
			},
		},
		{
			name: "secret found with URI-only pass outcome preserves the empty message and URI",
			analyzer: &troubleshootv1beta2.AnalyzeSecret{
				Namespace:  "test-namespace",
				SecretName: "test-secret",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							URI: "https://example.com/pass",
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
				Message: "",
				URI:     "https://example.com/pass",
				Title:   "Secret test-secret",
				IconKey: "kubernetes_analyze_secret",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/secret.svg?w=13&h=16",
			},
		},
		{
			name: "secret not found with URI-only fail outcome preserves the empty message and URI",
			analyzer: &troubleshootv1beta2.AnalyzeSecret{
				Namespace:  "test-namespace",
				SecretName: "test-secret",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							URI: "https://example.com/fail",
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
				Message: "",
				URI:     "https://example.com/fail",
				Title:   "Secret test-secret",
				IconKey: "kubernetes_analyze_secret",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/secret.svg?w=13&h=16",
			},
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
			a := AnalyzeSecret{
				analyzer: tt.analyzer,
			}
			got, err := a.analyzeSecret(tt.analyzer, getCollectedFileContents)
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
