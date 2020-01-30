package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.undefinedlabs.com/scopeagent"
)

func Test_compareDistributionConditionalToActual(t *testing.T) {
	test := scopeagent.StartTest(t)
	defer test.End()
	tests := []struct {
		name        string
		conditional string
		input       providers
		expected    bool
	}{
		{
			name:        "== microk8s when microk8s is found",
			conditional: "== microk8s",
			input: providers{
				microk8s: true,
			},
			expected: true,
		},
		{
			name:        "!= microk8s when microk8s is found",
			conditional: "!= microk8s",
			input: providers{
				microk8s: true,
			},
			expected: false,
		},
		{
			name:        "!== eks when gke is found",
			conditional: "!== eks",
			input: providers{
				gke: true,
			},
			expected: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			actual, err := compareDistributionConditionalToActual(test.conditional, test.input)
			req.NoError(err)

			assert.Equal(t, test.expected, actual)
		})
	}
}

func Test_mustNormalizeDistributionName(t *testing.T) {
	test := scopeagent.StartTest(t)
	defer test.End()
	tests := []struct {
		raw      string
		expected Provider
	}{
		{
			raw:      "microk8s",
			expected: microk8s,
		},
		{
			raw:      "MICROK8S",
			expected: microk8s,
		},
		{
			raw:      " microk8s ",
			expected: microk8s,
		},
		{
			raw:      "Docker-Desktop",
			expected: dockerDesktop,
		},
	}

	for _, test := range tests {
		t.Run(test.raw, func(t *testing.T) {
			actual := mustNormalizeDistributionName(test.raw)

			assert.Equal(t, test.expected, actual)
		})
	}
}
