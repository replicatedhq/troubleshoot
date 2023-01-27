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

type CollectInClusterSSLCertInfo struct {
	Collector    *troubleshootv1beta2.InClusterSSLCertInfo
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

// SSL Certificate Struct
type sslCert struct {
	CertName         string    `json:"Certificate Name"`
	DNSNames         []string  `json:"DNS Names"`
	IssuerCommonName string    `json:"Issuer"`
	Organizations    []string  `json:"Issuer Organizations"`
	CertDate         time.Time `json:"Certificate Expiration Date"`
	IsValid          bool      `json:"IsValid"`
	Location         location  `json:"Location,omitempty"`
}

// SSL Cert Location Struct
type location struct {
	Secret          string `json:"Secret Name,omitempty"`
	SecretNamespace string `json:"Secret Namespace,omitempty"`
}

func (c *CollectInClusterSSLCertInfo) Title() string {
	return getCollectorName(c)
}

func (c *CollectInClusterSSLCertInfo) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectInClusterSSLCertInfo) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	// Go Client Config -- start
	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, err
	} // Go Client Config -- end

	// Json object initilization - start
	var certInfo []sslCert
	var certJson = []byte("[]")
	errJson := json.Unmarshal(certJson, &certInfo)
	if errJson != nil {
		log.Println(errJson)
	} // Json object initilization - end

	// Collects SSL certificate data from "kubelet-client-cert" secret (Opaque) associated with deployment.apps/kotsadm.
	kubeletclientcertCerts := OpaqueSecretCertCollector("kubelet-client-cert", clientset)

	// Collects SSL certificate data from "registry-pki" secret (Opaque) associated with deployment.apps/registry.
	registrypkiCerts := OpaqueSecretCertCollector("registry-pki", clientset)

	// Collects SSL certificate data from all Secrets in a k8s cluster that are of type "kubernetes.io/tls".
	tlsSecretsCerts := TLSSecretCertCollector("type=kubernetes.io/tls", clientset)

	// Appends SSL certificate "kubelet-client-cert" and "registry-pki" collections to results Json.
	results := append(kubeletclientcertCerts, registrypkiCerts...)

	// Appends collections of SSL certs of Secrets with type "kubernetes.io/tls" to results Json.
	results = append(results, tlsSecretsCerts...)

	output.SaveResult(c.BundlePath, "ssl_certificates/incluster_ssl_certificates.json", bytes.NewBuffer(results))

	return output, err
}

// This function collects information for all certificates in the named Secret (secretName).
// This function should be used when a Secret is of type Opaque (NOT type of "kubernetes.io/tls").
// SecretName == name of secret to collect SSL certificates from.
func OpaqueSecretCertCollector(secretName string, client kubernetes.Interface) []byte {

	currentTime := time.Now()
	var certInfo []sslCert
	var certJson = []byte("[]")
	err := json.Unmarshal(certJson, &certInfo)
	if err != nil {
		log.Println(err)
	}

	listOptions := metav1.ListOptions{}
	secrets, _ := client.CoreV1().Secrets("").List(context.Background(), listOptions)

	for _, secret := range secrets.Items {
		if secretName == secret.Name {

			for certName, certSSL := range secret.Data {
				if certName[len(certName)-3:] == "crt" {

					data := string(certSSL)
					var block *pem.Block

					block, _ = pem.Decode([]byte(data))

					//parsed SSL certificate
					parsedCert, errParse := x509.ParseCertificate(block.Bytes)
					if errParse != nil {
						log.Println(errParse)
					}

					certInfo = append(certInfo, sslCert{
						CertName:         certName,
						DNSNames:         parsedCert.DNSNames,
						IssuerCommonName: parsedCert.Issuer.CommonName,
						Organizations:    parsedCert.Issuer.Organization,
						CertDate:         parsedCert.NotAfter,
						IsValid:          currentTime.Before(parsedCert.NotAfter),
						Location: location{
							Secret:          secret.Name,
							SecretNamespace: secret.Namespace,
						},
					})
					certJson, _ = json.MarshalIndent(certInfo, "", "\t")
				}
			}
		}
	}
	return certJson
}

// This function collects information for all certificates in Secrets of type "kubernetes.io/tls"
// This function will collect SSL certificate information for all Secrets of type "kubernetes.io/tls".
func TLSSecretCertCollector(fieldSelector string, client kubernetes.Interface) []byte {

	currentTime := time.Now()
	var certInfo []sslCert

	var certJson = []byte("[]")

	err := json.Unmarshal(certJson, &certInfo)
	if err != nil {
		log.Println(err)
	}

	listOptions := metav1.ListOptions{
		FieldSelector: fieldSelector,
	}
	secrets, _ := client.CoreV1().Secrets("").List(context.Background(), listOptions)

	for _, secret := range secrets.Items {

		for certName, certSSL := range secret.Data {
			if certName[len(certName)-3:] == "crt" {

				data := string(certSSL)
				var block *pem.Block

				block, _ = pem.Decode([]byte(data))

				//parsed SSL certificate
				parsedCert, errParse := x509.ParseCertificate(block.Bytes)
				if errParse != nil {
					log.Println(errParse)
				}

				certInfo = append(certInfo, sslCert{
					CertName:         certName,
					DNSNames:         parsedCert.DNSNames,
					IssuerCommonName: parsedCert.Issuer.CommonName,
					Organizations:    parsedCert.Issuer.Organization,
					CertDate:         parsedCert.NotAfter,
					IsValid:          currentTime.Before(parsedCert.NotAfter),
					Location: location{
						Secret:          secret.Name,
						SecretNamespace: secret.Namespace,
					},
				})
				certJson, _ = json.MarshalIndent(certInfo, "", "\t")
			}
		}
	}
	return certJson
}
