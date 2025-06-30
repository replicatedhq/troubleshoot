package collect

import (
	"context"
	"encoding/json"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestSecret(t *testing.T) {
	tests := []struct {
		name            string
		secretCollector *troubleshootv1beta2.Secret
		mockSecrets     []corev1.Secret
		want            CollectorResult
		wantErr         bool
	}{
		{
			name: "by name",
			secretCollector: &troubleshootv1beta2.Secret{
				Namespace: "test-namespace",
				Name:      "test-secret",
			},
			mockSecrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"test-key": []byte("test-value"),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-secret",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"test-key": []byte("test-value"),
					},
				},
			},
			want: CollectorResult{
				"secrets/test-namespace/test-secret.json": mustJSONMarshalIndent(t, SecretOutput{
					Namespace:    "test-namespace",
					Name:         "test-secret",
					SecretExists: true,
				}),
			},
		},
		{
			name: "by selector",
			secretCollector: &troubleshootv1beta2.Secret{
				Namespace: "test-namespace",
				Selector: []string{
					"app=my-app",
				},
			},
			mockSecrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "test-namespace",
						Labels:    map[string]string{"app": "my-app"},
					},
					Data: map[string][]byte{
						"test-key": []byte("test-value"),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-secret",
						Namespace: "test-namespace",
						Labels:    map[string]string{"app": "not-my-app"},
					},
					Data: map[string][]byte{
						"test-key": []byte("test-value"),
					},
				},
			},
			want: CollectorResult{
				"secrets/test-namespace/test-secret.json": mustJSONMarshalIndent(t, SecretOutput{
					Namespace:    "test-namespace",
					Name:         "test-secret",
					SecretExists: true,
				}),
			},
		},
		{
			name: "with key",
			secretCollector: &troubleshootv1beta2.Secret{
				Namespace: "test-namespace",
				Name:      "test-secret",
				Key:       "test-key",
			},
			mockSecrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"test-key":  []byte("test-value"),
						"other-key": []byte("other-value"),
					},
				},
			},
			want: CollectorResult{
				"secrets/test-namespace/test-secret/test-key.json": mustJSONMarshalIndent(t, SecretOutput{
					Namespace:    "test-namespace",
					Name:         "test-secret",
					Key:          "test-key",
					SecretExists: true,
					KeyExists:    true,
				}),
			},
		},
		{
			name: "with key and value",
			secretCollector: &troubleshootv1beta2.Secret{
				Namespace:    "test-namespace",
				Name:         "test-secret",
				Key:          "test-key",
				IncludeValue: true,
			},
			mockSecrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"test-key":  []byte("test-value"),
						"other-key": []byte("other-value"),
					},
				},
			},
			want: CollectorResult{
				"secrets/test-namespace/test-secret/test-key.json": mustJSONMarshalIndent(t, SecretOutput{
					Namespace:    "test-namespace",
					Name:         "test-secret",
					Key:          "test-key",
					SecretExists: true,
					KeyExists:    true,
					Value:        "test-value",
				}),
			},
		},
		{
			name: "not found",
			secretCollector: &troubleshootv1beta2.Secret{
				Namespace: "test-namespace",
				Name:      "test-secret",
			},
			mockSecrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-secret",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"test-key": []byte("test-value"),
					},
				},
			},
			want: CollectorResult{
				"secrets/test-namespace/test-secret.json": mustJSONMarshalIndent(t, SecretOutput{
					Namespace:    "test-namespace",
					Name:         "test-secret",
					SecretExists: false,
				}),
				"secrets-errors/test-namespace/test-secret.json": mustJSONMarshalIndent(t, []string{
					`secrets "test-secret" not found`,
				}),
			},
		},
		{
			name: "key not found",
			secretCollector: &troubleshootv1beta2.Secret{
				Namespace: "test-namespace",
				Name:      "test-secret",
				Key:       "test-key",
			},
			mockSecrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"other-key": []byte("other-value"),
					},
				},
			},
			want: CollectorResult{
				"secrets/test-namespace/test-secret/test-key.json": mustJSONMarshalIndent(t, SecretOutput{
					Namespace:    "test-namespace",
					Name:         "test-secret",
					Key:          "test-key",
					SecretExists: true,
					KeyExists:    false,
				}),
			},
		},
		{
			name: "with includeAllData",
			secretCollector: &troubleshootv1beta2.Secret{
				Namespace:      "test-namespace",
				Name:           "test-secret",
				IncludeAllData: true,
			},
			mockSecrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"database-password": []byte("secret123"),
						"api-key":           []byte("abc123xyz"),
						"jwt-secret":        []byte("mysupersecret"),
					},
				},
			},
			want: CollectorResult{
				"secrets/test-namespace/test-secret.json": mustJSONMarshalIndent(t, SecretOutput{
					Namespace:    "test-namespace",
					Name:         "test-secret",
					SecretExists: true,
					Data: map[string]string{
						"database-password": "secret123",
						"api-key":           "abc123xyz",
						"jwt-secret":        "mysupersecret",
					},
				}),
			},
		},
		{
			name: "with includeAllData and specific key",
			secretCollector: &troubleshootv1beta2.Secret{
				Namespace:      "test-namespace",
				Name:           "test-secret",
				Key:            "database-password",
				IncludeValue:   true,
				IncludeAllData: true,
			},
			mockSecrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"database-password": []byte("secret123"),
						"api-key":           []byte("abc123xyz"),
						"jwt-secret":        []byte("mysupersecret"),
					},
				},
			},
			want: CollectorResult{
				"secrets/test-namespace/test-secret.json": mustJSONMarshalIndent(t, SecretOutput{
					Namespace:    "test-namespace",
					Name:         "test-secret",
					Key:          "database-password",
					SecretExists: true,
					Data: map[string]string{
						"database-password": "secret123",
						"api-key":           "abc123xyz",
						"jwt-secret":        "mysupersecret",
					},
				}),
			},
		},
		{
			name: "with includeAllData secret not found",
			secretCollector: &troubleshootv1beta2.Secret{
				Namespace:      "test-namespace",
				Name:           "test-secret",
				IncludeAllData: true,
			},
			mockSecrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-secret",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"test-key": []byte("test-value"),
					},
				},
			},
			want: CollectorResult{
				"secrets/test-namespace/test-secret.json": mustJSONMarshalIndent(t, SecretOutput{
					Namespace:    "test-namespace",
					Name:         "test-secret",
					SecretExists: false,
				}),
				"secrets-errors/test-namespace/test-secret.json": mustJSONMarshalIndent(t, []string{
					`secrets "test-secret" not found`,
				}),
			},
		},
		{
			name: "with includeAllData by selector",
			secretCollector: &troubleshootv1beta2.Secret{
				Namespace: "test-namespace",
				Selector: []string{
					"app=my-app",
				},
				IncludeAllData: true,
			},
			mockSecrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "test-namespace",
						Labels:    map[string]string{"app": "my-app"},
					},
					Data: map[string][]byte{
						"database-password": []byte("secret123"),
						"api-key":           []byte("abc123xyz"),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-secret",
						Namespace: "test-namespace",
						Labels:    map[string]string{"app": "not-my-app"},
					},
					Data: map[string][]byte{
						"test-key": []byte("test-value"),
					},
				},
			},
			want: CollectorResult{
				"secrets/test-namespace/test-secret.json": mustJSONMarshalIndent(t, SecretOutput{
					Namespace:    "test-namespace",
					Name:         "test-secret",
					SecretExists: true,
					Data: map[string]string{
						"database-password": "secret123",
						"api-key":           "abc123xyz",
					},
				}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := testclient.NewSimpleClientset()
			for _, secret := range tt.mockSecrets {
				_, err := client.CoreV1().Secrets(secret.Namespace).Create(ctx, &secret, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			secretCollector := &CollectSecret{tt.secretCollector, "", "", nil, client, ctx, nil}
			got, err := secretCollector.Collect(nil)
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
