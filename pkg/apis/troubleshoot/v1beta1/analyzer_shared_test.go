package v1beta1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.undefinedlabs.com/scopeagent"
)

func TestAnalyze_Unmarshal(t *testing.T) {
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
			scopetest := scopeagent.StartTest(t)
			defer scopetest.End()
			req := require.New(t)

			a := Analyze{}
			err := json.Unmarshal([]byte(test.spec), &a)
			req.NoError(err)

			assert.Equal(t, test.expectObject, a)
		})
	}
}
