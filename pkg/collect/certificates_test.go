package collect

import (
	"log"
	"strings"
	"testing"
)

var chain = `-----BEGIN CERTIFICATE-----
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
-----END CERTIFICATE-----
`

var chain2 = `-----BEGIN CERTIFICATE-----
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

var chain3 = `-----BEGIN CERTIFICATE-----
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

var chain4 = `-----BEGIN CERTIFICATE-----
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

	expiredCert := []byte(chain)
	multiCert := []byte(chain2)
	validCert := []byte(chain3)
	nonCert := []byte(chain4)
	//add docs

	certParserTests := []struct {
		Name      string
		Certs     []byte
		CertQty   int
		IsSubject bool
		IsCert    bool
		IsValid   bool
	}{
		// Name, Certs, CertQty, IsSubject, IsCert, IsValid
		{"Widgits", expiredCert, 1, true, true, false},
		{"digicert", multiCert, 2, true, true, false}, //IsValid will not be evaluated for multicerts
		{"envoy", validCert, 1, true, true, true},
		{"non.crt", nonCert, 1, false, false, false}, // IsSubject and IsValid n/a for non.crt
	}

	for _, e := range certParserTests {

		results, _ := CertParser(e.Name, e.Certs)

		// tests if results contains a list of parsed certificates
		var isCert bool
		if len(results) == 0 {
			isCert = false
		} else {
			isCert = true
		}

		if isCert != e.IsCert {
			t.Errorf("Expected %v, but got %v that %s is a certificate", e.IsCert, isCert, e.Name)
			continue
		}

		for _, cert := range results {

			log.Println(e.Name, cert.Subject)

			// test checks if certificate subject contains a matching string; validates that can parse cert and pull back information
			if !strings.Contains(cert.Subject, e.Name) {
				t.Error("unable to parse certificate: ", e.Name)

			} else {
				isSubject := true
				if e.IsSubject != isSubject {
					t.Errorf("You expected that %s certificate contains %s in the Subject line but it was not present; here is the entire subject string: %v", e.Name, e.Name, cert.Subject)
				}
			}

			// test checks for expected quantity of certificates in a certificate file
			numCerts := len(results)
			if numCerts != e.CertQty {
				t.Errorf("expected %d certificates in slice but got %d", e.CertQty, numCerts)
			}
			if numCerts > 1 {
				continue
			}

			// test checks for expected certificate expired
			if cert.IsValid != e.IsValid {
				t.Errorf("When checing that %v certificate is expired, you expected %v, but got %v ", e.Name, e.IsValid, cert.IsValid)
			}

		}

	}

}

// validates that certificate count is correct when parsing a certificate input string.
func Test_decodePem(t *testing.T) {
	certDecoderTests := []struct {
		Name      string
		certInput string
		certCount int
	}{
		{"widgets-cert", chain, 1},
		{"digi-cert", chain2, 2},
	}
	for _, e := range certDecoderTests {
		results, _ := decodePem(e.certInput)
		certCount := 0

		for _, cert := range results.Certificate {
			log.Println(cert)
			certCount++
		}
		if certCount != e.certCount {
			t.Errorf("cert count -- expected %d, but got %d", e.certCount, certCount)
		}

	}
}
