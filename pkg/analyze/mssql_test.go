package analyzer

import (
	"encoding/json"
	"reflect"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_compareMssqlConditionalToActual(t *testing.T) {
	tests := []struct {
		name        string
		conditional string
		conn        collect.DatabaseConnection
		hasError    bool
		expect      bool
	}{
		{
			name:        "Is Connected Succeeded",
			conditional: "connected == true",
			conn: collect.DatabaseConnection{
				IsConnected: true,
				Error:       "",
				Version:     "",
			},
			hasError: false,
			expect:   true,
		},
		{
			name:        "Is Not Connected Succeeded",
			conditional: "connected == false",
			conn: collect.DatabaseConnection{
				IsConnected: false,
				Error:       "",
				Version:     "",
			},
			hasError: false,
			expect:   true,
		},
		{
			name:        "Exact Match Version String Succeeded",
			conditional: "version == 15.0.2000.1565",
			conn: collect.DatabaseConnection{
				IsConnected: true,
				Error:       "",
				Version:     "15.0.2000.1565",
			},
			hasError: false,
			expect:   true,
		},
		{
			name:        "Less Than Version Match Succeeded",
			conditional: "version < 15.x",
			conn: collect.DatabaseConnection{
				IsConnected: true,
				Error:       "",
				Version:     "14.0.2000.0",
			},
			hasError: false,
			expect:   true,
		},
		{
			name:        "Inverse Less Than Version Match With Greater Than Version Succeeded",
			conditional: "version > 15.x",
			conn: collect.DatabaseConnection{
				IsConnected: true,
				Error:       "",
				Version:     "14.0.2000.0",
			},
			hasError: false,
			expect:   false,
		},
		{
			name:        "Incorrect Conditional Provided Errors",
			conditional: "",
			conn:        collect.DatabaseConnection{},
			hasError:    true,
			expect:      false,
		},
		{
			name:        "Four Part Version Wildcard Match Less Than Succeed",
			conditional: "version < 15.0.2000.x",
			conn: collect.DatabaseConnection{
				IsConnected: true,
				Error:       "",
				Version:     "15.0.1999.0",
			},
			hasError: false,
			expect:   true,
		},
		{
			name:        "Four Part Version Wildcard Match Greater Than Succeed",
			conditional: "version > 15.0.2000.x",
			conn: collect.DatabaseConnection{
				IsConnected: true,
				Error:       "",
				Version:     "15.0.2001.0",
			},
			hasError: false,
			expect:   true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)
			actual, err := compareMssqlConditionalToActual(test.conditional, &test.conn)
			if test.hasError {
				req.Error(err)
			} else {
				req.NoError(err)
			}
			assert.Equal(t, test.expect, actual)

		})
	}
}

func TestAnalyzeMssql_Analyze(t *testing.T) {
	tests := []struct {
		name     string
		analyzer *troubleshootv1beta2.DatabaseAnalyze
		want     []*AnalyzeResult
		data     map[string]any
		wantErr  bool
	}{
		{
			name: "mssql analyze with passing condition",
			data: map[string]any{
				"isConnected": true,
				"version":     "15.0.2000.1565",
			},
			analyzer: &troubleshootv1beta2.DatabaseAnalyze{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "Must be SQLServer 15.x or later",
				},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "The SQLServer connection checks out",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "connected == false",
							Message: "Cannot connect to SQLServer",
						},
					},
				},
			},
			want: []*AnalyzeResult{
				{
					Title:   "Must be SQLServer 15.x or later",
					Message: "The SQLServer connection checks out",
					IsPass:  true,
				},
			},
		},
		{
			name: "mssql analyze with failing condition",
			data: map[string]any{
				"isConnected": true,
				"version":     "14.0.2000.1565",
			},
			analyzer: &troubleshootv1beta2.DatabaseAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "version < 15.x",
							Message: "The SQLServer must be at least version 15",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "The SQLServer connection checks out",
						},
					},
				},
			},
			want: []*AnalyzeResult{
				{
					Title:   "mssql",
					Message: "The SQLServer must be at least version 15",
					IsFail:  true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AnalyzeMssql{
				analyzer: tt.analyzer,
			}

			getFile := func(_ string) ([]byte, error) {
				return json.Marshal(tt.data)
			}

			got, err := a.Analyze(getFile, nil)
			assert.Equalf(t, tt.wantErr, err != nil, "got error = %v, wantErr %v", err, tt.wantErr)

			got2 := fromPointerSlice(got)
			want2 := fromPointerSlice(tt.want)
			if !reflect.DeepEqual(got2, want2) {
				t.Errorf("got = %v, want %v", toJSON(got2), toJSON(want2))
			}
		})
	}
}

func fromPointerSlice(in []*AnalyzeResult) []AnalyzeResult {
	out := make([]AnalyzeResult, len(in))
	for i := range in {
		out[i] = *in[i]
	}
	return out
}

func toJSON(in any) string {
	out, _ := json.MarshalIndent(in, "", "  ")
	return string(out)
}
