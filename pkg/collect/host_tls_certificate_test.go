package collect

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/analyze/types"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectHostTLS_Collect(t *testing.T) {
	// Create a self-signed certificate for testing
	cert, key, err := generateTestSelfSignedCert()
	require.NoError(t, err)

	// Start a test TLSCertificate server
	serverAddr, closeServer, err := startTestHttpsServer(cert, key)
	require.NoError(t, err)
	defer closeServer()

	// Create a temporary directory for the bundle
	bundlePath, err := os.MkdirTemp("", "tls-test")
	require.NoError(t, err)
	defer os.RemoveAll(bundlePath)

	// Create the necessary subdirectories
	hostTimePath := filepath.Join(bundlePath, "host-collectors", "time")
	err = os.MkdirAll(hostTimePath, 0755)
	require.NoError(t, err)

	type certFields struct {
		Issuer  string `json:"issuer"`
		Subject string `json:"subject"`
		IsCA    bool   `json:"is_ca"`
	}

	tests := []struct {
		name          string
		hostCollector *troubleshootv1beta2.HostTLSCertificate
		certFields    []certFields
		expectedCerts []certFields
		wantErr       bool
	}{
		{
			name: "successful collection",
			hostCollector: &troubleshootv1beta2.HostTLSCertificate{
				Address: serverAddr,
				HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
					CollectorName: "test-tls",
				},
			},
			certFields: []certFields{
				{
					Issuer:  "localhost",
					Subject: "localhost",
					IsCA:    false,
				},
			},
			wantErr: false,
		},
		{
			name: "failed connection",
			hostCollector: &troubleshootv1beta2.HostTLSCertificate{
				Address: "invalid-address:9999",
				HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
					CollectorName: "test-tls-failed",
				},
			},
			wantErr: true,
		},
		{
			name: "successful collection with expected cert",
			hostCollector: &troubleshootv1beta2.HostTLSCertificate{
				Address:             serverAddr,
				ExpectedCertSubpath: "expected-cert",
				HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
					CollectorName: "test-tls",
				},
			},
			certFields: []certFields{
				{
					Issuer:  "localhost",
					Subject: "localhost",
					IsCA:    false,
				},
			},
			expectedCerts: []certFields{
				{
					Issuer:  "expected",
					Subject: serverAddr,
					IsCA:    true,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CollectHostTLSCertificate{
				hostCollector: tt.hostCollector,
				BundlePath:    bundlePath,
			}

			collected, err := c.Collect(nil)
			require.NoError(t, err)
			require.NotNil(t, collected)

			expectedFilename := filepath.Join("host-collectors/tls-certificate", tt.hostCollector.CollectorName+".json")
			assert.Contains(t, collected, expectedFilename)

			// Validate the content
			var tlsInfo types.TLSInfo
			err = json.Unmarshal(collected[expectedFilename], &tlsInfo)

			require.NoError(t, err)

			if tt.wantErr {
				require.NotNil(t, tlsInfo.Error)
				return
			}
			require.Equal(t, "", tlsInfo.Error)

			// Verify we have certificate information
			require.NotEmpty(t, tlsInfo.PeerCertificates)

			// Verify the certificate fields match the expected values
			require.Equal(t, len(tt.certFields), len(tlsInfo.PeerCertificates))
			for i, crt := range tlsInfo.PeerCertificates {
				assert.Equal(t, tt.certFields[i].Issuer, crt.Issuer)
				assert.Equal(t, tt.certFields[i].Subject, crt.Subject)
				assert.Equal(t, tt.certFields[i].IsCA, crt.IsCA)
			}

			// verify that the expected certs array (returned by the server) matches the expected certs array
			require.Equal(t, len(tt.expectedCerts), len(tlsInfo.ExpectedCerts))
			for i, crt := range tlsInfo.ExpectedCerts {
				assert.Equal(t, tt.expectedCerts[i].Issuer, crt.Issuer)
				assert.Equal(t, tt.expectedCerts[i].Subject, crt.Subject)
				assert.Equal(t, tt.expectedCerts[i].IsCA, crt.IsCA)
			}
		})
	}
}

// Helper function to generate a self-signed certificate for testing
func generateTestSelfSignedCert() ([]byte, []byte, error) {
	// Generate a new private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to generate private key")
	}

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to generate serial number")
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		Issuer: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
	}

	// Create certificate using the template
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create certificate")
	}

	// Convert private key to DER format
	privateKeyDER := x509.MarshalPKCS1PrivateKey(privateKey)

	return derBytes, privateKeyDER, nil
}

// Helper function to start a test TLSCertificate server
func startTestHttpsServer(certDER, keyDER []byte) (string, func(), error) {
	// Encode certificate and key in PEM format
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyDER})

	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return "", nil, errors.Wrap(err, "failed to create X509 key pair")
	}

	// Create a simple HTTP handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println(r.URL.Path)
		if r.URL.Path == "/expected-cert" {
			w.WriteHeader(http.StatusOK)
			subject := r.Header.Get("tls-request-hostname")
			resp := fmt.Sprintf(`[{"issuer": "expected", "subject": "%s", "serial": "1234567890", "not_before": "abc", "not_after": "xyz", "is_ca": true}]`, subject)
			w.Write([]byte(resp))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("TLSCertificate Test Server"))
	})

	// Use httptest package to create a TLSCertificate server
	testServer := httptest.NewUnstartedServer(handler)
	testServer.TLS = &tls.Config{
		Certificates: []tls.Certificate{pair},
	}
	testServer.StartTLS()

	addr := strings.TrimPrefix(testServer.URL, "https://")

	return addr, testServer.Close, nil
}
