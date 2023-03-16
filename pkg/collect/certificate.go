package collect

import (
	"bytes"
	"context"
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

type CollectInclusterCertificate struct {
	Collector    *troubleshootv1beta2.InclusterCertificate
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
	Subject                 string            `json:"subject"`
	SubjectAlternativeNames []string          `json:"subjectAlternativeNames"`
	Issuer                  string            `json:"issuer"`
	Organizations           []string          `json:"issuerOrganizations"`
	NotAfter                time.Time         `json:"notAfter"`
	NotBefore               time.Time         `json:"notBefore"`
	IsValid                 bool              `json:"isValid"`
	IsCA                    bool              `json:"isCA"`
}

func (c *CollectInclusterCertificate) Title() string {
	return getCollectorName(c)
}

func (c *CollectInclusterCertificate) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectInclusterCertificate) Collect(progressChan chan<- interface{}) (CollectorResult, error) {

	output := NewResult()

	// collect configmap certificate
	cm := configMapCertCollector(c.Collector.ConfigMapSources, c.Client)

	// collect secret certificate
	secret := secretCertCollector(c.Collector.SecretSources, c.Client)

	results := append(cm, secret...)

	filePath := "certificates/certificates.json"

	output.SaveResult(c.BundlePath, filePath, bytes.NewBuffer(results))

	return output, nil
}

// configmap certificate collector function
func configMapCertCollector(configMapSources map[string]string, client kubernetes.Interface) []byte {

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
					if certName[len(certName)-3:] == "crt" {

						data := string(cert)
						var block *pem.Block

						block, _ = pem.Decode([]byte(data))

						//parsed SSL certificate
						parsedCert, errParse := x509.ParseCertificate(block.Bytes)
						if errParse != nil {
							log.Println(errParse)
						}

						certInfo = append(certInfo, ParsedCertificate{
							CertificateSource: CertificateSource{
								ConfigMapName: configMap.Name,
								Namespace:     configMap.Namespace,
							},
							CertName:                certName,
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

	return certJson
}

// secret certificate collector function
func secretCertCollector(secretSources map[string]string, client kubernetes.Interface) []byte {

	defer func() {
		if err := recover(); err != nil {
			//panicError := errors.New(fmt.Sprintf("error:%v", err))
			//trackErrors = append(trackErrors, panicError)
			log.Println(err)

		}
	}()


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

					block, errBlock = pem.Decode([]byte(data))
				

					}

					//parsed SSL certificate
					parsedCert, errParse := x509.ParseCertificate(block.Bytes)
					if errParse != nil {
						log.Println(errParse)
						if err != nil {
							return nil
						}

					}


					func() {
						if err := recover(); err != nil {
							/*
								err := errors.New(fmt.Sprintf("error:%s", err))
								trackErrors = append(trackErrors, err)
							*/
							log.Println(err)
						}
					}()

					certInfo = append(certInfo, ParsedCertificate{
						CertificateSource: CertificateSource{
							SecretName: secret.Name,
							Namespace:  secret.Namespace,
						},
						CertName:                certName,
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
	return certJson
}
