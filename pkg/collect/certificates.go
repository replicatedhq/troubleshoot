package collect

import (
	"bytes"
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/pkg/errors"
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
type CertificateSource struct {
	SecretName    string  `json:"secret,omitempty"`
	ConfigMapName string  `json:"configMap,omitempty"`
	Namespace     string  `json:"namespace,omitempty"`
	Errors        []error `json:"errors,omitempty"`
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
func configMapCertCollector(configMapSources map[string]string, client kubernetes.Interface) []byte {

	var trackErrors []error

	currentTime := time.Now()
	var certInfo []ParsedCertificate
	var certJson = []byte("[]")
	err := json.Unmarshal(certJson, &certInfo)
	if err != nil {
		log.Println(err)
	}

	for sourceName, namespace := range configMapSources {

		listOptions := metav1.ListOptions{}

		configMaps, _ := client.CoreV1().ConfigMaps(namespace).List(context.Background(), listOptions)

		for _, configMap := range configMaps.Items {
			if sourceName == configMap.Name {

				for certName, cert := range configMap.Data {
					//if certName[len(certName)-3:] == "crt" {

					data := string(cert)
					var block *pem.Block
					if strings.Contains(data, "BEGIN CERTIFICATE") && strings.Contains(data, "END CERTIFICATE") {

						block, _ = pem.Decode([]byte(data))

						//parsed SSL certificate
						parsedCert, errParse := x509.ParseCertificate(block.Bytes)
						if errParse != nil {
							log.Println(errParse)
						}

						if parsedCert.Issuer.CommonName != "" { //TODO: take this out

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
						} else {

							err := errors.New(("error: This object is not a certificate"))
							trackErrors = append(trackErrors, err)

						}
					}
				}
			}
		}
	}

	return certJson
}

// secret certificate collector function
func secretCertCollector(secretSources map[string]string, client kubernetes.Interface) []byte {
	var trackErrors []error

	currentTime := time.Now()
	var certInfo []ParsedCertificate
	var certJson = []byte("[]")
	err := json.Unmarshal(certJson, &certInfo)
	if err != nil {
		log.Println(err)
	}

	for sourceName, namespace := range secretSources {

		listOptions := metav1.ListOptions{}
		secrets, _ := client.CoreV1().Secrets(namespace).List(context.Background(), listOptions)

		for _, secret := range secrets.Items {
			if sourceName == secret.Name {

				for certName, cert := range secret.Data {
					//if certName[len(certName)-3:] == "crt" {

					data := string(cert)
					var block *pem.Block
					if strings.Contains(data, "BEGIN CERTIFICATE") && strings.Contains(data, "END CERTIFICATE") {

						block, _ = pem.Decode([]byte(data))

						//parsed SSL certificate
						parsedCert, errParse := x509.ParseCertificate(block.Bytes)
						if errParse != nil {
							fmt.Println("failed to parse certificate: %v", errParse.Error())
							return nil
						}

						//Just note for myself; will clean up in final version
						//x509.VerifyOptions()
						//x509.HostnameError
						/*
							opts := x509.VerifyOptions{
								DNSName: "deepsource.io",
								Roots:   roots,
							}
						*/

						certInfo = append(certInfo, ParsedCertificate{
							CertificateSource: CertificateSource{
								SecretName: secret.Name,
								Namespace:  secret.Namespace,
								Errors:     trackErrors,
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
					} else {

						err := errors.New(("error: This object is not a certificate"))
						trackErrors = append(trackErrors, err)

					}

				}
			}
		}
	}
	return certJson
}
