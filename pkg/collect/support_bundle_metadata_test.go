package collect

import (
	"context"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestCollectSupportBundleMetadata(t *testing.T) {
	tests := []struct {
		name        string
		collector   *troubleshootv1beta2.SupportBundleMetadata
		mockSecrets []corev1.Secret
		want        CollectorResult
		wantErr     bool
	}{
		{
			name: "reads all data fields from secret",
			collector: &troubleshootv1beta2.SupportBundleMetadata{
				Namespace: "test-ns",
			},
			mockSecrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "replicated-support-metadata",
						Namespace: "test-ns",
					},
					Data: map[string][]byte{
						"mykey":      []byte("myvalue"),
						"myversion":  []byte("1.0.0-example"),
						"numCrashes": []byte("57"),
					},
				},
			},
			want: CollectorResult{
				"metadata/cluster.json": mustJSONMarshalIndent(t, map[string]string{
					"mykey":      "myvalue",
					"myversion":  "1.0.0-example",
					"numCrashes": "57",
				}),
			},
		},
		{
			name: "empty data map",
			collector: &troubleshootv1beta2.SupportBundleMetadata{
				Namespace: "test-ns",
			},
			mockSecrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "replicated-support-metadata",
						Namespace: "test-ns",
					},
					Data: map[string][]byte{},
				},
			},
			want: CollectorResult{
				"metadata/cluster.json": mustJSONMarshalIndent(t, map[string]string{}),
			},
		},
		{
			name: "secret not found returns error",
			collector: &troubleshootv1beta2.SupportBundleMetadata{
				Namespace: "test-ns",
			},
			mockSecrets: []corev1.Secret{},
			wantErr:     true,
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
			c := &CollectSupportBundleMetadata{tt.collector, "", "", nil, client, ctx, nil}
			got, err := c.Collect(nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
