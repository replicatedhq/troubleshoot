package collect

import (
	"context"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestConfigMap(t *testing.T) {
	type args struct {
		ctx    context.Context
		client kubernetes.Interface
	}
	tests := []struct {
		name               string
		configMapCollector *troubleshootv1beta2.ConfigMap
		mockConfigMaps     []corev1.ConfigMap
		want               map[string][]byte
		wantErr            bool
	}{
		{
			name: "by name",
			configMapCollector: &troubleshootv1beta2.ConfigMap{
				Namespace: "test-namespace",
				Name:      "test-configmap",
			},
			mockConfigMaps: []corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-configmap",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"test-key": "test-value",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-configmap",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"test-key": "test-value",
					},
				},
			},
			want: map[string][]byte{
				"configmaps/test-namespace/test-configmap.json": mustJSONMarshalIndent(t, ConfigMapOutput{
					Namespace:       "test-namespace",
					Name:            "test-configmap",
					ConfigMapExists: true,
				}),
			},
		},
		{
			name: "by selector",
			configMapCollector: &troubleshootv1beta2.ConfigMap{
				Namespace: "test-namespace",
				Selector: []string{
					"app=my-app",
				},
			},
			mockConfigMaps: []corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-configmap",
						Namespace: "test-namespace",
						Labels:    map[string]string{"app": "my-app"},
					},
					Data: map[string]string{
						"test-key": "test-value",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-configmap",
						Namespace: "test-namespace",
						Labels:    map[string]string{"app": "not-my-app"},
					},
					Data: map[string]string{
						"test-key": "test-value",
					},
				},
			},
			want: map[string][]byte{
				"configmaps/test-namespace/test-configmap.json": mustJSONMarshalIndent(t, ConfigMapOutput{
					Namespace:       "test-namespace",
					Name:            "test-configmap",
					ConfigMapExists: true,
				}),
			},
		},
		{
			name: "with key",
			configMapCollector: &troubleshootv1beta2.ConfigMap{
				Namespace: "test-namespace",
				Name:      "test-configmap",
				Key:       "test-key",
			},
			mockConfigMaps: []corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-configmap",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"test-key":  "test-value",
						"other-key": "other-value",
					},
				},
			},
			want: map[string][]byte{
				"configmaps/test-namespace/test-configmap/test-key.json": mustJSONMarshalIndent(t, ConfigMapOutput{
					Namespace:       "test-namespace",
					Name:            "test-configmap",
					Key:             "test-key",
					ConfigMapExists: true,
					KeyExists:       true,
				}),
			},
		},
		{
			name: "with key and value",
			configMapCollector: &troubleshootv1beta2.ConfigMap{
				Namespace:    "test-namespace",
				Name:         "test-configmap",
				Key:          "test-key",
				IncludeValue: true,
			},
			mockConfigMaps: []corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-configmap",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"test-key":  "test-value",
						"other-key": "other-value",
					},
				},
			},
			want: map[string][]byte{
				"configmaps/test-namespace/test-configmap/test-key.json": mustJSONMarshalIndent(t, ConfigMapOutput{
					Namespace:       "test-namespace",
					Name:            "test-configmap",
					Key:             "test-key",
					ConfigMapExists: true,
					KeyExists:       true,
					Value:           "test-value",
				}),
			},
		},
		{
			name: "not found",
			configMapCollector: &troubleshootv1beta2.ConfigMap{
				Namespace: "test-namespace",
				Name:      "test-configmap",
			},
			mockConfigMaps: []corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-configmap",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"test-key": "test-value",
					},
				},
			},
			want: map[string][]byte{
				"configmaps/test-namespace/test-configmap.json": mustJSONMarshalIndent(t, ConfigMapOutput{
					Namespace:       "test-namespace",
					Name:            "test-configmap",
					ConfigMapExists: false,
				}),
				"configmaps-errors/test-namespace/test-configmap.json": mustJSONMarshalIndent(t, []string{
					`configmaps "test-configmap" not found`,
				}),
			},
		},
		{
			name: "key not found",
			configMapCollector: &troubleshootv1beta2.ConfigMap{
				Namespace: "test-namespace",
				Name:      "test-configmap",
				Key:       "test-key",
			},
			mockConfigMaps: []corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-configmap",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"other-key": "other-value",
					},
				},
			},
			want: map[string][]byte{
				"configmaps/test-namespace/test-configmap/test-key.json": mustJSONMarshalIndent(t, ConfigMapOutput{
					Namespace:       "test-namespace",
					Name:            "test-configmap",
					Key:             "test-key",
					ConfigMapExists: true,
					KeyExists:       false,
				}),
			},
		},
		{
			name: "collectAll",
			configMapCollector: &troubleshootv1beta2.ConfigMap{
				Namespace:      "test-namespace",
				Name:           "test-configmap",
				IncludeAllData: true,
			},
			mockConfigMaps: []corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-configmap",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"test-key1": "test-value1",
						"test-key2": "test-value2",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-configmap",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"test-key": "test-value",
					},
				},
			},
			want: map[string][]byte{
				"configmaps/test-namespace/test-configmap.json": mustJSONMarshalIndent(t, ConfigMapOutput{
					Namespace:       "test-namespace",
					Name:            "test-configmap",
					ConfigMapExists: true,
					Data: map[string]string{
						"test-key1": "test-value1",
						"test-key2": "test-value2",
					},
				}),
			},
		},
		{
			name: "collectAll no data",
			configMapCollector: &troubleshootv1beta2.ConfigMap{
				Namespace:      "test-namespace",
				Name:           "test-configmap",
				IncludeAllData: true,
			},
			mockConfigMaps: []corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-configmap",
						Namespace: "test-namespace",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-configmap",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"test-key": "test-value",
					},
				},
			},
			want: map[string][]byte{
				"configmaps/test-namespace/test-configmap.json": mustJSONMarshalIndent(t, ConfigMapOutput{
					Namespace:       "test-namespace",
					Name:            "test-configmap",
					ConfigMapExists: true,
				}),
			},
		},
		{
			name: "collectAll with slectKey",
			configMapCollector: &troubleshootv1beta2.ConfigMap{
				Namespace:      "test-namespace",
				Name:           "test-configmap",
				Key:            "test-key1",
				IncludeAllData: true,
			},
			mockConfigMaps: []corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-configmap",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"test-key1": "test-value1",
						"test-key2": "test-value2",
					},
				},
			},
			want: map[string][]byte{
				"configmaps/test-namespace/test-configmap/test-key1.json": mustJSONMarshalIndent(t, ConfigMapOutput{
					Namespace:       "test-namespace",
					Name:            "test-configmap",
					ConfigMapExists: true,
					Key:             "test-key1",
					Data: map[string]string{
						"test-key1": "test-value1",
						"test-key2": "test-value2",
					},
					KeyExists: true,
				}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := testclient.NewSimpleClientset()
			for _, configMap := range tt.mockConfigMaps {
				_, err := client.CoreV1().ConfigMaps(configMap.Namespace).Create(ctx, &configMap, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			got, err := ConfigMap(ctx, client, tt.configMapCollector)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
