package collect

import (
	"context"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func Test_selectorToString(t *testing.T) {
	tests := []struct {
		name     string
		selector []string
		expect   string
	}{
		{
			name:     "app=api",
			selector: []string{"app=api"},
			expect:   "app-api",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := selectorToString(test.selector)
			assert.Equal(t, test.expect, actual)
		})
	}
}

func Test_DeterministicIDForCollector(t *testing.T) {
	tests := []struct {
		name      string
		collector *troubleshootv1beta2.Collect
		expect    string
	}{
		{
			name: "cluster-info",
			collector: &troubleshootv1beta2.Collect{
				ClusterInfo: &troubleshootv1beta2.ClusterInfo{},
			},
			expect: "cluster-info",
		},
		{
			name: "cluster-resources",
			collector: &troubleshootv1beta2.Collect{
				ClusterResources: &troubleshootv1beta2.ClusterResources{},
			},
			expect: "cluster-resources",
		},
		{
			name: "secret",
			collector: &troubleshootv1beta2.Collect{
				Secret: &troubleshootv1beta2.Secret{
					Namespace: "top-secret",
					Name:      "secret-agent-woman",
				},
			},
			expect: "secret-top-secret-secret-agent-woman",
		},
		{
			name: "secret selector",
			collector: &troubleshootv1beta2.Collect{
				Secret: &troubleshootv1beta2.Secret{
					Namespace: "top-secret",
					Selector:  []string{"this=is", "rather=long", "for=testing", "more=words", "too=many", "abcdef!=123456"},
				},
			},
			expect: "secret-top-secret-this-is-rather-long-for-testing-more-words-to",
		},
		{
			name: "configmap",
			collector: &troubleshootv1beta2.Collect{
				ConfigMap: &troubleshootv1beta2.ConfigMap{
					Namespace: "top-secret",
					Name:      "secret-agent-woman",
				},
			},
			expect: "configmap-top-secret-secret-agent-woman",
		},
		{
			name: "configmap selector",
			collector: &troubleshootv1beta2.Collect{
				ConfigMap: &troubleshootv1beta2.ConfigMap{
					Namespace: "top-secret",
					Selector:  []string{"this=is", "rather=long", "for=testing", "more=words", "too=many", "abcdef!=123456"},
				},
			},
			expect: "configmap-top-secret-this-is-rather-long-for-testing-more-words",
		},
		{
			name: "logs",
			collector: &troubleshootv1beta2.Collect{
				Logs: &troubleshootv1beta2.Logs{
					Namespace: "top-secret",
					Selector:  []string{"this=is", "rather=long", "for=testing", "more=words", "too=many", "abcdef!=123456"},
				},
			},
			expect: "logs-top-secret-this-is-rather-long-for-testing-more-words-too-",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := DeterministicIDForCollector(test.collector)
			assert.Equal(t, test.expect, actual)
		})
	}
}

