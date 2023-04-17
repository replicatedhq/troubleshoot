package collect

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var expiredCertChain = `-----BEGIN CERTIFICATE-----
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
-----END CERTIFICATE-----`

var multiCertChain = `-----BEGIN CERTIFICATE-----
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
-----END CERTIFICATE-----`

var validCertChain = `-----BEGIN CERTIFICATE-----
MIIDejCCAmKgAwIBAgIEAZaq0DANBgkqhkiG9w0BAQsFADAuMRgwFgYDVQQDEw9Q
cm9qZWN0IENvbnRvdXIxEjAQBgNVBAUTCTYxNTkyOTg5MTAeFw0yMzAyMjQwNDI3
MThaFw0yNDAyMjUwNDI3MTZaMBAxDjAMBgNVBAMTBWVudm95MIIBIjANBgkqhkiG
9w0BAQEFAAOCAQ8AMIIBCgKCAQEAqsNmmxb1ICso6Ay25lapcRyvLxAX/5u422uV
eiNn5jseCVXfg1jJr2Symrgou2dgMtIpZoVKT7w0el8sNpD+az5oMOWUNfTGEpYI
5zNAhtiedxCWcX15gfezOZ/DECL7HP8U37JFVdazm0CjvlQWI+8rFGvFQJJFDLYJ
h0fHGfbX6L/ST6cANtkXZyIU6CYFSgniuuDjHmQnQr6CC8lkisJxY5QVS7MZ02RR
nU/dK14ABY+mo/0ZeBKR5si04hr4i18nJJnk4DnHN+jQ/WWWSO1yLqr9kAOj37dA
nRAeKuzkx/VbU8DC/3sh0otcazoWO470D+irOy1is0whDArLNwIDAQABo4G9MIG6
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
-----END CERTIFICATE-----
`

var nonCertChain = `-----BEGIN CERTIFICATE-----
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
-----END CERTIFICATE-----
`

// tests validate that the certParser function correctly parses a certificate
func TestCertParser(t *testing.T) {
	type parsedCertResult struct {
		Name      string
		CertQty   int
		IsSubject bool
		IsValid   []bool
	}

	tests := []struct {
		name        string
		certName    string
		certificate []byte
		expiredTime time.Time
		want        parsedCertResult
	}{
		{
			name:        "expired certificate",
			certName:    "Widgits",
			certificate: []byte(expiredCertChain),
			expiredTime: time.Now(),
			want: parsedCertResult{
				CertQty:   1,
				IsSubject: true,
				IsValid:   []bool{false},
			},
		},
		{
			name:        "multi certificate",
			certName:    "digicert",
			certificate: []byte(multiCertChain),
			expiredTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			want: parsedCertResult{
				CertQty:   2,
				IsSubject: true,
				IsValid:   []bool{false, true},
			},
		},
		{
			name:        "valid certificate",
			certName:    "envoy",
			certificate: []byte(validCertChain),
			expiredTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			want: parsedCertResult{
				CertQty:   1,
				IsSubject: true,
				IsValid:   []bool{true},
			},
		},
		{
			name:        "non certificate",
			certName:    "non.crt",
			certificate: []byte(nonCertChain),
			expiredTime: time.Now(),
			want: parsedCertResult{
				CertQty:   0,
				IsSubject: false,
				IsValid:   []bool{true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			certs, _ := CertParser(tt.certName, tt.certificate, tt.expiredTime)
			assert.Equal(t, tt.want.CertQty, len(certs), "Expected %v, but got %v", tt.want.CertQty, len(certs))

			for idx, cert := range certs {
				if !strings.Contains(cert.Subject, tt.certName) {
					isSubject := false
					assert.Equal(t, tt.want.IsSubject, isSubject, "Expected %v, but got %v", tt.want.IsSubject, isSubject)
				}
				assert.Equal(t, tt.want.IsValid[idx], cert.IsValid, "Expected %v, but got %v", tt.want.IsValid[idx], cert.IsValid)
			}
		})
	}
}

// validates that certificate count is correct when parsing a certificate input string.
func Test_decodePem(t *testing.T) {
	tests := []struct {
		name        string
		certificate []byte
		wantQty     int
	}{
		{
			name:        "expired certificate",
			certificate: []byte(expiredCertChain),
			wantQty:     1,
		},
		{
			name:        "multi certificate",
			certificate: []byte(multiCertChain),
			wantQty:     2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert, _ := decodePem(tt.certificate)
			assert.Equal(t, tt.wantQty, len(cert.Certificate), "Expected %v, but got %v", tt.wantQty, len(cert.Certificate))
		})
	}
}
