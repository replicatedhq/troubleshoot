package collect

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

const CertMissing = "cert-missing"
const CertValid = "cert-valid"
const CertInvalid = "cert-invalid"

type CollectHostCertificatesCollection struct {
	hostCollector *troubleshootv1beta2.HostCertificatesCollection
	BundlePath    string
}

type HostCertificatesCollection struct {
	CertificatePath  string              `json:"certificatePath,omitempty"`
	CertificateChain []ParsedCertificate `json:"certificateChain,omitempty"`
	Message          string              `json:"message,omitempty"`
}

func (c *CollectHostCertificatesCollection) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "Host Certificates Collection")
}

func (c *CollectHostCertificatesCollection) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostCertificatesCollection) SkipRedaction() bool {
	return c.hostCollector.SkipRedaction
}

func (c *CollectHostCertificatesCollection) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	var results []HostCertificatesCollection

	for _, certPath := range c.hostCollector.Paths {
		results = append(results, HostCertsParser(certPath))
	}

	resultsJson, errResultJson := json.MarshalIndent(results, "", "\t")
	if errResultJson != nil {
		return nil, errResultJson
	}

	collectorName := c.hostCollector.CollectorName
	if collectorName == "" {
		collectorName = "certificatesCollection"
	}
	name := filepath.Join("host-collectors/certificatesCollection", collectorName+".json")

	output := NewResult()
	output.SaveResult(c.BundlePath, name, bytes.NewBuffer(resultsJson))

	return output, nil
}

func HostCertsParser(certPath string) HostCertificatesCollection {
	var certInfo []ParsedCertificate

	cert, err := ioutil.ReadFile(certPath)
	if err != nil {
		return HostCertificatesCollection{
			CertificatePath: certPath,
			Message:         CertMissing,
		}
	}

	certChain, _ := decodePem(cert)

	if len(certChain.Certificate) == 0 {
		return HostCertificatesCollection{
			CertificatePath: certPath,
			Message:         CertInvalid,
		}
	}

	for _, cert := range certChain.Certificate {
		parsedCert, errParse := x509.ParseCertificate(cert)
		if errParse != nil {
			return HostCertificatesCollection{
				CertificatePath: certPath,
				Message:         CertInvalid,
			}
		}
		currentTime := time.Now()
		certInfo = append(certInfo, ParsedCertificate{
			Subject:                 parsedCert.Subject.ToRDNSequence().String(),
			SubjectAlternativeNames: parsedCert.DNSNames,
			Issuer:                  parsedCert.Issuer.ToRDNSequence().String(),
			NotAfter:                parsedCert.NotAfter,
			NotBefore:               parsedCert.NotBefore,
			IsValid:                 currentTime.Before(parsedCert.NotAfter) && currentTime.After(parsedCert.NotBefore),
			IsCA:                    parsedCert.IsCA,
		})
	}

	return HostCertificatesCollection{
		CertificatePath:  certPath,
		CertificateChain: certInfo,
		Message:          CertValid,
	}
}

func (c *CollectHostCertificatesCollection) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}
