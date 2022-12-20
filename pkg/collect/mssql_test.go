package collect

import (
	"context"
	"testing"

	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func Test_parseMSSqlVersion(t *testing.T) {
	tests := []struct {
		mssqlVersion string
		expect       string
	}{
		{
			//  docker run -d --name pgnine -e POSTGRES_PASSWORD=password mssql:9
			mssqlVersion: `Microsoft Azure SQL Edge Developer (RTM) - 15.0.2000.1565 (ARM64)
	Jun 14 2022 00:37:12
	Copyright (C) 2019 Microsoft Corporation
	Linux (Ubuntu 18.04.6 LTS aarch64) <ARM64>`,
			expect: "15.0.2000.1565",
		},
	}
	for _, test := range tests {
		t.Run(test.mssqlVersion, func(t *testing.T) {
			req := require.New(t)
			actual, err := parseMSSqlVersion(test.mssqlVersion)
			req.NoError(err)

			assert.Equal(t, test.expect, actual)

		})
	}
}

func TestCollectMSSql_createConnectConfigPlainText(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		hasError bool
	}{
		{
			name: "valid uri creates mssql connection config successfully",
			uri:  "sqlserver://user:password@sql.contoso.com:1433/master",
		},
		{
			name:     "empty uri fails to create mssql connection config with error",
			uri:      "",
			hasError: true,
		},
		{
			name:     "invalid protocol fails to create mssql connection config with error",
			uri:      "http://somehost:5432",
			hasError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CollectMSSql{
				Context: context.Background(),
				Collector: &v1beta2.Database{
					URI: tt.uri,
				},
			}

			connCfg, err := c.createConnectConfig()
			assert.Equal(t, err != nil, tt.hasError)
			if err == nil {
				require.NotNil(t, connCfg)
				assert.Equal(t, connCfg.Host, "sql.contoso.com")
				assert.Equal(t, connCfg.Database, "master")
			} else {
				t.Log(err)
				assert.Nil(t, connCfg)
			}
		})
	}
}
