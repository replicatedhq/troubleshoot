package collect

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_parseMsSqlVersion(t *testing.T) {
	tests := []struct {
		mssqlVersion string
		expect       string
	}{
		{
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
			actual, err := parseMsSqlVersion(test.mssqlVersion)
			req.NoError(err)

			assert.Equal(t, test.expect, actual)

		})
	}
}
