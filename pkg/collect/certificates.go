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
	for secretName, namespaces := range c.Collector.Secrets {
		for _, namespace := range namespaces {

			secretCollections := secretCertCollector(secretName, namespace, c.Client)
			results = append(results, secretCollections...)
		}
	}

	// collect secret certificate
	/*
		for configMapName, namespaces := range c.Collector.ConfigMaps {
			for _, namespace := range namespaces {

				configMapCollections := configMapCertCollector(configMapName, namespace, c.Client)
				log.Println("configMap: ", configMapName)
				log.Println("namespace: ", namespace)
				results = append(results, configMapCollections...)
				//log.Println("final results: ", results)
			}
		}
	*/

	certsJson, _ := json.MarshalIndent(results, "", "\t")

	filePath := "certificates/certificates.json"

	output.SaveResult(c.BundlePath, filePath, bytes.NewBuffer(certsJson))

	return output, nil
}

// configmap certificate collector function will
// func configMapCertCollector(configMapName string, namespace string, client kubernetes.Interface) CertCollection {
func configMapCertCollector(configMapName string, namespace string, client kubernetes.Interface) []CertCollection {

	results := []CertCollection{}

	// Collect from configMaps
	listOptions := metav1.ListOptions{}

	configMaps, _ := client.CoreV1().ConfigMaps(namespace).List(context.Background(), listOptions)

	trackErrors := []string{}

	collection := []ParsedCertificate{}

	for _, configMap := range configMaps.Items {
		if configMapName == configMap.Name {
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

		}
	}

	return results
}

// secret certificate collector function
// func secretCertCollector(secretName map[string]string, client kubernetes.Interface) CertCollection {

func secretCertCollector(secretName string, namespace string, client kubernetes.Interface) []CertCollection {

	results := []CertCollection{}

	// Collect from secrets
	listOptions := metav1.ListOptions{}
	// TODO: Handle RBAC errors. Not to be worked on yet
	secrets, _ := client.CoreV1().Secrets(namespace).List(context.Background(), listOptions)
	trackErrors := []string{}

	collection := []ParsedCertificate{}

	for _, secret := range secrets.Items {
		//if secretName == secret.Name {
		// Collect from secret
		source := &CertificateSource{
			SecretName: secret.Name,
			Namespace:  secret.Namespace,
		}

		for certName, certs := range secret.Data {

			//certInfo := CertParser(certName, certs, certCollection)
			certInfo, _ := CertParser(certName, certs)

			collection = append(collection, certInfo...)

			//log.Println("coolection: ", collection)

		}
		results = append(results, CertCollection{
			Source:           source,
			Errors:           trackErrors,
			CertificateChain: collection,
		})
		//}

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
	//func CertParser(certName string, certs []byte, parsedCerts []ParsedCertificate, source *CertificateSource, trackErrors []string) CertCollection {
	currentTime := time.Now()
	data := string(certs)
	certInfo := []ParsedCertificate{}
	trackErrors := []string{}
	//results := CertCollection{}

	certChain := decodePem(data)

	//if strings.Contains(data, "BEGIN CERTIFICATE") && strings.Contains(data, "END CERTIFICATE") {

	for _, cert := range certChain.Certificate {

		//parsed SSL certificate
		parsedCert, errParse := x509.ParseCertificate(cert)
		if errParse != nil {
			trackErrors = append(trackErrors, "error: This object is not a certificate")
			continue // End here, start parsing the next cert in the for loop
		}

		certInfo := append(certInfo, ParsedCertificate{
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
		return certInfo, nil

	}

	//}

	return certInfo, nil
}
