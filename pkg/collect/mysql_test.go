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

func TestCollectMysql_createConnectConfigPlainText(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		hasError bool
	}{
		{
			name: "valid uri creates mysql connection config successfully",
			uri:  "user:password@tcp(localhost:3306)/defaultdb",
		},
		{
			name:     "empty uri fails to create mysql connection config with error",
			uri:      "",
			hasError: true,
		},
		{
			name:     "invalid protocol fails to create mysql connection config with error",
			uri:      "http://somehost:3306",
			hasError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CollectMysql{
				Context: context.Background(),
				Collector: &v1beta2.Database{
					URI: tt.uri,
				},
			}

			cfg, err := c.createConnectConfig()
			assert.Equal(t, tt.hasError, err != nil)
			if err == nil {
			require.NotNil(t, cfg)
			assert.Equal(t, "localhost:3306", cfg.Addr)
			assert.Equal(t, "defaultdb", cfg.DBName)
			} else {
				t.Log(err)
				assert.Nil(t, cfg)
			}
		})
	}
}

func TestCollectMysql_createConnectConfigTLS(t *testing.T) {
	k8sClient := testclient.NewSimpleClientset()

	c := &CollectMysql{
		Client:  k8sClient,
		Context: context.Background(),
		Collector: &v1beta2.Database{
			URI: "user:password@tcp(localhost:3306)/defaultdb",
			TLS: &v1beta2.TLSParams{
				CACert:     testutils.GetTestFixture(t, "db/ca.pem"),
				ClientCert: testutils.GetTestFixture(t, "db/client.pem"),
				ClientKey:  testutils.GetTestFixture(t, "db/client-key.pem"),
			},
		},
	}

	cfg, err := c.createConnectConfig()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "localhost:3306", cfg.Addr)

	// Check TLS config exists and is configured correctly
	require.NotNil(t, cfg.TLS)

	// Check client cert
	require.Len(t, cfg.TLS.Certificates, 1)
	require.Len(t, cfg.TLS.Certificates[0].Certificate, 1)
	cert := cfg.TLS.Certificates[0]
	clientCert, err := x509.ParseCertificate(cert.Certificate[0])
	require.NoError(t, err)
	assert.Equal(t, "CN=client,L=Didcot,ST=Oxfordshire,C=UK", clientCert.Subject.String())

	// Check client key
	block, _ := pem.Decode([]byte(testutils.GetTestFixture(t, "db/client-key.pem")))
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	require.NoError(t, err)
	assert.True(t, key.Equal(cert.PrivateKey.(*rsa.PrivateKey)))

	assert.NotNil(t, cfg.TLS.RootCAs)
	assert.False(t, cfg.TLS.InsecureSkipVerify)
}

func TestCollectMysql_createConnectConfigTLSSkipVerify(t *testing.T) {
	c := &CollectMysql{
		Context: context.Background(),
		Collector: &v1beta2.Database{
			URI: "user:password@tcp(localhost:3306)/defaultdb",
			TLS: &v1beta2.TLSParams{
				SkipVerify: true,
			},
		},
	}

	cfg, err := c.createConnectConfig()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	require.NotNil(t, cfg.TLS)
	assert.True(t, cfg.TLS.InsecureSkipVerify)
}

func TestCollectMysql_createConnectConfigTLSSecret(t *testing.T) {
	k8sClient := testclient.NewSimpleClientset()

	c := &CollectMysql{
		Client:  k8sClient,
		Context: context.Background(),
		Collector: &v1beta2.Database{
			URI: "user:password@tcp(localhost:3306)/defaultdb",
			TLS: &v1beta2.TLSParams{
				Secret: createTLSSecret(t, k8sClient, map[string]string{
					"cacert":     testutils.GetTestFixture(t, "db/ca.pem"),
					"clientCert": testutils.GetTestFixture(t, "db/client.pem"),
					"clientKey":  testutils.GetTestFixture(t, "db/client-key.pem"),
				}),
			},
		},
	}

	cfg, err := c.createConnectConfig()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	require.NotNil(t, cfg.TLS)
	assert.NotNil(t, cfg.TLS.RootCAs)
	assert.NotEmpty(t, cfg.TLS.Certificates)
	assert.False(t, cfg.TLS.InsecureSkipVerify)
}
