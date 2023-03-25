package collect

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"log"
	"strings"
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

func (c *CollectCertificates) Collect(progressChan chan<- interface{}) (CollectorResult, error) {

	output := NewResult()
	results := []CertCollection{}

	// collect secret certificate
	for secretName, namespace := range c.Collector.Secrets {
		secretCollections := secretCertCollector(secretName, namespace, c.Client)
		results = append(results, secretCollections) // Explode the slice

	}

	certsJson, _ := json.MarshalIndent(results, "", "\t")

	filePath := "certificates/certificates.json"

	output.SaveResult(c.BundlePath, filePath, bytes.NewBuffer(certsJson))

	return output, nil
}

// configmap certificate collector function
func configMapCertCollector(configMapName string, namespace string, client kubernetes.Interface) CertCollection {
	currentTime := time.Now()
	var certInfo []ParsedCertificate
	var trackErrors []string
	var source = &CertificateSource{}

	listOptions := metav1.ListOptions{}

	configMaps, _ := client.CoreV1().ConfigMaps(namespace).List(context.Background(), listOptions)

	for _, configMap := range configMaps.Items {
		if configMapName == configMap.Name {

			for certName, certs := range configMap.Data {
				data := string(certs)

				if strings.Contains(data, "BEGIN CERTIFICATE") && strings.Contains(data, "END CERTIFICATE") {

					source = &CertificateSource{
						ConfigMapName: configMap.Name,
						Namespace:     configMap.Namespace,
					}

					certChain := decodePem(data)

					for _, cert := range certChain.Certificate {

						//parsed SSL certificate
						parsedCert, errParse := x509.ParseCertificate(cert)
						if errParse != nil {

							trackErrors = append(trackErrors, errParse.Error())
						}

						// Subject example: CN=DigiCert High Assurance EV CA-1,OU=www.digicert.com,O=DigiCert Inc,C=US
						// Issuer example: CN=DigiCert High Assurance EV Root CA,OU=www.digicert.com,O=DigiCert Inc,C=US
						certInfo = append(certInfo, ParsedCertificate{
							CertName:                certName,
							Subject:                 parsedCert.Subject.ToRDNSequence().String(),
							SubjectAlternativeNames: parsedCert.DNSNames,
							Issuer:                  parsedCert.Issuer.ToRDNSequence().String(),
							NotAfter:                parsedCert.NotAfter,
							NotBefore:               parsedCert.NotBefore,
							IsValid:                 currentTime.Before(parsedCert.NotAfter),
							IsCA:                    parsedCert.IsCA,
						})

					}
				} else {
					trackErrors = append(trackErrors, "error: This object is not a certificate")

				}
			}

		}
	}
	return CertCollection{
		Source:           source,
		Errors:           trackErrors,
		CertificateChain: certInfo,
	}
}

// secret certificate collector function
// func secretCertCollector(secretName map[string]string, client kubernetes.Interface) CertCollection {
func secretCertCollector(secretName string, namespace string, client kubernetes.Interface) CertCollection {

	results := CertCollection{}

	// Collect from secrets
	listOptions := metav1.ListOptions{}
	// TODO: Handle RBAC errors. Not to be worked on yet
	secrets, _ := client.CoreV1().Secrets(namespace).List(context.Background(), listOptions)
	trackErrors := []string{}
	certInfo := []ParsedCertificate{}

	for _, secret := range secrets.Items {
		if secretName == secret.Name {
			// Collect from secret
			source := &CertificateSource{
				SecretName: secret.Name,
				Namespace:  secret.Namespace,
			}

			for certName, certs := range secret.Data {

				//certInfo := CertParser(certName, certs, certCollection)
				collection := CertParser(certName, certs, certInfo, source, trackErrors)

				log.Println("coolection: ", collection)

			}
		}

	}
	//log.Println("my results: ", results)
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

// func CertParser(certName string, certs []byte) ([]ParsedCertificate, []string) {
func CertParser(certName string, certs []byte, parsedCerts []ParsedCertificate, source *CertificateSource, trackErrors []string) CertCollection {
	currentTime := time.Now()
	data := string(certs)
	//certInfo := ParsedCertificate{}
	results := CertCollection{}

	certChain := decodePem(data)

	if strings.Contains(data, "BEGIN CERTIFICATE") && strings.Contains(data, "END CERTIFICATE") {

		for _, cert := range certChain.Certificate {

			//parsed SSL certificate
			parsedCert, errParse := x509.ParseCertificate(cert)
			if errParse != nil {
				trackErrors = append(trackErrors, "error: This object is not a certificate")
				continue // End here, start parsing the next cert in the for loop
			}

			parsedCerts := append(parsedCerts, ParsedCertificate{
				CertName:                certName,
				Subject:                 parsedCert.Subject.ToRDNSequence().String(),
				SubjectAlternativeNames: parsedCert.DNSNames,
				Issuer:                  parsedCert.Issuer.CommonName,
				NotAfter:                parsedCert.NotAfter,
				NotBefore:               parsedCert.NotBefore,
				IsValid:                 currentTime.Before(parsedCert.NotAfter),
				IsCA:                    parsedCert.IsCA,
			})

			//log.Println("certCollect-final: ", *certInfo)
			//certCollection = append(certCollection, certInfo...)

			results = CertCollection{
				Source:           source,
				Errors:           trackErrors,
				CertificateChain: parsedCerts,
			}

		}
		//log.Println("dagr: ", certInfo)

	}

	return results
}
