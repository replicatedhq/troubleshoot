package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_doCompareHostCPU(t *testing.T) {
	tests := []struct {
		name     string
		operator string
		desired  string
		actual   int
		expected bool
	}{
		{
			name:     "< 16",
			operator: "<",
			desired:  "16",
			actual:   8,
			expected: true,
		},
		{
			name:     "< 8 when actual is 8",
			operator: "<",
			desired:  "8",
			actual:   8,
			expected: false,
		},
		{
			name:     "<= 8 when actual is 8",
			operator: "<=",
			desired:  "8",
			actual:   8,
			expected: true,
		},
		{
			name:     "<= 8 when actual is 16",
			operator: "<=",
			desired:  "8",
			actual:   16,
			expected: false,
		},
		{
			name:     "== 8 when actual is 16",
			operator: "==",
			desired:  "8",
			actual:   16,
			expected: false,
		},
		{
			name:     "== 8 when actual is 8",
			operator: "==",
			desired:  "8",
			actual:   8,
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			actual, err := doCompareHostCPU(test.operator, test.desired, test.actual)
			req.NoError(err)

			assert.Equal(t, test.expected, actual)

		})
	}
}
