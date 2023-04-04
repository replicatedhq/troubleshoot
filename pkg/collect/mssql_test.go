package collect

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_parseMsSqlVersion(t *testing.T) {
	tests := []struct {
		name         string
		mssqlVersion string
		expect       string
		hasError     bool
	}{
		{
			name: "Valid String Succeed",
			mssqlVersion: `Microsoft Azure SQL Edge Developer (RTM) - 15.0.2000.1565 (ARM64)
			Jun 14 2022 00:37:12
			Copyright (C) 2019 Microsoft Corporation
			Linux (Ubuntu 18.04.6 LTS aarch64) <ARM64>`,
			expect:   "15.0.2000.1565",
			hasError: false,
		},
		{
			name: "SemVer String Pass",
			mssqlVersion: `Microsoft Azure SQL Edge Developer (RTM) - 15.0.1565 (ARM64)
			Jun 14 2022 00:37:12
			Copyright (C) 2019 Microsoft Corporation
			Linux (Ubuntu 18.04.6 LTS aarch64) <ARM64>`,
			expect:   "15.0.1565",
			hasError: false,
		},
		{
			name: "Missing SQL Fail",
			mssqlVersion: `Microsoft Azure Edge Developer (RTM) - 15.1.2020.1565 (ARM64)
			Jun 14 2022 00:37:12
			Copyright (C) 2019 Microsoft Corporation
			Linux (Ubuntu 18.04.6 LTS aarch64) <ARM64>`,
			expect:   "",
			hasError: true,
		},
		{
			name:         "Empty String Fail",
			mssqlVersion: "",
			expect:       "",
			hasError:     true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)
			actual, err := parseMsSqlVersion(test.mssqlVersion)
			if test.hasError {
				req.Error(err)
			} else {
				req.NoError(err)
			}
			assert.Equal(t, test.expect, actual)

		})
	}
}
