package analyzer

import (
	"encoding/json"
	"testing"

	"github.com/replicatedhq/troubleshoot/pkg/analyze/types"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeHostTLS(t *testing.T) {
	tests := []struct {
		name         string
		tlsInfo      *types.TLSInfo
		hostAnalyzer *troubleshootv1beta2.TLSAnalyze
		result       []*AnalyzeResult
		expectErr    bool
	}{
		{
			name: "issuer passes",
			tlsInfo: &types.TLSInfo{
				PeerCertificates: []types.CertInfo{
					{
						Issuer: "foo",
					},
				},
			},
			hostAnalyzer: &troubleshootv1beta2.TLSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "issuer == abc",
							Message: "issuer was abc",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "issuer == foo",
							Message: "issuer was foo",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "issuer == bar",
							Message: "issuer was bar",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "TLS",
					IsPass:  true,
					Message: "issuer was foo",
				},
			},
		},

		{
			name: "invalid check type",
			tlsInfo: &types.TLSInfo{
				PeerCertificates: []types.CertInfo{
					{
						Issuer: "foo",
					},
				},
			},
			hostAnalyzer: &troubleshootv1beta2.TLSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When: "this is invalid",
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "fallthrough to fail",
			tlsInfo: &types.TLSInfo{
				PeerCertificates: []types.CertInfo{
					{
						Issuer: "foo",
					},
				},
			},
			hostAnalyzer: &troubleshootv1beta2.TLSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "issuer == abc",
							Message: "issuer was abc",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "",
							Message: "issuer was not abc",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "TLS",
					IsFail:  true,
					Message: "issuer was not abc",
				},
			},
		},
		{
			name: "second cert matches",
			tlsInfo: &types.TLSInfo{
				PeerCertificates: []types.CertInfo{
					{
						Issuer: "foo",
					},
					{
						Issuer: "bar",
					},
				},
			},
			hostAnalyzer: &troubleshootv1beta2.TLSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "issuer == bar",
							Message: "issuer was bar",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "",
							Message: "issuer was not bar",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "TLS",
					IsPass:  true,
					Message: "issuer was bar",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)
			b, err := json.Marshal(test.tlsInfo)
			if err != nil {
				t.Fatal(err)
			}

			getCollectedFileContents := func(filename string) ([]byte, error) {
				return b, nil
			}

			result, err := (&AnalyzeHostTLS{test.hostAnalyzer}).Analyze(getCollectedFileContents, nil)
			if test.expectErr {
				req.Error(err)
			} else {
				req.NoError(err)
			}

			assert.Equal(t, test.result, result)
		})
	}
}
