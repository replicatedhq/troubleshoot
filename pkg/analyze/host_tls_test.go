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
				CollectorName: "test-tls",
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
				CollectorName: "test-tls",
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
				CollectorName: "test-tls",
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
				CollectorName: "test-tls",
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
		{
			name: "replicated.app results",
			// {"peer_certificates":[{"issuer":"E5","subject":"replicated.app","serial":"366647399446765119739694467366731491294821","not_before":"2025-02-12T20:48:06Z","not_after":"2025-05-13T20:48:05Z","is_ca":false,"raw":"MIIDkDCCAxagAwIBAgISBDV62JAIFJ/68CYg4GL6V75lMAoGCCqGSM49BAMDMDIxCzAJBgNVBAYTAlVTMRYwFAYDVQQKEw1MZXQncyBFbmNyeXB0MQswCQYDVQQDEwJFNTAeFw0yNTAyMTIyMDQ4MDZaFw0yNTA1MTMyMDQ4MDVaMBkxFzAVBgNVBAMTDnJlcGxpY2F0ZWQuYXBwMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEtLBSWSjTkS63NvKjChgv0IRLXQ+8qQVZfGa27M8Odvok+0nDivOLwvXToIfcsb87rj2ZolYaMZ51oZMAPTer8KOCAiMwggIfMA4GA1UdDwEB/wQEAwIHgDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwDAYDVR0TAQH/BAIwADAdBgNVHQ4EFgQUAcqA0Ax0SYkbpaTHUE1/3K+F+8MwHwYDVR0jBBgwFoAUnytfzzwhT50Et+0rLMTGcIvS1w0wVQYIKwYBBQUHAQEESTBHMCEGCCsGAQUFBzABhhVodHRwOi8vZTUuby5sZW5jci5vcmcwIgYIKwYBBQUHMAKGFmh0dHA6Ly9lNS5pLmxlbmNyLm9yZy8wKwYDVR0RBCQwIoIQKi5yZXBsaWNhdGVkLmFwcIIOcmVwbGljYXRlZC5hcHAwEwYDVR0gBAwwCjAIBgZngQwBAgEwggEFBgorBgEEAdZ5AgQCBIH2BIHzAPEAdgDM+w9qhXEJZf6Vm1PO6bJ8IumFXA2XjbapflTA/kwNsAAAAZT8INAKAAAEAwBHMEUCIQChEKizx3gdCm8rATD9JULAHFmhCcNCiPjLaKIuyV624AIgDIxiox82edgyw1ENlsCTVgM849qDBOLr41WeSIG04kwAdwATSt8atZhCCXgMb+9MepGkFrcjSc5YV2rfrtqnwqvgIgAAAZT8INHBAAAEAwBIMEYCIQCuxLNpzW2lYKwJ2uLBSu14wH0jc+oxrH4lA/QLXm2CeQIhAM66PhzUGhgrypwyKW0F9AwH73tPHoRZHWgDnWBjX7B+MAoGCCqGSM49BAMDA2gAMGUCMFMMHRfHegdMljQd68qhTv6hL0ySV9nn2u85mWcilDHUMbQSGaqH8liyMfNlR/a7+gIxALSdKsdnyrqAsViMDAMo56gwaq8EeGtSyfWfmPiRlRinn6ANwxf6bs6J3csBgM+hdw=="},{"issuer":"ISRG Root X1","subject":"E5","serial":"174873564306387906651619802726858882526","not_before":"2024-03-13T00:00:00Z","not_after":"2027-03-12T23:59:59Z","is_ca":true,"raw":"MIIEVzCCAj+gAwIBAgIRAIOPbGPOsTmMYgZigxXJ/d4wDQYJKoZIhvcNAQELBQAwTzELMAkGA1UEBhMCVVMxKTAnBgNVBAoTIEludGVybmV0IFNlY3VyaXR5IFJlc2VhcmNoIEdyb3VwMRUwEwYDVQQDEwxJU1JHIFJvb3QgWDEwHhcNMjQwMzEzMDAwMDAwWhcNMjcwMzEyMjM1OTU5WjAyMQswCQYDVQQGEwJVUzEWMBQGA1UEChMNTGV0J3MgRW5jcnlwdDELMAkGA1UEAxMCRTUwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAAQNCzqKa2GOtu/cX1jnxkJFVKtj9mZhSAouWXW0gQI3ULc/FnncmOyhKJdyIBwsz9V8UiBOVHhbhBRrwJCuhezAUUE8Wod/Bk3U/mDR+mwt4X2VEIiiCFQPmRpM5uoKrNijgfgwgfUwDgYDVR0PAQH/BAQDAgGGMB0GA1UdJQQWMBQGCCsGAQUFBwMCBggrBgEFBQcDATASBgNVHRMBAf8ECDAGAQH/AgEAMB0GA1UdDgQWBBSfK1/PPCFPnQS37SssxMZwi9LXDTAfBgNVHSMEGDAWgBR5tFnme7bl5AFzgAiIyBpY9umbbjAyBggrBgEFBQcBAQQmMCQwIgYIKwYBBQUHMAKGFmh0dHA6Ly94MS5pLmxlbmNyLm9yZy8wEwYDVR0gBAwwCjAIBgZngQwBAgEwJwYDVR0fBCAwHjAcoBqgGIYWaHR0cDovL3gxLmMubGVuY3Iub3JnLzANBgkqhkiG9w0BAQsFAAOCAgEAH3KdNEVCQdqk0LKyuNImTKdRJY1C2uw2SJajuhqkyGPY8C+zzsufZ+mgnhnq1A2KVQOSykOEnUbx1cy637rBAihx97r+bcwbZM6sTDIaEriR/PLk6LKs9Be0uoVxgOKDcpG9svD33J+G9Lcfv1K9luDmSTgG6XNFIN5vfI5gs/lMPyojEMdIzK9blcl2/1vKxO8WGCcjvsQ1nJ/Pwt8LQZBfOFyVXP8ubAp/au3dc4EKWG9MO5zcx1qT9+NXRGdVWxGvmBFRAajciMfXME1ZuGmk3/GOkoAM7ZkjZmleyokP1LGzmfJcUd9s7eeu1/9/eg5XlXd/55GtYjAM+C4DG5i7eaNqcm2F+yxYIPt6cbbtYVNJCGfHWqHEQ4FYStUyFnv8sjyqU8ypgZaNJ9aVcWSICLOIE1/Qv/7oKsnZCWJ926wU6RqG1OYPGOi1zuABhLw61cuPVDT28nQS/e6z95cJXq0eK1BcaJ6fJZsmbjRgD5p3mvEf5vdQM7MCEvU0tHbsx2I5mHHJoABHb8KVBgWp/lcXGWiWaeOyB7RP+OfDtvi2OsapxXiV7vNVs7fMlrRjY1joKaqmmycnBvAq14AEbtyLsVfOS66B8apkeFX2NY4XPEYV4ZSCe8VHPrdrERk2wILG3T/EGmSIkCYVUMSnjmJdVQD9F6Na/+zmXCc="}]}
			tlsInfo: &types.TLSInfo{
				PeerCertificates: []types.CertInfo{
					{
						Issuer:    "E5",
						Subject:   "replicated.app",
						Serial:    "366647399446765119739694467366731491294821",
						NotBefore: "2025-02-12T20:48:06Z",
						NotAfter:  "2025-05-13T20:48:05Z",
						IsCA:      false,
					},
					{
						Issuer:    "ISRG Root X1",
						Subject:   "E5",
						Serial:    "174873564306387906651619802726858882526",
						NotBefore: "2024-03-13T00:00:00Z",
						NotAfter:  "2027-03-12T23:59:59Z",
						IsCA:      true,
					},
				},
			},
			hostAnalyzer: &troubleshootv1beta2.TLSAnalyze{
				CollectorName: "test-tls",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "issuer == E5",
							Message: "The issuer for replicated.app is E5 (letsencrypt) as expected",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "",
							Message: "The issuer for replicated.app is not E5 (letsencrypt)",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "TLS",
					IsPass:  true,
					Message: "The issuer for replicated.app is E5 (letsencrypt) as expected",
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
