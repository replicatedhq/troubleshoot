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

// Certificate collection struct
type CertCollection struct {
	Source           *CertificateSource  `json:"source"`
	Errors           []string            `json:"errors"`
	CertificateChain []ParsedCertificate `json:"certificateChain"`
}

// Certificate source
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

	// collect certificates from secrets
	for secretName, namespaces := range c.Collector.Secrets {
		for _, namespace := range namespaces {

			secretCollections := secretCertCollector(secretName, namespace, c.Client)
			results = append(results, secretCollections...)
		}
	}

	// collect certificates from configMaps
	for configMapName, namespaces := range c.Collector.ConfigMaps {
		for _, namespace := range namespaces {
			configMapCollections := configMapCertCollector(configMapName, namespace, c.Client)
			results = append(results, configMapCollections...)
		}
	}

	certsJson, _ := json.MarshalIndent(results, "", "\t")

	filePath := "certificates/certificates.json"

	output.SaveResult(c.BundlePath, filePath, bytes.NewBuffer(certsJson))

	return output, nil
}

// configmap certificate collector function will
func configMapCertCollector(configMapName string, namespace string, client kubernetes.Interface) []CertCollection {

	results := []CertCollection{}

	//listOptions := metav1.ListOptions{}
	//configMaps, _ := client.CoreV1().ConfigMaps(namespace).List(context.Background(), listOptions)

	getOptions := metav1.GetOptions{}

	// Collect from configMaps
	configMap, _ := client.CoreV1().ConfigMaps(namespace).Get(context.Background(), configMapName, getOptions)

	trackErrors := []string{}

	collection := []ParsedCertificate{}

	//Collect from configMap
	source := &CertificateSource{
		ConfigMapName: configMap.Name,
		Namespace:     configMap.Namespace,
	}
	for certName, c := range configMap.Data {

		certs := []byte(c)

		certInfo, _ := CertParser(certName, certs)

		collection = append(collection, certInfo...)
	}
	results = append(results, CertCollection{
		Source:           source,
		Errors:           trackErrors,
		CertificateChain: collection,
	})

	return results
}

// secret certificate collector function
func secretCertCollector(secretName string, namespace string, client kubernetes.Interface) []CertCollection {

	results := []CertCollection{}

	//listOptions := metav1.ListOptions{}
	//secrets, _ := client.CoreV1().Secrets(namespace).List(context.Background(), listOptions)

	// TODO: Handle RBAC errors. Not to be worked on yet
	getOptions := metav1.GetOptions{}
	// Collect from secrets
	secret, _ := client.CoreV1().Secrets(namespace).Get(context.Background(), secretName, getOptions)

	trackErrors := []string{}

	collection := []ParsedCertificate{}

	if secret.Name == "" {
		log.Println("The secret does not exist in this namespace")
		trackErrors = append(trackErrors, "The secret does not exist in this namespace")
		secret.Name = secretName
		secret.Namespace = namespace
	}

	// Collect from secret
	source := &CertificateSource{
		SecretName: secret.Name,
		Namespace:  secret.Namespace,
	}

	for certName, certs := range secret.Data {
		certInfo, _ := CertParser(certName, certs)

		collection = append(collection, certInfo...)

	}
	results = append(results, CertCollection{
		Source:           source,
		Errors:           trackErrors,
		CertificateChain: collection,
	})

	return results
}

// decode pem and validate data source contains
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

// Certificate parser
func CertParser(certName string, certs []byte) ([]ParsedCertificate, []string) {
	//func CertParser(certName string, certs []byte, parsedCerts []ParsedCertificate, source *CertificateSource, trackErrors []string) CertCollection {
	//TODO: return trackErrors as well.
	currentTime := time.Now()
	data := string(certs)
	certInfo := []ParsedCertificate{}
	trackErrors := []string{}

	certChain := decodePem(data)

	for _, cert := range certChain.Certificate {

		//parsed SSL certificate
		parsedCert, errParse := x509.ParseCertificate(cert)
		if errParse != nil {
			trackErrors = append(trackErrors, errParse.Error())
			continue
		}

		certInfo = append(certInfo, ParsedCertificate{
			CertName:                certName,
			Subject:                 parsedCert.Subject.ToRDNSequence().String(),
			SubjectAlternativeNames: parsedCert.DNSNames,
			Issuer:                  parsedCert.Issuer.CommonName,
			NotAfter:                parsedCert.NotAfter,
			NotBefore:               parsedCert.NotBefore,
			IsValid:                 currentTime.Before(parsedCert.NotAfter),
			IsCA:                    parsedCert.IsCA,
		})
	}
	return certInfo, nil //return trackErrors
}
