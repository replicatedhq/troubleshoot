package collect

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"log"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type CollectCertificates struct {
	Collector    *troubleshootv1beta2.Certificates
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

// Collect source information - where certificate came from.
type CertCollection struct {
	Source           *CertificateSource  `json:"source"`
	Errors           []string            `json:"errors"`
	CertificateChain []ParsedCertificate `json:"certificateChain"`
}

type CertificateSource struct {
	SecretName    string `json:"secret,omitempty"`
	ConfigMapName string `json:"configMap,omitempty"`
	Namespace     string `json:"namespace,omitempty"`
}

// Certificate Struct
type ParsedCertificate struct {
	CertName                string    `json:"certificate"`
	Subject                 string    `json:"subject"`
	SubjectAlternativeNames []string  `json:"subjectAlternativeNames"`
	Issuer                  string    `json:"issuer"`
	NotAfter                time.Time `json:"notAfter"`
	NotBefore               time.Time `json:"notBefore"`
	IsValid                 bool      `json:"isValid"`
	IsCA                    bool      `json:"isCA"`
}

func (c *CollectCertificates) Title() string {
	return getCollectorName(c)
}

func (c *CollectCertificates) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectCertificates) Collect(progressChan chan<- interface{}) CollectorResult {

	output := NewResult()
	results := []CertCollection{}

	// collect secret certificate
	for secretName, namespace := range c.Collector.Secrets {
		secretCollections := secretCertCollector(secretName, namespace, c.Client)
		results = append(results, secretCollections...) // Explode the slice
	}

	certsJson, _ := json.MarshalIndent(results, "", "\t")

	filePath := "certificates/certificates.json"

	output.SaveResult(c.BundlePath, filePath, bytes.NewBuffer(certsJson))

	return output
}

// configmap certificate collector function
/*
func configMapCertCollector(configMapName string, namespace string, client kubernetes.Interface) CertCollection {

}
*/

// secret certificate collector function
// func secretCertCollector(secretName map[string]string, client kubernetes.Interface) CertCollection {
func secretCertCollector(secretName string, namespace string, client kubernetes.Interface) []CertCollection {

	results := []CertCollection{}

	// Collect from secrets
	listOptions := metav1.ListOptions{}
	// TODO: Handle RBAC errors. Not to be worked on yet
	secrets, _ := client.CoreV1().Secrets(namespace).List(context.Background(), listOptions)

	for _, secret := range secrets.Items {
		// Collect from secret
		source := &CertificateSource{
			SecretName: secret.Name,
			Namespace:  secret.Namespace,
		}
		log.Println("secret items: ", secrets.Items)

		trackErrors := []string{}

		for certName, certs := range secret.Data {
			certInfo, _ := CertParser(certName, certs)

			results = append(results, CertCollection{
				Source:           source,
				Errors:           trackErrors,
				CertificateChain: certInfo,
			})
			log.Println("myresults:", results)
		}

	}
	return results
}

func decodePem(certInput string) tls.Certificate {
	var cert tls.Certificate
	certPEMBlock := []byte(certInput)
	var certDERBlock *pem.Block
	for {
		certDERBlock, certPEMBlock = pem.Decode(certPEMBlock)
		if certDERBlock == nil {
			break
		}
		if certDERBlock.Type == "CERTIFICATE" {
			cert.Certificate = append(cert.Certificate, certDERBlock.Bytes)
		}
	}
	return cert
}

//for certName, certs := range secret.Data {

func CertParser(certName string, certs []byte) ([]ParsedCertificate, []string) {
	currentTime := time.Now()
	var trackErrors []string
	data := string(certs)
	var certificateChainCollection []ParsedCertificate

	certChain := decodePem(data)

	for _, cert := range certChain.Certificate {

		//parsed SSL certificate
		parsedCert, errParse := x509.ParseCertificate(cert)
		if errParse != nil {
			trackErrors = append(trackErrors, "error: This object is not a certificate")
			continue // End here, start parsing the next cert in the for loop
		}

		certInfo := ParsedCertificate{
			CertName:                certName,
			Subject:                 parsedCert.Subject.ToRDNSequence().String(),
			SubjectAlternativeNames: parsedCert.DNSNames,
			Issuer:                  parsedCert.Issuer.CommonName,
			NotAfter:                parsedCert.NotAfter,
			NotBefore:               parsedCert.NotBefore,
			IsValid:                 currentTime.Before(parsedCert.NotAfter),
			IsCA:                    parsedCert.IsCA,
		}
		certificateChainCollection = append(certificateChainCollection, certInfo)
	}
	return certificateChainCollection, trackErrors
}
