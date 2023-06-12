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

type CollectHostCertificateValidity struct {
	hostCollector *troubleshootv1beta2.HostCertificateValidity
	BundlePath    string
}

type HostCertificateValidityCollection struct {
	CertificatePath  string              `json:"certificatePath,omitempty"`
	CertificateChain []ParsedCertificate `json:"certificateChain,omitempty"`
	Message          string              `json:"message,omitempty"`
}

func (c *CollectHostCertificateValidity) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "Host Certificate Validity")
}

func (c *CollectHostCertificateValidity) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostCertificateValidity) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	var results []HostCertificateValidityCollection

	for _, certPath := range c.hostCollector.Paths {
		results = append(results, HostCertsParser(certPath))
	}

	resultsJson, errResultJson := json.MarshalIndent(results, "", "\t")
	if errResultJson != nil {
		return nil, errResultJson
	}

	collectorName := c.hostCollector.CollectorName
	if collectorName == "" {
		collectorName = "certificateValidity"
	}
	name := filepath.Join("host-collectors/certificateValidity", collectorName+".json")

	output := NewResult()
	output.SaveResult(c.BundlePath, name, bytes.NewBuffer(resultsJson))

	return output, nil
}

func HostCertsParser(certPath string) HostCertificateValidityCollection {
	var certInfo []ParsedCertificate

	cert, err := ioutil.ReadFile(certPath)
	if err != nil {
		return HostCertificateValidityCollection{
			CertificatePath: certPath,
			Message:         CertMissing,
		}
	}

	certChain, _ := decodePem(cert)

	if len(certChain.Certificate) == 0 {
		return HostCertificateValidityCollection{
			CertificatePath: certPath,
			Message:         CertInvalid,
		}
	}

	for _, cert := range certChain.Certificate {
		parsedCert, errParse := x509.ParseCertificate(cert)
		if errParse != nil {
			return HostCertificateValidityCollection{
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

	return HostCertificateValidityCollection{
		CertificatePath:  certPath,
		CertificateChain: certInfo,
		Message:          CertValid,
	}
}
