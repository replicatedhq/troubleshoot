package collect

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
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
	CertificateChain []ParsedCertificate `json:"certificateChain"`
	Errors           []error             `json:"errors"`
	Source           CertificateSource   `json:"source"`
}

type CertificateSource struct {
	SecretName    string `json:"secret,omitempty"`
	ConfigMapName string `json:"configMap,omitempty"`
	Namespace     string `json:"namespace,omitempty"`
}

// Certificate Struct
type ParsedCertificate struct {
	CertificateSource       CertificateSource `json:"source"`
	CertName                string            `json:"certificate"`
	Subject                 pkix.Name         `json:"subject"`
	SubjectAlternativeNames []string          `json:"subjectAlternativeNames"`
	Issuer                  string            `json:"issuer"`
	Organizations           []string          `json:"issuerOrganizations"`
	NotAfter                time.Time         `json:"notAfter"`
	NotBefore               time.Time         `json:"notBefore"`
	IsValid                 bool              `json:"isValid"`
	IsCA                    bool              `json:"isCA"`
}

func (c *CollectCertificates) Title() string {
	return getCollectorName(c)
}

func (c *CollectCertificates) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectCertificates) Collect(progressChan chan<- interface{}) (CollectorResult, error) {

	output := NewResult()

	// collect configmap certificate
	cm := configMapCertCollector(c.Collector.ConfigMaps, c.Client)

	// collect secret certificate
	secret := secretCertCollector(c.Collector.Secrets, c.Client)

	results := append(cm, secret...)

	filePath := "certificates/certificates.json"

	output.SaveResult(c.BundlePath, filePath, bytes.NewBuffer(results))

	return output, nil
}

// configmap certificate collector function
func configMapCertCollector(configMapName map[string]string, client kubernetes.Interface) []byte {

	//var trackErrors []error

	currentTime := time.Now()
	var certInfo []ParsedCertificate
	var certJson = []byte("[]")
	err := json.Unmarshal(certJson, &certInfo)
	if err != nil {
		log.Println(err)
	}

	for sourceName, namespace := range configMapName {

		listOptions := metav1.ListOptions{}

		configMaps, _ := client.CoreV1().ConfigMaps(namespace).List(context.Background(), listOptions)

		for _, configMap := range configMaps.Items {
			if sourceName == configMap.Name {

				for certName, certs := range configMap.Data {
					data := string(certs)

					if strings.Contains(data, "BEGIN CERTIFICATE") && strings.Contains(data, "END CERTIFICATE") {

						certChain := decodePem(data)

						for _, cert := range certChain.Certificate {

							//parsed SSL certificate
							parsedCert, errParse := x509.ParseCertificate(cert)
							if errParse != nil {
								log.Println(errParse)
							}

							certInfo = append(certInfo, ParsedCertificate{
								CertificateSource: CertificateSource{
									ConfigMapName: configMap.Name,
									Namespace:     configMap.Namespace,
								},
								CertName:                certName,
								Subject:                 parsedCert.Subject, //TODO
								SubjectAlternativeNames: parsedCert.DNSNames,
								Issuer:                  parsedCert.Issuer.CommonName,
								Organizations:           parsedCert.Issuer.Organization,
								NotAfter:                parsedCert.NotAfter,
								NotBefore:               parsedCert.NotBefore,
								IsValid:                 currentTime.Before(parsedCert.NotAfter),
								IsCA:                    parsedCert.IsCA,
							})
							certJson, _ = json.MarshalIndent(certInfo, "", "\t")
						}

					}
				}
			}
		}
	}

	return certJson
}

// secret certificate collector function
func secretCertCollector(secretName map[string]string, client kubernetes.Interface) []byte {
	//var trackErrors []error

	currentTime := time.Now()
	var certInfo []ParsedCertificate
	var certJson = []byte("[]")
	err := json.Unmarshal(certJson, &certInfo)
	if err != nil {
		log.Println(err)
	}

	for sourceName, namespace := range secretName {

		listOptions := metav1.ListOptions{}
		secrets, _ := client.CoreV1().Secrets(namespace).List(context.Background(), listOptions)

		for _, secret := range secrets.Items {
			if sourceName == secret.Name {

				for certName, certs := range secret.Data {

					data := string(certs)

					if strings.Contains(data, "BEGIN CERTIFICATE") && strings.Contains(data, "END CERTIFICATE") {

						certChain := decodePem(data)

						for _, cert := range certChain.Certificate {

							//parsed SSL certificate
							parsedCert, errParse := x509.ParseCertificate(cert)
							if errParse != nil {
								fmt.Println("failed to parse certificate: %v", errParse.Error())
								continue
							}

							certInfo = append(certInfo, ParsedCertificate{
								CertificateSource: CertificateSource{
									SecretName: secret.Name,
									Namespace:  secret.Namespace,
								},
								CertName:                certName,
								Subject:                 parsedCert.Subject,
								SubjectAlternativeNames: parsedCert.DNSNames,
								Issuer:                  parsedCert.Issuer.CommonName,
								Organizations:           parsedCert.Issuer.Organization,
								NotAfter:                parsedCert.NotAfter,
								NotBefore:               parsedCert.NotBefore,
								IsValid:                 currentTime.Before(parsedCert.NotAfter),
								IsCA:                    parsedCert.IsCA,
							})
							certJson, _ = json.MarshalIndent(certInfo, "", "\t")
						}
					}
				}

			}
		}
	}
	return certJson
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
