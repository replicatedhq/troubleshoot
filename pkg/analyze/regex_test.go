package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_compareRegex(t *testing.T) {
	tests := []struct {
		name         string
		conditional  string
		foundMatches map[string]string
		expected     bool
	}{
		{
			name:        "Loss < 5",
			conditional: "Loss < 5",
			foundMatches: map[string]string{
				"Transmitted": "5",
				"Received":    "4",
				"Loss":        "20",
			},
			expected: true,
		},
		{
			name:        "Hostname = icecream",
			conditional: "Hostname = icecream",
			foundMatches: map[string]string{
				"Something": "5",
				"Hostname":  "icecream",
			},
			expected: true,
		},
		{
			name:        "Day >= 23",
			conditional: "Day >= 23",
			foundMatches: map[string]string{
				"day": "5",
				"Day": "24",
			},
			expected: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			actual, err := compareRegex(test.conditional, test.foundMatches)
			req.NoError(err)

			assert.Equal(t, test.expected, actual)
		})
	}

}
