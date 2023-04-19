package analyzer

import (
	"fmt"
	"testing"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_certificates(t *testing.T) {
	tests := []struct {
		name           string
		analyzer       troubleshootv1beta2.CertificatesAnalyze
		expectResult   []*AnalyzeResult
		getFile        getFile
		filePath, file string
	}{
		{
			name: "pass case",
			analyzer: troubleshootv1beta2.CertificatesAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "certificate is valid",
						},
					},
				},
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  true,
					IsWarn:  false,
					IsFail:  false,
					Title:   "Cerfiticates Verification",
					Message: "ca.crt certificate is valid, obtained from kube-root-ca.crt configmap within kurl namespace",
				},
			},
			filePath: "certificates/certificates.json",
			file: `[{
				"source": {
					"configMap": "kube-root-ca.crt",
					"namespace": "kurl"
				},
				"certificateChain": [
					{
						"certificate": "ca.crt",
						"subject": "CN=kubernetes",
						"subjectAlternativeNames": [
							"kubernetes"
						],
						"issuer": "CN=kubernetes",
						"notAfter": "2033-04-16T00:30:20Z",
						"notBefore": "2023-04-19T00:30:20Z",
						"isValid": true,
						"isCA": true
					}
				]
			}]`,
		},
		{
			name: "failed case",
			analyzer: troubleshootv1beta2.CertificatesAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "notAfter < Today",
							Message: "certificate has expired",
						},
					},
				},
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "Cerfiticates Verification",
					Message: "ca.crt certificate has expired, obtained from kube-root-ca.crt configmap within kurl namespace",
				},
			},
			filePath: "certificates/certificates.json",
			file: `[{
				"source": {
					"configMap": "kube-root-ca.crt",
					"namespace": "kurl"
				},
				"certificateChain": [
					{
						"certificate": "ca.crt",
						"subject": "CN=kubernetes",
						"subjectAlternativeNames": [
							"kubernetes"
						],
						"issuer": "CN=kubernetes",
						"notAfter": "2023-04-16T00:30:20Z",
						"notBefore": "2021-04-19T00:30:20Z",
						"isValid": false,
						"isCA": true
					}
				]
			}]`,
		},
		{
			name: "warning case",
			analyzer: troubleshootv1beta2.CertificatesAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "notAfter < Today + 15 days",
							Message: "certificate is about to expire",
						},
					},
				},
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  true,
					IsFail:  false,
					Title:   "Cerfiticates Verification",
					Message: "ca.crt certificate is about to expire in 15 days, obtained from kube-root-ca.crt configmap within kurl namespace",
				},
			},
			filePath: "certificates/certificates.json",
			file: fmt.Sprintf(`[{
				"source": {
					"configMap": "kube-root-ca.crt",
					"namespace": "kurl"
				},
				"certificateChain": [
					{
						"certificate": "ca.crt",
						"subject": "CN=kubernetes",
						"subjectAlternativeNames": [
							"kubernetes"
						],
						"issuer": "CN=kubernetes",
						"notAfter": "%s",
						"notBefore": "2021-04-16T00:30:20Z",
						"isValid": true,
						"isCA": true
					}
				]
			}]`, time.Now().AddDate(0, 0, 5).Format("2006-01-02T15:04:05Z")),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)
			if test.getFile == nil {
				test.getFile = func(n string) ([]byte, error) {
					assert.Equal(t, n, test.filePath)
					return []byte(test.file), nil
				}
			}

			a := AnalyzeCertificates{
				analyzer: &test.analyzer,
			}

			actual, err := a.AnalyzeCertificates(&test.analyzer, test.getFile)
			t.Log(actual)
			req.NoError(err)

			assert.Equal(t, test.expectResult, actual)
		})
	}
}
