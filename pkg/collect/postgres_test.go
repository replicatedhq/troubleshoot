package collect

import (
	"context"
	"testing"

	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func Test_parsePostgresVersion(t *testing.T) {
	tests := []struct {
		postgresVersion string
		expect          string
	}{
		{
			//  docker run -d --name pgnine -e POSTGRES_PASSWORD=password postgres:9
			postgresVersion: "PostgreSQL 9.6.17 on x86_64-pc-linux-gnu (Debian 9.6.17-2.pgdg90+1), compiled by gcc (Debian 6.3.0-18+deb9u1) 6.3.0 20170516, 64-bit",
			expect:          "9.6.17",
		},
		{
			//  docker run -d --name pgten -e POSTGRES_PASSWORD=password postgres:10
			postgresVersion: "PostgreSQL 10.12 (Debian 10.12-2.pgdg90+1) on x86_64-pc-linux-gnu, compiled by gcc (Debian 6.3.0-18+deb9u1) 6.3.0 20170516, 64-bit",
			expect:          "10.12",
		},
		{
			//  docker run -d --name pgeleven -e POSTGRES_PASSWORD=password postgres:11
			postgresVersion: "PostgreSQL 11.7 (Debian 11.7-2.pgdg90+1) on x86_64-pc-linux-gnu, compiled by gcc (Debian 6.3.0-18+deb9u1) 6.3.0 20170516, 64-bit",
			expect:          "11.7",
		},
		{
			// docker run -d --name pgtwelve -e POSTGRES_PASSWORD=password postgres:12
			postgresVersion: "PostgreSQL 12.2 (Debian 12.2-2.pgdg100+1) on x86_64-pc-linux-gnu, compiled by gcc (Debian 8.3.0-6) 8.3.0, 64-bit",
			expect:          "12.2",
		},
	}
	for _, test := range tests {
		t.Run(test.postgresVersion, func(t *testing.T) {
			req := require.New(t)
			actual, err := parsePostgresVersion(test.postgresVersion)
			req.NoError(err)

			assert.Equal(t, test.expect, actual)

		})
	}
}

func TestCollectPostgres_createConnectConfigPlainText(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		hasError bool
	}{
		{
			name: "valid uri creates postgres connection config successfully",
			uri:  "postgresql://user:password@my-pghost:5432/defaultdb?sslmode=require",
		},
		{
			name:     "empty uri fails to create postgres connection config with error",
			uri:      "",
			hasError: true,
		},
		{
			name:     "invalid redis protocol fails to create postgres connection config with error",
			uri:      "http://somehost:5432",
			hasError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CollectPostgres{
				Context: context.Background(),
				Collector: &v1beta2.Database{
					URI: tt.uri,
				},
			}

			connCfg, err := c.createConnectConfig()
			assert.Equal(t, tt.hasError, err != nil)
			if err == nil {
				require.NotNil(t, connCfg)
				assert.Equal(t, connCfg.Host, "my-pghost")
				assert.Equal(t, connCfg.Database, "defaultdb")
			} else {
				t.Log(err)
				assert.Nil(t, connCfg)
			}
		})
	}
}

func TestCollectPostgres_createConnectConfigTLS(t *testing.T) {
	k8sClient := testclient.NewSimpleClientset()

	c := &CollectPostgres{
		Client:  k8sClient,
		Context: context.Background(),
		Collector: &v1beta2.Database{
			URI: "postgresql://user:password@my-pghost:5432/defaultdb?sslmode=require",
			TLS: &v1beta2.TLSParams{
				CACert:     getTestFixture(t, "db/ca.pem"),
				ClientCert: getTestFixture(t, "db/client.pem"),
				ClientKey:  getTestFixture(t, "db/client-key.pem"),
			},
		},
	}

	connCfg, err := c.createConnectConfig()
	assert.NoError(t, err)
	assert.NotNil(t, connCfg)
	assert.Equal(t, connCfg.Host, "my-pghost")
	assert.NotNil(t, connCfg.TLSConfig.Certificates)
	assert.NotNil(t, connCfg.TLSConfig.RootCAs)
	assert.False(t, connCfg.TLSConfig.InsecureSkipVerify)
}
