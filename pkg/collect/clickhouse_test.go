package collect

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/replicatedhq/troubleshoot/internal/testutils"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestCollectClickhouse_createConnectConfigPlainText(t *testing.T) {
	tests := []struct {
		name         string
		uri          string
		hasError     bool
		expectedHost string
	}{
		{
			name:         "valid uri creates clickhouse connection config successfully",
			uri:          "clickhouse://user:password@my-chhost:9000/defaultdb?dial_timeout=10s",
			expectedHost: "my-chhost",
		},
		{
			name:     "empty uri fails to create clickhouse connection config with error",
			uri:      "",
			hasError: true,
		},
		{
			name:         "invalid protocol creates clickhouse connection config with native protocol (default behaviour)",
			uri:          "postgresql://somehost:9000",
			hasError:     false,
			expectedHost: "somehost",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CollectClickHouse{
				Context: context.Background(),
				Collector: &v1beta2.Database{
					URI: tt.uri,
				},
			}

			connCfg, err := c.createConnectConfig()
			assert.Equal(t, tt.hasError, err != nil)
			if err == nil {
				require.NotNil(t, connCfg)
				if tt.expectedHost != "" {
					require.NotEmpty(t, connCfg.Addr)
					assert.Contains(t, connCfg.Addr[0], tt.expectedHost)
				}
			} else {
				t.Log(err)
				assert.Nil(t, connCfg)
			}
		})
	}
}

func TestCollectClickhouse_createConnectConfigTLS(t *testing.T) {
	k8sClient := testclient.NewClientset()

	c := &CollectClickHouse{
		Client:  k8sClient,
		Context: context.Background(),
		Collector: &v1beta2.Database{
			URI: "clickhouse://user:password@my-chhost:9440/defaultdb",
			TLS: &v1beta2.TLSParams{
				CACert:     testutils.GetTestFixture(t, "db/ca.pem"),
				ClientCert: testutils.GetTestFixture(t, "db/client.pem"),
				ClientKey:  testutils.GetTestFixture(t, "db/client-key.pem"),
				SkipVerify: false,
			},
		},
	}

	connCfg, err := c.createConnectConfig()
	assert.NoError(t, err)
	assert.NotNil(t, connCfg)
	require.NotEmpty(t, connCfg.Addr)
	assert.Contains(t, connCfg.Addr[0], "my-chhost")

	// Check TLS config exists
	assert.NotNil(t, connCfg.TLS)

	// Check client cert
	require.Len(t, connCfg.TLS.Certificates, 1)
	require.Len(t, connCfg.TLS.Certificates[0].Certificate, 1)
	cert := connCfg.TLS.Certificates[0]
	clientCert, err := x509.ParseCertificate(cert.Certificate[0])
	require.NoError(t, err)
	assert.Equal(t, "CN=client,L=Didcot,ST=Oxfordshire,C=UK", clientCert.Subject.String())

	// Check client key
	block, _ := pem.Decode([]byte(testutils.GetTestFixture(t, "db/client-key.pem")))
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	require.NoError(t, err)
	assert.True(t, key.Equal(cert.PrivateKey.(*rsa.PrivateKey)))

	assert.NotNil(t, connCfg.TLS.RootCAs)
	assert.False(t, connCfg.TLS.InsecureSkipVerify)
}
