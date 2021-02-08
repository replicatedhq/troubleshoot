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

func Test_compareHostCPUConditionalToActual(t *testing.T) {
	tests := []struct {
		name          string
		when          string
		logicalCount  int
		physicalCount int
		expected      bool
	}{
		{
			name:          "physical > 4, when physical is 8",
			when:          "physical > 4",
			logicalCount:  0,
			physicalCount: 8,
			expected:      true,
		},
		{
			name:          "physical > 4, when physical is 4",
			when:          "physical > 4",
			logicalCount:  0,
			physicalCount: 4,
			expected:      false,
		},
		{
			name:          "physical > 4, when physical is 3, logical is 6",
			when:          "physical > 4",
			logicalCount:  6,
			physicalCount: 3,
			expected:      false,
		},
		{
			name:          "logical > 4, when physical is 4, logical is 8",
			when:          "logical > 4",
			logicalCount:  8,
			physicalCount: 4,
			expected:      true,
		},
		{
			name:          ">= 4, when physical is 2, logical is 4",
			when:          ">= 4",
			logicalCount:  4,
			physicalCount: 2,
			expected:      true,
		},
		{
			name:          "count < 4, when physical is 2, logical is 4",
			when:          "count < 4",
			logicalCount:  4,
			physicalCount: 2,
			expected:      false,
		},
		{
			name:          "count <= 4, when physical is 2, logical is 4",
			when:          "count <= 4",
			logicalCount:  4,
			physicalCount: 2,
			expected:      true,
		},
		{
			name:          "== 4, physical is 4, logical is 4",
			when:          "== 4",
			logicalCount:  4,
			physicalCount: 4,
			expected:      true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			actual, err := compareHostCPUConditionalToActual(test.when, test.logicalCount, test.physicalCount)
			req.NoError(err)

			assert.Equal(t, test.expected, actual)
		})
	}
}
