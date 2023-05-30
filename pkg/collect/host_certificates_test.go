package collect

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/replicatedhq/troubleshoot/internal/testutils"
	"github.com/stretchr/testify/assert"
)

func Test_HostCertParser(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.txt")
	tests := []struct {
		name                string
		filePath, certChain string
		want                HostCertsCollection
	}{
		{
			name:      "valid certificate",
			filePath:  path,
			certChain: certChains["validCert"],
			want: HostCertsCollection{
				CertificatePath: path,
				CertificateChain: []ParsedCertificate{
					{
						Subject: "CN=envoy",
						SubjectAlternativeNames: []string{
							"envoy",
							"envoy.projectcontour",
							"envoy.projectcontour.svc",
							"envoy.projectcontour.svc.cluster.local",
						},
						Issuer:    "SERIALNUMBER=615929891,CN=Project Contour",
						NotAfter:  time.Date(2024, time.February, 25, 4, 27, 16, 0, time.UTC),
						NotBefore: time.Date(2023, time.February, 24, 4, 27, 18, 0, time.UTC),
						IsValid:   true,
						IsCA:      false,
					},
				},
				Message: "cert-valid",
			},
		},
		{
			name:      "expired certificate",
			filePath:  path,
			certChain: certChains["expiredCert"],
			want: HostCertsCollection{
				CertificatePath: path,
				CertificateChain: []ParsedCertificate{
					{
						Subject:                 "O=Internet Widgits Pty Ltd,ST=Some-State,C=AU",
						SubjectAlternativeNames: nil,
						Issuer:                  "O=Internet Widgits Pty Ltd,ST=Some-State,C=AU",
						NotAfter:                time.Date(2015, time.September, 12, 21, 52, 2, 0, time.UTC),
						NotBefore:               time.Date(2012, time.September, 12, 21, 52, 2, 0, time.UTC),
						IsValid:                 false,
						IsCA:                    true,
					},
				},
				Message: "cert-valid",
			},
		},
		{
			name:      "missing certificate",
			filePath:  "",
			certChain: "",
			want: HostCertsCollection{
				CertificatePath: "",
				Message:         "cert-missing",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutils.CreateTestFileWithData(t, path, tt.certChain)
			got := HostCertsParser(tt.filePath)
			assert.Equal(t, tt.want, got)
		})
	}
}