func Test_createTLSConfig(t *testing.T) {
	k8sClient := testclient.NewSimpleClientset()

	tests := []struct {
		name       string
		tlsParams  v1beta2.TLSParams
		caCertOnly bool
		hasError   bool
	}{
		{
			name: "complete tls params creates config successfully",
			tlsParams: v1beta2.TLSParams{
				CACert:     getTestFixture(t, "db/ca.pem"),
				ClientCert: getTestFixture(t, "db/client.pem"),
				ClientKey:  getTestFixture(t, "db/client-key.pem"),
			},
		},
		{
			name: "complete tls params in secret creates config successfully",
			tlsParams: v1beta2.TLSParams{
				Secret: createTLSSecret(t, k8sClient, map[string]string{
					"cacert":     getTestFixture(t, "db/ca.pem"),
					"clientCert": getTestFixture(t, "db/client.pem"),
					"clientKey":  getTestFixture(t, "db/client-key.pem"),
				}),
			},
		},
		{
			name: "tls params with skip verify creates config successfully",
			tlsParams: v1beta2.TLSParams{
				SkipVerify: true,
			},
		},
		{
			name: "tls params with CA cert only in secret creates config successfully",
			tlsParams: v1beta2.TLSParams{
				Secret: createTLSSecret(t, k8sClient, map[string]string{
					"cacert": getTestFixture(t, "db/ca.pem"),
				}),
			},
			caCertOnly: true,
		},
		{
			name: "tls params with CA cert only creates config successfully",
			tlsParams: v1beta2.TLSParams{
				CACert: getTestFixture(t, "db/ca.pem"),
			},
			caCertOnly: true,
		},
		{
			name:     "empty TLS parameters fails to create config with error",
			hasError: true,
		},
		{
			name: "missing CA cert fails to create config with error",
			tlsParams: v1beta2.TLSParams{
				ClientCert: getTestFixture(t, "db/client.pem"),
				ClientKey:  getTestFixture(t, "db/client-key.pem"),
			},
			hasError: true,
		},
		{
			name: "missing client cert fails to create config with error",
			tlsParams: v1beta2.TLSParams{
				CACert:    getTestFixture(t, "db/ca.pem"),
				ClientKey: getTestFixture(t, "db/client-key.pem"),
			},
			hasError: true,
		},
		{
			name: "missing client key fails to create config with error",
			tlsParams: v1beta2.TLSParams{
				CACert:     getTestFixture(t, "db/ca.pem"),
				ClientCert: getTestFixture(t, "db/client.pem"),
			},
			hasError: true,
		},
		{
			name: "missing CA cert in secret fails to create config with error",
			tlsParams: v1beta2.TLSParams{
				Secret: createTLSSecret(t, k8sClient, map[string]string{
					"clientCert": getTestFixture(t, "db/client.pem"),
					"clientKey":  getTestFixture(t, "db/client-key.pem"),
				}),
			},
			hasError: true,
		},
		{
			name: "missing client cert in secret fails to create config with error",
			tlsParams: v1beta2.TLSParams{
				Secret: createTLSSecret(t, k8sClient, map[string]string{
					"cacert":    getTestFixture(t, "db/ca.pem"),
					"clientKey": getTestFixture(t, "db/client-key.pem"),
				}),
			},
			hasError: true,
		},
		{
			name: "missing client key in secret fails to create config with error",
			tlsParams: v1beta2.TLSParams{
				Secret: createTLSSecret(t, k8sClient, map[string]string{
					"cacert":     getTestFixture(t, "db/ca.pem"),
					"clientCert": getTestFixture(t, "db/client.pem"),
				}),
			},
			hasError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tlsCfg, err := createTLSConfig(context.Background(), k8sClient, &tt.tlsParams)
			assert.Equalf(t, tt.hasError, err != nil, "createTLSConfig() error = %v, wantErr %v", err, tt.hasError)

			if err == nil {
				require.NotNil(t, tlsCfg)

				if tt.tlsParams.SkipVerify {
					assert.True(t, tlsCfg.InsecureSkipVerify)
					assert.Nil(t, tlsCfg.RootCAs)
					assert.Nil(t, tlsCfg.Certificates)
				} else {
					// TLS parameter objects are opaque. Just check if they were created.
					// There is no trivial way to inspect their metadata. Trust me :)
					assert.NotNil(t, tlsCfg.RootCAs)
					assert.Equal(t, tt.caCertOnly, tlsCfg.Certificates == nil)
					assert.False(t, tlsCfg.InsecureSkipVerify)
				}
			} else {
				t.Log(err)
				assert.Nil(t, tlsCfg)
			}
		})
	}
}

func randStringRunes(n int) string {
	runes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")
	b := make([]rune, n)
	for i := range b {
		b[i] = runes[rand.Intn(len(runes))]
	}
	return string(b)
}

// createTLSSecret create a secret in a fake client
func createTLSSecret(t *testing.T, client kubernetes.Interface, secretData map[string]string) *v1beta2.TLSSecret {
	t.Helper()

	// Generate unique names cause we reuse the same client
	secretName := "secret-name-" + randStringRunes(20)
	namespace := "namespace-" + randStringRunes(20)

	_, err := client.CoreV1().Secrets(namespace).Create(
		context.Background(),
		&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: secretName,
			},
			StringData: secretData,
		},
		metav1.CreateOptions{},
	)
	require.NoError(t, err)

	return &v1beta2.TLSSecret{
		Namespace: namespace,
		Name:      secretName,
	}
}

func getTestFixture(t *testing.T, path string) string {
	t.Helper()
	p := filepath.Join("../../testdata", path)
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	return string(b)
}
