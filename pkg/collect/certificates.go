package collect

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
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
	Errors           []string            `json:"errors,omitempty"`
	CertificateChain []ParsedCertificate `json:"certificateChain,omitempty"`
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

func (c *CollectCertificates) SkipRedaction() bool {
	return c.Collector.SkipRedaction
}

func (c *CollectCertificates) Collect(progressChan chan<- interface{}) (CollectorResult, error) {

	output := NewResult()
	results := []CertCollection{}

	// collect certificates from secrets
	for _, secret := range c.Collector.Secrets {
		for _, namespace := range secret.Namespaces {
			secretCollections := secretCertCollector(secret.Name, namespace, c.Client)
			results = append(results, secretCollections...)
		}
	}

	// collect certificates from configMaps
	for _, configMap := range c.Collector.ConfigMaps {
		for _, namespace := range configMap.Namespaces {
			configMapCollections := configMapCertCollector(configMap.Name, namespace, c.Client)
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

// configmap certificate collector
func configMapCertCollector(configMapName string, namespace string, client kubernetes.Interface) []CertCollection {

	results := []CertCollection{}
	var trackErrors []string
	collection := []ParsedCertificate{}

	getOptions := metav1.GetOptions{}

	// Collect from configMaps
	configMap, err := client.CoreV1().ConfigMaps(namespace).Get(context.Background(), configMapName, getOptions)
	if err != nil {

		// collect certificate source information
		source := &CertificateSource{
			ConfigMapName: configMapName,
			Namespace:     namespace,
		}
		trackErrors := append(trackErrors, err.Error())

		results = append(results, CertCollection{
			Source: source,
			Errors: trackErrors,
		})
	} else {
		//Collect from configMap
		source := &CertificateSource{
			ConfigMapName: configMap.Name,
			Namespace:     configMap.Namespace,
		}
		for certName, c := range configMap.Data {

			certs := []byte(c)

			certInfo, parserError := CertParser(certName, certs, time.Now())

			trackErrors = append(trackErrors, parserError...)

			collection = append(collection, certInfo...)
		}

		results = append(results, CertCollection{
			Source:           source,
			Errors:           trackErrors,
			CertificateChain: collection,
		})
	}

	return results
}

// secret certificate collector
func secretCertCollector(secretName string, namespace string, client kubernetes.Interface) []CertCollection {

	results := []CertCollection{}
	var trackErrors []string
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
		trackErrors = append(trackErrors, err.Error())

		results = append(results, CertCollection{
			Source: source,
			Errors: trackErrors,
		})
	} else {
		// Collect from secret
		source := &CertificateSource{
			SecretName: secret.Name,
			Namespace:  secret.Namespace,
		}

		for certName, certs := range secret.Data {
			certInfo, parserError := CertParser(certName, certs, time.Now())

			trackErrors = append(trackErrors, parserError...)

			collection = append(collection, certInfo...)

		}
		results = append(results, CertCollection{
			Source:           source,
			Errors:           trackErrors,
			CertificateChain: collection,
		})
	}

	return results
}

// Certificate parser
func CertParser(certName string, certs []byte, currentTime time.Time) ([]ParsedCertificate, []string) {
	certInfo := []ParsedCertificate{}
	var trackErrors []string
	if currentTime.IsZero() {
		currentTime = time.Now()
	}
	certChain, decodePemTrackErrors := decodePem(certs)

	if decodePemTrackErrors != "" {
		trackErrors = append(trackErrors, decodePemTrackErrors+" "+certName)
		return nil, trackErrors
	} else {
		for _, cert := range certChain.Certificate {
			parsedCert, errParse := x509.ParseCertificate(cert)
			if errParse != nil {
				trackErrors = append(trackErrors, errParse.Error())
				return nil, trackErrors
			}

			certInfo = append(certInfo, ParsedCertificate{
				CertName:                certName,
				Subject:                 parsedCert.Subject.ToRDNSequence().String(),
				SubjectAlternativeNames: parsedCert.DNSNames,
				Issuer:                  parsedCert.Issuer.ToRDNSequence().String(),
				NotAfter:                parsedCert.NotAfter,
				NotBefore:               parsedCert.NotBefore,
				IsValid:                 currentTime.Before(parsedCert.NotAfter) && currentTime.After(parsedCert.NotBefore),
				IsCA:                    parsedCert.IsCA,
			})
		}

	}

	return certInfo, trackErrors
}
