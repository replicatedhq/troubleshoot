package analyzer

import (
	"testing"

	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_compareMssqlConditionalToActual(t *testing.T) {
	tests := []struct {
		name        string
		conditional string
		conn        collect.DatabaseConnection
		hasError    bool
		expect      bool
	}{
		{
			name:        "Is Connected Succeeded",
			conditional: "connected == true",
			conn: collect.DatabaseConnection{
				IsConnected: true,
				Error:       "",
				Version:     "",
			},
			hasError: false,
			expect:   true,
		},
		{
			name:        "Is Not Connected Succeeded",
			conditional: "connected == false",
			conn: collect.DatabaseConnection{
				IsConnected: false,
				Error:       "",
				Version:     "",
			},
			hasError: false,
			expect:   true,
		},
		{
			name:        "Exact Match Version String Succeeded",
			conditional: "version == 15.0.2000.1565",
			conn: collect.DatabaseConnection{
				IsConnected: true,
				Error:       "",
				Version:     "15.0.2000.1565",
			},
			hasError: false,
			expect:   true,
		},
		{
			name:        "Less Than Version Match Succeeded",
			conditional: "version < 15.x",
			conn: collect.DatabaseConnection{
				IsConnected: true,
				Error:       "",
				Version:     "14.0.2000.0",
			},
			hasError: false,
			expect:   true,
		},
		{
			name:        "Inverse Less Than Version Match With Greater Than Version Succeeded",
			conditional: "version > 15.x",
			conn: collect.DatabaseConnection{
				IsConnected: true,
				Error:       "",
				Version:     "14.0.2000.0",
			},
			hasError: false,
			expect:   false,
		},
		{
			name:        "Incorrect Conditional Provided Errors",
			conditional: "",
			conn:        collect.DatabaseConnection{},
			hasError:    true,
			expect:      false,
		},
		{
			name:        "Four Part Version Wildcard Match Less Than Succeed",
			conditional: "version < 15.0.2000.x",
			conn: collect.DatabaseConnection{
				IsConnected: true,
				Error:       "",
				Version:     "15.0.1999.0",
			},
			hasError: false,
			expect:   true,
		},
		{
			name:        "Four Part Version Wildcard Match Greater Than Succeed",
			conditional: "version > 15.0.2000.x",
			conn: collect.DatabaseConnection{
				IsConnected: true,
				Error:       "",
				Version:     "15.0.2001.0",
			},
			hasError: false,
			expect:   true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)
			actual, err := compareMssqlConditionalToActual(test.conditional, &test.conn)
			if test.hasError {
				req.Error(err)
			} else {
				req.NoError(err)
			}
			assert.Equal(t, test.expect, actual)

		})
	}
}
