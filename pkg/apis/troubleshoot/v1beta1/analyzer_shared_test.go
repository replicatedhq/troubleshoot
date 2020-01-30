package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.undefinedlabs.com/scopeagent"
	"gopkg.in/yaml.v2"
)

func TestAnalyze_Unmarshal(t *testing.T) {
	test := scopeagent.StartTest(t)
	defer test.End()
	tests := []struct {
		name         string
		spec         string
		expectObject Analyze
	}{
		{
			name: "clusterVersion",
			spec: `clusterVersion:
  outcomes:
    - fail:
        message: failed
    - pass:
        message: passed`,
			expectObject: Analyze{
				ClusterVersion: &ClusterVersion{
					Outcomes: []*Outcome{
						&Outcome{
							Fail: &SingleOutcome{
								Message: "failed",
							},
						},
						&Outcome{
							Pass: &SingleOutcome{
								Message: "passed",
							},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			a := Analyze{}
			err := yaml.Unmarshal([]byte(test.spec), &a)
			req.NoError(err)

			assert.Equal(t, test.expectObject, a)
		})
	}
}
