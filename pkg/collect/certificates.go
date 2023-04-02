package collect

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
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

	certsJson, errCertJson := json.MarshalIndent(results, "", "\t")
	if errCertJson != nil {
		return nil, errCertJson
	}

	filePath := "certificates/certificates.json"

	err := output.SaveResult(c.BundlePath, filePath, bytes.NewBuffer(certsJson))
	if err != nil {
		return nil, err
	}

	return output, nil
}

// configmap certificate collector function will
func configMapCertCollector(configMapName string, namespace string, client kubernetes.Interface) []CertCollection {

	results := []CertCollection{}
	trackErrors := []string{}
	collection := []ParsedCertificate{}

	getOptions := metav1.GetOptions{}

	// Collect from configMaps
	configMap, err := client.CoreV1().ConfigMaps(namespace).Get(context.Background(), configMapName, getOptions)
	if err != nil {

		// collect certificate source information
		source := &CertificateSource{
			SecretName: configMapName,
			Namespace:  namespace,
		}
		trackErrors = append(trackErrors, "Either the configMap does not exist in this namespace or RBAC permissions are prenventing certificate collection")

		results = append(results, CertCollection{
			Source:           source,
			Errors:           trackErrors,
			CertificateChain: collection,
		})

		return results

	}

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
	trackErrors := []string{}
	collection := []ParsedCertificate{}

	getOptions := metav1.GetOptions{}
	// Collect from secrets
	secret, err := client.CoreV1().Secrets(namespace).Get(context.Background(), secretName, getOptions)
	if err != nil {

		// collect certificate source information
		source := &CertificateSource{
			SecretName: secretName,
			Namespace:  namespace,
		}
		trackErrors = append(trackErrors, "Either the secret does not exist in this namespace or RBAC permissions are prenventing certificate collection")

		results = append(results, CertCollection{
			Source:           source,
			Errors:           trackErrors,
			CertificateChain: collection,
		})

		return results

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
func decodePem(certInput string) (tls.Certificate, []string) {
	var cert tls.Certificate
	trackErrors := []string{}
	certPEMBlock := []byte(certInput)
	var certDERBlock *pem.Block
	for {
		certDERBlock, certPEMBlock = pem.Decode(certPEMBlock)
		if certDERBlock == nil {
			trackErrors = append(trackErrors, "decodePem function error: cert block is empty")

			break
		}
		if certDERBlock.Type == "CERTIFICATE" {
			cert.Certificate = append(cert.Certificate, certDERBlock.Bytes)
		}
	}
	return cert, trackErrors
}

// Certificate parser
func CertParser(certName string, certs []byte) ([]ParsedCertificate, []string) {
	//TODO: return trackErrors as well.
	currentTime := time.Now()
	data := string(certs)
	certInfo := []ParsedCertificate{}
	trackErrors := []string{}

	certChain, decodePemTrackErrors := decodePem(data)

	trackErrors = append(trackErrors, decodePemTrackErrors...)

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
			Issuer:                  parsedCert.Issuer.ToRDNSequence().String(),
			NotAfter:                parsedCert.NotAfter,
			NotBefore:               parsedCert.NotBefore,
			IsValid:                 currentTime.Before(parsedCert.NotAfter),
			IsCA:                    parsedCert.IsCA,
		})
	}
	return certInfo, trackErrors
}
