package collect

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	v1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func Test_extractServerVersion(t *testing.T) {
	tests := []struct {
		name string
		info string
		want string
	}{
		{
			name: "extracts version successfully",
			info: `
			# Server
			redis_version:7.0.5
			redis_git_sha1:00000000
			redis_git_dirty:0
			redis_build_id:eb3578384289228a
			`,
			want: "7.0.5",
		},
		{
			name: "extracts version but fails",
			info: "",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractServerVersion(tt.info)
			assert.Equalf(t, tt.want, got, "extractServerVersion() = %v, want %v", got, tt.want)
		})
	}
}

func TestCollectRedis_createPlainTextClient(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		hasError bool
	}{
		{
			name: "valid uri creates redis client successfully",
			uri:  "redis://localhost:6379",
		},
		{
			name:     "empty uri fails to create client with error",
			uri:      "",
			hasError: true,
		},
		{
			name:     "invalid redis protocol fails to create client with error",
			uri:      "http://localhost:6379",
			hasError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CollectRedis{
				Collector: &v1beta2.Database{
					URI: tt.uri,
				},
			}

			client, err := c.createClient()
			assert.Equal(t, err != nil, tt.hasError)
			if err == nil {
				require.NotNil(t, client)
				assert.Equal(t, client.Options().Addr, "localhost:6379")
			} else {
				t.Log(err)
				assert.Nil(t, client)
			}
		})
	}
}

func TestCollectRedis_createMTLSClient(t *testing.T) {
	tests := []struct {
		name      string
		uri       string
		tlsParams v1beta2.TLSParams
		hasError  bool
	}{
		{
			name: "valid uri and tls params creates redis client successfully",
			uri:  "redis://localhost:6379",
			tlsParams: v1beta2.TLSParams{
				CACert:     getTestFixture(t, "db/ca.pem"),
				ClientCert: getTestFixture(t, "db/client.pem"),
				ClientKey:  getTestFixture(t, "db/client-key.pem"),
			},
		},
		{
			name: "valid uri and tls params secret creates redis client successfully",
			uri:  "redis://localhost:6379",
			tlsParams: v1beta2.TLSParams{
				Secret: &v1beta2.TLSSecret{
					Name:      "test-secret",
					Namespace: "default",
				},
			},
		},
		{
			name:     "empty TLS parameters fails to create client with error",
			uri:      "redis://localhost:6379",
			hasError: true,
		},
		{
			name:     "empty TLS parameters fails to create client with error",
			uri:      "redis://localhost:6379",
			hasError: true,
		},
		{
			name: "missing CA cert fails to create client with error",
			uri:  "redis://localhost:6379",
			tlsParams: v1beta2.TLSParams{
				ClientCert: getTestFixture(t, "db/client.pem"),
				ClientKey:  getTestFixture(t, "db/client-key.pem"),
			},
			hasError: true,
		},
		{
			name: "missing client cert fails to create client with error",
			uri:  "redis://localhost:6379",
			tlsParams: v1beta2.TLSParams{
				CACert:    getTestFixture(t, "db/ca.pem"),
				ClientKey: getTestFixture(t, "db/client-key.pem"),
			},
			hasError: true,
		},
		{
			name: "missing client key fails to create client with error",
			uri:  "redis://localhost:6379",
			tlsParams: v1beta2.TLSParams{
				CACert:     getTestFixture(t, "db/ca.pem"),
				ClientCert: getTestFixture(t, "db/client.pem"),
			},
			hasError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CollectRedis{
				Client: testclient.NewSimpleClientset(),
				Collector: &v1beta2.Database{
					URI: tt.uri,
					TLS: &tt.tlsParams,
				},
			}

			if tt.tlsParams.Secret != nil {
				// Create the secret in our fake client
				_, err := c.Client.CoreV1().Secrets(tt.tlsParams.Secret.Namespace).Create(
					context.Background(),
					&v1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name: tt.tlsParams.Secret.Name,
						},
						StringData: map[string]string{
							"cacert":     getTestFixture(t, "db/ca.pem"),
							"clientCert": getTestFixture(t, "db/client.pem"),
							"clientKey":  getTestFixture(t, "db/client-key.pem"),
						},
					},
					metav1.CreateOptions{},
				)
				require.NoError(t, err)
			}

			client, err := c.createClient()
			assert.Equalf(t, err != nil, tt.hasError, "createClient() error = %v, wantErr %v", err, tt.hasError)
			if err == nil {
				require.NotNil(t, client)
				opt := client.Options()
				assert.Equal(t, opt.Addr, "localhost:6379")

				// TLS parameter objects are opaque. Just check if they were created.
				// There is no trivial way to inspect their metadata. Trust me :)
				assert.NotNil(t, opt.TLSConfig.RootCAs)
				assert.NotNil(t, opt.TLSConfig.Certificates)
			} else {
				t.Log(err)
				assert.Nil(t, client)
			}
		})
	}
}

func getTestFixture(t *testing.T, path string) string {
	t.Helper()
	p := filepath.Join("../../testdata", path)
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	return string(b)
}
