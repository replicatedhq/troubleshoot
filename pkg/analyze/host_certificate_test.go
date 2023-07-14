package analyzer

import (
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeCertificate(t *testing.T) {
	tests := []struct {
		name         string
		status       string
		hostAnalyzer *troubleshootv1beta2.CertificateAnalyze
		result       []*AnalyzeResult
		expectErr    bool
	}{
		{
			name:   "key-pair-valid",
			status: collect.KeyPairValid,
			hostAnalyzer: &troubleshootv1beta2.CertificateAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-missing",
							Message: "Certificate key pair not found",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-switched",
							Message: "Public and private keys switched",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-encrypted",
							Message: "Private key is encrypted",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-mismatch",
							Message: "Public and private keys don't match",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-invalid",
							Message: "Certificate key pair is invalid",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-valid",
							Message: "Certificate key pair is valid",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Certificate Key Pair",
					IsPass:  true,
					Message: "Certificate key pair is valid",
				},
			},
		},
		{
			name:   "key-pair-invalid",
			status: collect.KeyPairInvalid,
			hostAnalyzer: &troubleshootv1beta2.CertificateAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-missing",
							Message: "Certificate key pair not found",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-switched",
							Message: "Public and private keys switched",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-encrypted",
							Message: "Private key is encrypted",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-mismatch",
							Message: "Public and private keys don't match",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-invalid",
							Message: "Certificate key pair is invalid",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-valid",
							Message: "Certificate key pair is valid",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Certificate Key Pair",
					IsFail:  true,
					Message: "Certificate key pair is invalid",
				},
			},
		},
		{
			name:   "key-pair-missing",
			status: collect.KeyPairMissing,
			hostAnalyzer: &troubleshootv1beta2.CertificateAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-missing",
							Message: "Certificate key pair not found",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-switched",
							Message: "Public and private keys switched",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-encrypted",
							Message: "Private key is encrypted",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-mismatch",
							Message: "Public and private keys don't match",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-invalid",
							Message: "Certificate key pair is invalid",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-valid",
							Message: "Certificate key pair is valid",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Certificate Key Pair",
					IsFail:  true,
					Message: "Certificate key pair not found",
				},
			},
		},
		{
			name:   "key-pair-switched",
			status: collect.KeyPairSwitched,
			hostAnalyzer: &troubleshootv1beta2.CertificateAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-missing",
							Message: "Certificate key pair not found",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-switched",
							Message: "Public and private keys switched",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-encrypted",
							Message: "Private key is encrypted",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-mismatch",
							Message: "Public and private keys don't match",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-invalid",
							Message: "Certificate key pair is invalid",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-valid",
							Message: "Certificate key pair is valid",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Certificate Key Pair",
					IsFail:  true,
					Message: "Public and private keys switched",
				},
			},
		},
		{
			name:   "key-pair-encrypted",
			status: collect.KeyPairEncrypted,
			hostAnalyzer: &troubleshootv1beta2.CertificateAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-missing",
							Message: "Certificate key pair not found",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-switched",
							Message: "Public and private keys switched",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-encrypted",
							Message: "Private key is encrypted",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-mismatch",
							Message: "Public and private keys don't match",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-invalid",
							Message: "Certificate key pair is invalid",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-valid",
							Message: "Certificate key pair is valid",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Certificate Key Pair",
					IsFail:  true,
					Message: "Private key is encrypted",
				},
			},
		},
		{
			name:   "key-pair-mismatch",
			status: collect.KeyPairMismatch,
			hostAnalyzer: &troubleshootv1beta2.CertificateAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-missing",
							Message: "Certificate key pair not found",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-switched",
							Message: "Public and private keys switched",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-encrypted",
							Message: "Private key is mismatch",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-mismatch",
							Message: "Public and private keys don't match",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-invalid",
							Message: "Certificate key pair is invalid",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "key-pair-valid",
							Message: "Certificate key pair is valid",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Certificate Key Pair",
					IsFail:  true,
					Message: "Public and private keys don't match",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			getCollectedFileContents := func(filename string) ([]byte, error) {
				return []byte(test.status), nil
			}

			result, err := (&AnalyzeHostCertificate{test.hostAnalyzer}).Analyze(getCollectedFileContents, nil)
			if test.expectErr {
				req.Error(err)
			} else {
				req.NoError(err)
			}

			assert.Equal(t, test.result, result)
		})
	}
}
