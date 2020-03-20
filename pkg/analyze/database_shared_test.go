package analyzer

import (
	"testing"

	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.undefinedlabs.com/scopeagent"
)

func Test_compareDatabaseConditionalToActual(t *testing.T) {
	tests := []struct {
		name          string
		conditional   string
		result        collect.DatabaseConnection
		expectedMatch bool
	}{
		{
			name:        "not connected, expected not connected",
			conditional: "connected == false",
			result: collect.DatabaseConnection{
				IsConnected: false,
			},
			expectedMatch: true,
		},
		{
			name:        "connected, expected connected",
			conditional: "connected == true",
			result: collect.DatabaseConnection{
				IsConnected: true,
			},
			expectedMatch: true,
		},
		{
			name:        "not connected, expected connected",
			conditional: "connected == true",
			result: collect.DatabaseConnection{
				IsConnected: false,
			},
			expectedMatch: false,
		},
		{
			name:        "version 9.3.0, want > 10.0.0",
			conditional: "version >= 10.0.0",
			result: collect.DatabaseConnection{
				IsConnected: true,
				Version:     "9.3.0",
			},
			expectedMatch: false,
		},
		{
			name:        "version 12.0.0, want > 10.0.0",
			conditional: "version >= 10.0.0",
			result: collect.DatabaseConnection{
				IsConnected: true,
				Version:     "12.0.0",
			},
			expectedMatch: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scopetest := scopeagent.StartTest(t)
			defer scopetest.End()
			req := require.New(t)

			actual, err := compareDatabaseConditionalToActual(test.conditional, &test.result)
			req.NoError(err)

			assert.Equal(t, test.expectedMatch, actual)

		})
	}
}
