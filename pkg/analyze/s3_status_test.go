package analyzer

import (
	"encoding/json"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeS3Status(t *testing.T) {
	tests := []struct {
		name         string
		analyzer     *troubleshootv1beta2.DatabaseAnalyze
		collected    *collect.DatabaseConnection
		wantPass     bool
		wantFail     bool
		wantWarn     bool
		wantMessage  string
	}{
		{
			name: "connected, pass",
			analyzer: &troubleshootv1beta2.DatabaseAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "connected == false",
							Message: "Cannot access the S3 bucket.",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "connected == true",
							Message: "S3 bucket is accessible.",
						},
					},
				},
			},
			collected: &collect.DatabaseConnection{
				IsConnected: true,
			},
			wantPass:    true,
			wantMessage: "S3 bucket is accessible.",
		},
		{
			name: "not connected, fail with error appended",
			analyzer: &troubleshootv1beta2.DatabaseAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "connected == false",
							Message: "Cannot access the S3 bucket.",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "connected == true",
							Message: "S3 bucket is accessible.",
						},
					},
				},
			},
			collected: &collect.DatabaseConnection{
				IsConnected: false,
				Error:       "operation error S3: HeadBucket, StatusCode: 403",
			},
			wantFail:    true,
			wantMessage: "Cannot access the S3 bucket. operation error S3: HeadBucket, StatusCode: 403",
		},
		{
			name: "not connected, fail without error",
			analyzer: &troubleshootv1beta2.DatabaseAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "connected == false",
							Message: "Cannot access the S3 bucket.",
						},
					},
				},
			},
			collected: &collect.DatabaseConnection{
				IsConnected: false,
			},
			wantFail:    true,
			wantMessage: "Cannot access the S3 bucket.",
		},
		{
			name: "warn outcome",
			analyzer: &troubleshootv1beta2.DatabaseAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "connected == false",
							Message: "S3 bucket may be inaccessible.",
						},
					},
				},
			},
			collected: &collect.DatabaseConnection{
				IsConnected: false,
			},
			wantWarn:    true,
			wantMessage: "S3 bucket may be inaccessible.",
		},
		{
			name: "unconditional fail",
			analyzer: &troubleshootv1beta2.DatabaseAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "Always fails.",
						},
					},
				},
			},
			collected: &collect.DatabaseConnection{
				IsConnected: true,
			},
			wantFail:    true,
			wantMessage: "Always fails.",
		},
		{
			name: "custom collector name",
			analyzer: &troubleshootv1beta2.DatabaseAnalyze{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "My S3 Check",
				},
				CollectorName: "my-bucket",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "connected == true",
							Message: "Bucket OK.",
						},
					},
				},
			},
			collected: &collect.DatabaseConnection{
				IsConnected: true,
			},
			wantPass:    true,
			wantMessage: "Bucket OK.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collectedData, err := json.Marshal(tt.collected)
			require.NoError(t, err)

			a := &AnalyzeS3Status{analyzer: tt.analyzer}

			getFile := func(path string) ([]byte, error) {
				return collectedData, nil
			}

			results, err := a.Analyze(getFile, nil)
			require.NoError(t, err)
			require.Len(t, results, 1)

			result := results[0]
			assert.Equal(t, tt.wantPass, result.IsPass)
			assert.Equal(t, tt.wantFail, result.IsFail)
			assert.Equal(t, tt.wantWarn, result.IsWarn)
			assert.Equal(t, tt.wantMessage, result.Message)

			if tt.analyzer.CheckName != "" {
				assert.Equal(t, tt.analyzer.CheckName, result.Title)
			}
		})
	}
}
