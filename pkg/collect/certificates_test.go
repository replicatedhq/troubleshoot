package collect

import (
	"context"
	"testing"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
)

var certChains = map[string]string{
	"expiredCert": `-----BEGIN CERTIFICATE-----
MIIB0zCCAX2gAwIBAgIJAI/M7BYjwB+uMA0GCSqGSIb3DQEBBQUAMEUxCzAJBgNV
BAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBX
aWRnaXRzIFB0eSBMdGQwHhcNMTIwOTEyMjE1MjAyWhcNMTUwOTEyMjE1MjAyWjBF
MQswCQYDVQQGEwJBVTETMBEGA1UECAwKU29tZS1TdGF0ZTEhMB8GA1UECgwYSW50
ZXJuZXQgV2lkZ2l0cyBQdHkgTHRkMFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBANLJ
hPHhITqQbPklG3ibCVxwGMRfp/v4XqhfdQHdcVfHap6NQ5Wok/4xIA+ui35/MmNa
rtNuC+BdZ1tMuVCPFZcCAwEAAaNQME4wHQYDVR0OBBYEFJvKs8RfJaXTH08W+SGv
zQyKn0H8MB8GA1UdIwQYMBaAFJvKs8RfJaXTH08W+SGvzQyKn0H8MAwGA1UdEwQF
MAMBAf8wDQYJKoZIhvcNAQEFBQADQQBJlffJHybjDGxRMqaRmDhX0+6v02TUKZsW
r5QuVbpQhH6u+0UgcW0jp9QwpxoPTLTWGXEWBBBurxFwiCBhkQ+V
-----END CERTIFICATE-----`,
	"multiCert": `-----BEGIN CERTIFICATE-----
MIIG5jCCBc6gAwIBAgIQAze5KDR8YKauxa2xIX84YDANBgkqhkiG9w0BAQUFADBs
MQswCQYDVQQGEwJVUzEVMBMGA1UEChMMRGlnaUNlcnQgSW5jMRkwFwYDVQQLExB3
d3cuZGlnaWNlcnQuY29tMSswKQYDVQQDEyJEaWdpQ2VydCBIaWdoIEFzc3VyYW5j
ZSBFViBSb290IENBMB4XDTA3MTEwOTEyMDAwMFoXDTIxMTExMDAwMDAwMFowaTEL
MAkGA1UEBhMCVVMxFTATBgNVBAoTDERpZ2lDZXJ0IEluYzEZMBcGA1UECxMQd3d3
LmRpZ2ljZXJ0LmNvbTEoMCYGA1UEAxMfRGlnaUNlcnQgSGlnaCBBc3N1cmFuY2Ug
RVYgQ0EtMTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAPOWYth1bhn/
PzR8SU8xfg0ETpmB4rOFVZEwscCvcLssqOcYqj9495BoUoYBiJfiOwZlkKq9ZXbC
7L4QWzd4g2B1Rca9dKq2n6Q6AVAXxDlpufFP74LByvNK28yeUE9NQKM6kOeGZrzw
PnYoTNF1gJ5qNRQ1A57bDIzCKK1Qss72kaPDpQpYSfZ1RGy6+c7pqzoC4E3zrOJ6
4GAiBTyC01Li85xH+DvYskuTVkq/cKs+6WjIHY9YHSpNXic9rQpZL1oRIEDZaARo
LfTAhAsKG3jf7RpY3PtBWm1r8u0c7lwytlzs16YDMqbo3rcoJ1mIgP97rYlY1R4U
pPKwcNSgPqcCAwEAAaOCA4UwggOBMA4GA1UdDwEB/wQEAwIBhjA7BgNVHSUENDAy
BggrBgEFBQcDAQYIKwYBBQUHAwIGCCsGAQUFBwMDBggrBgEFBQcDBAYIKwYBBQUH
AwgwggHEBgNVHSAEggG7MIIBtzCCAbMGCWCGSAGG/WwCATCCAaQwOgYIKwYBBQUH
AgEWLmh0dHA6Ly93d3cuZGlnaWNlcnQuY29tL3NzbC1jcHMtcmVwb3NpdG9yeS5o
dG0wggFkBggrBgEFBQcCAjCCAVYeggFSAEEAbgB5ACAAdQBzAGUAIABvAGYAIAB0
AGgAaQBzACAAQwBlAHIAdABpAGYAaQBjAGEAdABlACAAYwBvAG4AcwB0AGkAdAB1
AHQAZQBzACAAYQBjAGMAZQBwAHQAYQBuAGMAZQAgAG8AZgAgAHQAaABlACAARABp
AGcAaQBDAGUAcgB0ACAARQBWACAAQwBQAFMAIABhAG4AZAAgAHQAaABlACAAUgBl
AGwAeQBpAG4AZwAgAFAAYQByAHQAeQAgAEEAZwByAGUAZQBtAGUAbgB0ACAAdwBo
AGkAYwBoACAAbABpAG0AaQB0ACAAbABpAGEAYgBpAGwAaQB0AHkAIABhAG4AZAAg
AGEAcgBlACAAaQBuAGMAbwByAHAAbwByAGEAdABlAGQAIABoAGUAcgBlAGkAbgAg
AGIAeQAgAHIAZQBmAGUAcgBlAG4AYwBlAC4wEgYDVR0TAQH/BAgwBgEB/wIBADCB
gwYIKwYBBQUHAQEEdzB1MCQGCCsGAQUFBzABhhhodHRwOi8vb2NzcC5kaWdpY2Vy
dC5jb20wTQYIKwYBBQUHMAKGQWh0dHA6Ly93d3cuZGlnaWNlcnQuY29tL0NBQ2Vy
dHMvRGlnaUNlcnRIaWdoQXNzdXJhbmNlRVZSb290Q0EuY3J0MIGPBgNVHR8EgYcw
gYQwQKA+oDyGOmh0dHA6Ly9jcmwzLmRpZ2ljZXJ0LmNvbS9EaWdpQ2VydEhpZ2hB
c3N1cmFuY2VFVlJvb3RDQS5jcmwwQKA+oDyGOmh0dHA6Ly9jcmw0LmRpZ2ljZXJ0
LmNvbS9EaWdpQ2VydEhpZ2hBc3N1cmFuY2VFVlJvb3RDQS5jcmwwHQYDVR0OBBYE
FExYyyXwQU9S9CjIgUObpqig5pLlMB8GA1UdIwQYMBaAFLE+w2kD+L9HAdSYJhoI
Au9jZCvDMA0GCSqGSIb3DQEBBQUAA4IBAQBMeheHKF0XvLIyc7/NLvVYMR3wsXFU
nNabZ5PbLwM+Fm8eA8lThKNWYB54lBuiqG+jpItSkdfdXJW777UWSemlQk808kf/
roF/E1S3IMRwFcuBCoHLdFfcnN8kpCkMGPAc5K4HM+zxST5Vz25PDVR708noFUjU
xbvcNRx3RQdIRYW9135TuMAW2ZXNi419yWBP0aKb49Aw1rRzNubS+QOy46T15bg+
BEkAui6mSnKDcp33C4ypieez12Qf1uNgywPE3IjpnSUBAHHLA7QpYCWP+UbRe3Gu
zVMSW4SOwg/H7ZMZ2cn6j1g0djIvruFQFGHUqFijyDATI+/GJYw2jxyA
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIDxTCCAq2gAwIBAgIQAqxcJmoLQJuPC3nyrkYldzANBgkqhkiG9w0BAQUFADBs
MQswCQYDVQQGEwJVUzEVMBMGA1UEChMMRGlnaUNlcnQgSW5jMRkwFwYDVQQLExB3
d3cuZGlnaWNlcnQuY29tMSswKQYDVQQDEyJEaWdpQ2VydCBIaWdoIEFzc3VyYW5j
ZSBFViBSb290IENBMB4XDTA2MTExMDAwMDAwMFoXDTMxMTExMDAwMDAwMFowbDEL
MAkGA1UEBhMCVVMxFTATBgNVBAoTDERpZ2lDZXJ0IEluYzEZMBcGA1UECxMQd3d3
LmRpZ2ljZXJ0LmNvbTErMCkGA1UEAxMiRGlnaUNlcnQgSGlnaCBBc3N1cmFuY2Ug
RVYgUm9vdCBDQTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAMbM5XPm
+9S75S0tMqbf5YE/yc0lSbZxKsPVlDRnogocsF9ppkCxxLeyj9CYpKlBWTrT3JTW
PNt0OKRKzE0lgvdKpVMSOO7zSW1xkX5jtqumX8OkhPhPYlG++MXs2ziS4wblCJEM
xChBVfvLWokVfnHoNb9Ncgk9vjo4UFt3MRuNs8ckRZqnrG0AFFoEt7oT61EKmEFB
Ik5lYYeBQVCmeVyJ3hlKV9Uu5l0cUyx+mM0aBhakaHPQNAQTXKFx01p8VdteZOE3
hzBWBOURtCmAEvF5OYiiAhF8J2a3iLd48soKqDirCmTCv2ZdlYTBoSUeh10aUAsg
EsxBu24LUTi4S8sCAwEAAaNjMGEwDgYDVR0PAQH/BAQDAgGGMA8GA1UdEwEB/wQF
MAMBAf8wHQYDVR0OBBYEFLE+w2kD+L9HAdSYJhoIAu9jZCvDMB8GA1UdIwQYMBaA
FLE+w2kD+L9HAdSYJhoIAu9jZCvDMA0GCSqGSIb3DQEBBQUAA4IBAQAcGgaX3Nec
nzyIZgYIVyHbIUf4KmeqvxgydkAQV8GK83rZEWWONfqe/EW1ntlMMUu4kehDLI6z
eM7b41N5cdblIZQB2lWHmiRk9opmzN6cN82oNLFpmyPInngiK3BD41VHMWEZ71jF
hS9OMPagMRYjyOfiZRYzy78aG6A9+MpeizGLYAiJLQwGXFK3xPkKmNEVX58Svnw2
Yzi9RKR/5CYrCsSXaQ3pjOLAEFe4yHYSkVXySGnYvCoCWw9E1CAx2/S6cCZdkGCe
vEsXCS+0yx5DaMkHJ8HSXPfqIbloEpw8nL+e/IBcm2PN7EeqJSdnoDfzAIJ9VNep
+OkuE6N36B9K
-----END CERTIFICATE-----`,
	"validCert": `-----BEGIN CERTIFICATE-----
MIIDEjCCAfqgAwIBAgIUWG+wobb7huLDKQ4bUn80QKsbVSQwDQYJKoZIhvcNAQEL
BQAwGDEWMBQGA1UEAwwNRXhhbXBsZVNlcnZlcjAeFw0yNDAyMjYyMjA5MjdaFw0y
OTAyMjQyMjA5MjdaMBgxFjAUBgNVBAMMDUV4YW1wbGVTZXJ2ZXIwggEiMA0GCSqG
SIb3DQEBAQUAA4IBDwAwggEKAoIBAQCsb9MSNERE7l4b/vxzkRnL6qvkjodbY3yQ
DDCbG29jJ71wSwYoz0v4n/Q/tDaJDBUcSjSPIa+L6Gn1BGLNUD2LFec5H5XlwcDq
A0D8qzvPLMYaIw4p04M/2Wvkqme79yQta8jXaCnPxTkgysno/FuQR+nDMqXoyW01
wMmj8JAYFs9MVPbmP97RqbVHnn7cydzvQvi4+dfBuII9L+TvTQY2o42bbCqCcmov
16m0uXp98j4CFfv/8KhCmQcqS0uyvp+J/iJZzEow+1NP72pozPEPv9JhXsYDb+vH
KxX5k44fYXVYVQHxYQllLjDDRcbz2DJh2DQXMZTiTLE63tC0m2RjAgMBAAGjVDBS
MDEGA1UdEQQqMCiCCGV4YW1wbGUxgghleGFtcGxlMoIIZXhhbXBsZTOCCGV4YW1w
bGU0MB0GA1UdDgQWBBTEkowqyNS1ez5XJ0shro/18UQlqTANBgkqhkiG9w0BAQsF
AAOCAQEANhnxV81NZmt8FcgrcRt73WdKDCE0hQbci65ZkAbcTVOCAs6Rw+aEC04L
0gKJT4Zmx3kR3fUwcDQRhjoXIrhmvP59k/8SN0N+Ua50oEYikaetsxEDr8oikwP2
eHDKU451jZZfcyII2Jx8XjgtiExcNZL3N9ZtSstp3I9n9BSmMdQHDIvPxNeGdJri
l4YGTm+sJLs+c5efSNaEPw1Dr6bhAPamNQwvIspuASJlCTcMCLw2+F3dEolAFUuM
E5wI9498c+P3pvl85UfiQISTu95jOl7YmZOYl8xBpu9yhQ7kjWhVRdVAnhl9Ts9F
3TD68rCMXegJYyg5VbiwVdSPkcw3eQ==
-----END CERTIFICATE-----`,
	"nonCert": `-----BEGIN CERTIFICATE-----
Oy1is0whDArLNwIDAQABo4G9MIG6
MA4GA1UdDwEB/wQEAwIE8DAdBgNVHQ4EFgQUbAavOY+vIXgc44k8GvLHH6mdzzYw
HwYDVR0jBBgwFoAUb+S3Mu7cbwZiqZaEHTEMyUhLyH4waAYDVR0RBGEwX4IFZW52
b3mCFGVudm95LnByb2plY3Rjb250b3VyghhlbnZveS5wcm9qZWN0Y29udG91ci5z
dmOCJmVudm95LnByb2plY3Rjb250b3VyLnN2Yy5jbHVzdGVyLmxvY2FsMA0GCSqG
SIb3DQEBCwUAA4IBAQBIKpBD1T9tugzJF7lajbdulXTb9qGibwQALqauskX9Sq57
po/R2TjyxywLn4DgM7BAzzu9qfHWf+S4eQjRUHQshPbUEX9CEsSd5tCu8ZHVbBds
6qFagl2+YQ9ng0Xwta9ezvctM3T6Dy9Kkf5OOe9ysMEsBX7s8NFxe68Qku+cExr3
78oERlIoNOlT0cNbFLAlH2svNv1uB4qOThRDha52L+mlUdZfTMYZAwNDJWm52t/M
NCIm5NJ5jAJpcJmoEb+JMP3j0x6wydHDXFtGm3WRggZRcrjasyodSKK6szbf96+9
6syzAwvg9xxNtFxwbhRqqplMEz2sDWaggTrxCQzd
-----END CERTIFICATE-----`,
}

