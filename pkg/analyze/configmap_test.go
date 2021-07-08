package analyzer

import (
	"testing"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_analyzeConfigMap(t *testing.T) {
	tests := []struct {
		name      string
		analyzer  *troubleshootv1beta2.AnalyzeConfigMap
		mockFiles map[string][]byte
		want      *AnalyzeResult
		wantErr   bool
	}{
		{
			name: "found",
			analyzer: &troubleshootv1beta2.AnalyzeConfigMap{
				Namespace:     "test-namespace",
				ConfigMapName: "test-configmap",
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
				"configmaps/test-namespace/test-configmap.json": mustJSONMarshalIndent(t, collect.ConfigMapOutput{
					Namespace:       "test-namespace",
					Name:            "test-configmap",
					ConfigMapExists: true,
				}),
			},
			want: &AnalyzeResult{
				IsPass:  true,
				Message: "Found",
				Title:   "ConfigMap test-configmap",
				IconKey: "kubernetes_analyze_secret",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/secret.svg?w=13&h=16",
			},
		},
		{
			name: "not found",
			analyzer: &troubleshootv1beta2.AnalyzeConfigMap{
				Namespace:     "test-namespace",
				ConfigMapName: "test-configmap",
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
				"configmaps/test-namespace/test-configmap.json": mustJSONMarshalIndent(t, collect.ConfigMapOutput{
					Namespace:       "test-namespace",
					Name:            "test-configmap",
					ConfigMapExists: false,
				}),
			},
			want: &AnalyzeResult{
				IsFail:  true,
				Message: "Not found",
				Title:   "ConfigMap test-configmap",
				IconKey: "kubernetes_analyze_secret",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/secret.svg?w=13&h=16",
			},
		},
		{
			name: "key found",
			analyzer: &troubleshootv1beta2.AnalyzeConfigMap{
				Namespace:     "test-namespace",
				ConfigMapName: "test-configmap",
				Key:           "test-key",
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
				"configmaps/test-namespace/test-configmap/test-key.json": mustJSONMarshalIndent(t, collect.ConfigMapOutput{
					Namespace:       "test-namespace",
					Name:            "test-configmap",
					Key:             "test-key",
					ConfigMapExists: true,
					KeyExists:       true,
				}),
			},
			want: &AnalyzeResult{
				IsPass:  true,
				Message: "Key found",
				Title:   "ConfigMap test-configmap",
				IconKey: "kubernetes_analyze_secret",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/secret.svg?w=13&h=16",
			},
		},
		{
			name: "key not found",
			analyzer: &troubleshootv1beta2.AnalyzeConfigMap{
				Namespace:     "test-namespace",
				ConfigMapName: "test-configmap",
				Key:           "test-key",
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
				"configmaps/test-namespace/test-configmap/test-key.json": mustJSONMarshalIndent(t, collect.ConfigMapOutput{
					Namespace:       "test-namespace",
					Name:            "test-configmap",
					Key:             "test-key",
					ConfigMapExists: true,
					KeyExists:       false,
				}),
			},
			want: &AnalyzeResult{
				IsFail:  true,
				Message: "Key not found",
				Title:   "ConfigMap test-configmap",
				IconKey: "kubernetes_analyze_secret",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/secret.svg?w=13&h=16",
			},
		},
		{
			name: "key not found configmap not found",
			analyzer: &troubleshootv1beta2.AnalyzeConfigMap{
				Namespace:     "test-namespace",
				ConfigMapName: "test-configmap",
				Key:           "test-key",
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
			got, err := analyzeConfigMap(tt.analyzer, getCollectedFileContents)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
