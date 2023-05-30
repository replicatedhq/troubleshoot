package analyzer

import (
	"fmt"
	"testing"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeCertificates(t *testing.T) {
	tests := []struct {
		name         string
		file         string
		hostAnalyzer *troubleshootv1beta2.CertificateAnalyze
		result       []*AnalyzeResult
		expectErr    bool
	}{
		{
			name: "certificate-valid",
			file: fmt.Sprintf(`[{
				"certificatePath": "apiserver-kubelet-client.crt",
				"certificateChain": [
					{
						"certificate": "ca.crt",
						"subject": "CN=kubernetes",
						"subjectAlternativeNames": [
							"kubernetes"
						],
						"issuer": "CN=kubernetes",
						"notAfter": "%s",
						"notBefore": "2023-04-19T00:30:20Z",
						"isValid": true,
						"isCA": true
					}
				],
				"message": "cert-valid"
			}]`, time.Now().AddDate(1, 0, 0).Format("2006-01-02T15:04:05Z")),
			hostAnalyzer: &troubleshootv1beta2.CertificateAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "Certificate is valid",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					IsPass:  true,
					IsWarn:  false,
					IsFail:  false,
					Title:   "Host Cerfiticates Verification",
					Message: "Certificate is valid, obtained from apiserver-kubelet-client.crt",
				},
			},
		},
		{
			name: "certificate-invalid",
			file: `[{
				"certificatePath": "apiserver-kubelet-client.crt",
				"certificateChain": [
					{
						"certificate": "ca.crt",
						"subject": "CN=kubernetes",
						"subjectAlternativeNames": [
							"kubernetes"
						],
						"issuer": "CN=kubernetes",
						"notAfter": "2022-04-16T00:30:20Z",
						"notBefore": "2021-04-19T00:30:20Z",
						"isValid": false,
						"isCA": true
					}
				],
				"message": "cert-invalid"
			}]`,
			hostAnalyzer: &troubleshootv1beta2.CertificateAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "notAfter < Today",
							Message: "Certificate has expired",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "Host Cerfiticates Verification",
					Message: "Certificate has expired, obtained from apiserver-kubelet-client.crt",
				},
			},
		},
		{
			name: "certificate-about-to-expire",
			file: fmt.Sprintf(`[{
				"certificatePath": "apiserver-kubelet-client.crt",
				"certificateChain": [
					{
						"certificate": "ca.crt",
						"subject": "CN=kubernetes",
						"subjectAlternativeNames": [
							"kubernetes"
						],
						"issuer": "CN=kubernetes",
						"notAfter": "%s",
						"notBefore": "2021-04-19T00:30:20Z",
						"isValid": true,
						"isCA": true
					}
				],
				"message": "cert-valid"
			}]`, time.Now().AddDate(0, 0, 5).Format("2006-01-02T15:04:05Z")),
			hostAnalyzer: &troubleshootv1beta2.CertificateAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "notAfter < Today + 15 days",
							Message: "Certificate is about to expire",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  true,
					IsFail:  false,
					Title:   "Host Cerfiticates Verification",
					Message: "Certificate is about to expire in 15 days, obtained from apiserver-kubelet-client.crt",
				},
			},
		},
		{
			name: "certificate-missing",
			file: `[{
				"certificatePath": "apiserver-kubelet-client.crt",
				"message": "cert-missing"
			}]`,
			hostAnalyzer: &troubleshootv1beta2.CertificateAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{},
			},
			result: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "Host Cerfiticates Verification",
					Message: "Certificate is missing, cannot be obtained from apiserver-kubelet-client.crt",
				},
			},
		},
		{
			name: "certificate-valid-and-about-to-expire",
			file: fmt.Sprintf(`[{
				"certificatePath": "apiserver-kubelet-client.crt",
				"certificateChain": [
					{
						"certificate": "ca.crt",
						"subject": "CN=kubernetes",
						"subjectAlternativeNames": [
							"kubernetes"
						],
						"issuer": "CN=kubernetes",
						"notAfter": "%s",
						"notBefore": "2021-04-19T00:30:20Z",
						"isValid": true,
						"isCA": true
					}
				],
				"message": "cert-valid"
			}]`, time.Now().AddDate(0, 0, 5).Format("2006-01-02T15:04:05Z")),
			hostAnalyzer: &troubleshootv1beta2.CertificateAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "Certificate is valid",
						},
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "notAfter < Today + 15 days",
							Message: "Certificate is about to expire",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  true,
					IsFail:  false,
					Title:   "Host Cerfiticates Verification",
					Message: "Certificate is about to expire in 15 days, obtained from apiserver-kubelet-client.crt",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			getCollectedFileContents := func(filename string) ([]byte, error) {
				return []byte(test.file), nil
			}

			result, err := (&AnalyzeHostCertificates{test.hostAnalyzer}).Analyze(getCollectedFileContents)
			if test.expectErr {
				req.Error(err)
			} else {
				req.NoError(err)
			}

			assert.Equal(t, test.result, result)
		})
	}
}