// tests validate that the certParser function correctly parses a certificate
func TestCertParser(t *testing.T) {
	tests := []struct {
		name          string
		certChainName string
		Collectors    []troubleshootv1beta2.Collect
		want          []CertCollection
	}{
		{
			name:          "expired certificate",
			certChainName: "expiredCert",
			Collectors: []troubleshootv1beta2.Collect{
				{
					Certificates: &troubleshootv1beta2.Certificates{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Secrets: []troubleshootv1beta2.CertificateSource{
							{
								Name:       "expiredCert",
								Namespaces: []string{"test"},
							},
						},
					},
				},
			},
			want: []CertCollection{
				{
					Source: &CertificateSource{
						Namespace:  "test",
						SecretName: "expiredCert",
					},
					CertificateChain: []ParsedCertificate{
						{
							CertName:                "tls.crt",
							Subject:                 "O=Internet Widgits Pty Ltd,ST=Some-State,C=AU",
							SubjectAlternativeNames: nil,
							Issuer:                  "O=Internet Widgits Pty Ltd,ST=Some-State,C=AU",
							NotAfter:                time.Date(2015, time.September, 12, 21, 52, 2, 0, time.UTC),
							NotBefore:               time.Date(2012, time.September, 12, 21, 52, 2, 0, time.UTC),
							IsValid:                 false,
							IsCA:                    true,
						},
					},
				},
			},
		},
		{
			name:          "multiple certificate",
			certChainName: "multiCert",
			Collectors: []troubleshootv1beta2.Collect{
				{
					Certificates: &troubleshootv1beta2.Certificates{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Secrets: []troubleshootv1beta2.CertificateSource{
							{
								Name:       "multiCert",
								Namespaces: []string{"test"},
							},
						},
					},
				},
			},
			want: []CertCollection{
				{
					Source: &CertificateSource{
						Namespace:  "test",
						SecretName: "multiCert",
					},
					CertificateChain: []ParsedCertificate{
						{
							CertName:                "tls.crt",
							Subject:                 "CN=DigiCert High Assurance EV CA-1,OU=www.digicert.com,O=DigiCert Inc,C=US",
							SubjectAlternativeNames: nil,
							Issuer:                  "CN=DigiCert High Assurance EV Root CA,OU=www.digicert.com,O=DigiCert Inc,C=US",
							NotAfter:                time.Date(2021, time.November, 10, 0, 0, 0, 0, time.UTC),
							NotBefore:               time.Date(2007, time.November, 9, 12, 0, 0, 0, time.UTC),
							IsValid:                 false,
							IsCA:                    true,
						},
						{
							CertName:                "tls.crt",
							Subject:                 "CN=DigiCert High Assurance EV Root CA,OU=www.digicert.com,O=DigiCert Inc,C=US",
							SubjectAlternativeNames: nil,
							Issuer:                  "CN=DigiCert High Assurance EV Root CA,OU=www.digicert.com,O=DigiCert Inc,C=US",
							NotAfter:                time.Date(2031, time.November, 10, 0, 0, 0, 0, time.UTC), NotBefore: time.Date(2006, time.November, 10, 0, 0, 0, 0, time.UTC),
							IsValid: true,
							IsCA:    true,
						},
					},
				},
			},
		},
		{
			name:          "valid certificate",
			certChainName: "validCert",
			Collectors: []troubleshootv1beta2.Collect{
				{
					Certificates: &troubleshootv1beta2.Certificates{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Secrets: []troubleshootv1beta2.CertificateSource{
							{
								Name:       "validCert",
								Namespaces: []string{"test"},
							},
						},
					},
				},
			},
			want: []CertCollection{
				{
					Source: &CertificateSource{
						Namespace:  "test",
						SecretName: "validCert",
					},
					CertificateChain: []ParsedCertificate{
						{
							CertName: "tls.crt",
							Subject:  "CN=ExampleServer",
							SubjectAlternativeNames: []string{
								"example1",
								"example2",
								"example3",
								"example4",
							},
							Issuer:    "CN=ExampleServer",
							NotAfter:  time.Date(2029, time.February, 24, 22, 9, 27, 0, time.UTC),
							NotBefore: time.Date(2024, time.February, 26, 22, 9, 27, 0, time.UTC),
							IsValid:   true,
							IsCA:      false,
						},
					},
				},
			},
		},
		{
			name:          "non valid certificate",
			certChainName: "nonCert",
			Collectors: []troubleshootv1beta2.Collect{
				{
					Certificates: &troubleshootv1beta2.Certificates{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Secrets: []troubleshootv1beta2.CertificateSource{
							{
								Name:       "nonCert",
								Namespaces: []string{"test"},
							},
						},
					},
				},
			},
			want: []CertCollection{
				{
					Source: &CertificateSource{
						Namespace:  "test",
						SecretName: "nonCert",
					},
					Errors:           []string{"x509: malformed certificate"},
					CertificateChain: []ParsedCertificate{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := tt.Collectors[0].Certificates.Secrets[0].Namespaces[0]
			client := testclient.NewSimpleClientset()

			_, err := createTestSecret(client, tt.certChainName, ns)
			require.NoError(t, err)
			got := secretCertCollector(tt.certChainName, ns, client)
			assert.Equal(t, tt.want, got)
		})
	}
}

func createTestSecret(client kubernetes.Interface, secretName, ns string) (*corev1.Secret, error) {
	return client.CoreV1().Secrets(ns).Create(context.TODO(), &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: ns,
		}, Data: map[string][]byte{
			"tls.crt": []byte(certChains[secretName]),
		},
	}, metav1.CreateOptions{})
}
