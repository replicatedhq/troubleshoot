package analyzer

import (
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_compareRuntimeConditionalToActual(t *testing.T) {
	tests := []struct {
		name        string
		conditional string
		actual      string
		expected    bool
	}{
		{
			name:        "containerd://1.2.5 = containerd",
			conditional: "= containerd",
			actual:      "containerd://1.2.5",
			expected:    true,
		},
		{
			name:        "containerd://1.2.5 == containerd",
			conditional: "== containerd",
			actual:      "containerd://1.2.5",
			expected:    true,
		},
		{
			name:        "containerd://1.2.5 === containerd",
			conditional: "=== containerd",
			actual:      "containerd://1.2.5",
			expected:    true,
		},
		{
			name:        "containerd://1.2.5 != containerd",
			conditional: "!= containerd",
			actual:      "containerd://1.2.5",
			expected:    false,
		},
		{
			name:        "containerd://1.2.5 !== containerd",
			conditional: "!== containerd",
			actual:      "containerd://1.2.5",
			expected:    false,
		},
		{
			name:        "containerd://1.2.5 !== containerd",
			conditional: "!=== containerd",
			actual:      "containerd://1.2.5",
			expected:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			actual, err := compareRuntimeConditionalToActual(test.conditional, test.actual)
			req.NoError(err)

			assert.Equal(t, test.expected, actual)

		})
	}
}

func Test_containerRuntime(t *testing.T) {
	tests := []struct {
		name         string
		analyzer     troubleshootv1beta2.ContainerRuntime
		expectResult AnalyzeResult
		files        map[string][]byte
	}{
		{
			name: "no containerd, when it's containerd",
			analyzer: troubleshootv1beta2.ContainerRuntime{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "!= containerd",
							Message: "pass",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "containerd detected",
						},
					},
				},
			},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  false,
				IsFail:  true,
				Title:   "Container Runtime",
				Message: "containerd detected",
				IconKey: "kubernetes_container_runtime",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/container-runtime.svg?w=23&h=16",
			},
			files: map[string][]byte{
				"cluster-resources/nodes.json": []byte(collectedNodes),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			getFiles := func(n string) ([]byte, error) {
				return test.files[n], nil
			}

			a := AnalyzeContainerRuntime{
				analyzer: &test.analyzer,
			}

			actual, err := a.analyzeContainerRuntime(&test.analyzer, getFiles)
			req.NoError(err)

			assert.Equal(t, &test.expectResult, actual)

		})
	}
}
