package analyzer

import (
	"encoding/json"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeHostHTTP(t *testing.T) {
	tests := []struct {
		name         string
		httpResult   *httpResult
		hostAnalyzer *troubleshootv1beta2.HTTPAnalyze
		result       []*AnalyzeResult
		expectErr    bool
	}{
		{
			name: "error",
			httpResult: &httpResult{
				Error: &collect.HTTPError{
					Message: "i/o timeout",
				},
			},
			hostAnalyzer: &troubleshootv1beta2.HTTPAnalyze{
				CollectorName: "registry",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "error",
							Message: "Failed to reach replicated.registry.com",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "HTTP Request",
					IsFail:  true,
					Message: "Failed to reach replicated.registry.com",
				},
			},
		},
		{
			name: "200",
			httpResult: &httpResult{
				Response: &collect.HTTPResponse{
					Status: 200,
				},
			},
			hostAnalyzer: &troubleshootv1beta2.HTTPAnalyze{
				CollectorName: "registry",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "error",
							Message: "Failed to reach replicated.registry.com",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "statusCode == 204",
							Message: "No content",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "statusCode == 200",
							Message: "Successfully reached registry",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "HTTP Request",
					IsPass:  true,
					Message: "Successfully reached registry",
				},
			},
		},
		{
			name: "skip default on pass",
			httpResult: &httpResult{
				Response: &collect.HTTPResponse{
					Status: 200,
				},
			},
			hostAnalyzer: &troubleshootv1beta2.HTTPAnalyze{
				CollectorName: "collector",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "statusCode == 200",
							Message: "passed",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							Message: "default",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "HTTP Request",
					IsPass:  true,
					Message: "passed",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)
			b, err := json.Marshal(test.httpResult)
			if err != nil {
				t.Fatal(err)
			}

			getCollectedFileContents := func(filename string) ([]byte, error) {
				return b, nil
			}

			result, err := (&AnalyzeHostHTTP{test.hostAnalyzer}).Analyze(getCollectedFileContents)
			if test.expectErr {
				req.Error(err)
			} else {
				req.NoError(err)
			}

			assert.Equal(t, test.result, result)
		})
	}
}
