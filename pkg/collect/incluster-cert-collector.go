package collect

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"log"
	"path/filepath"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type CollectInClusterCertificateInfo struct {
	Collector    *troubleshootv1beta2.InClusterCertificateInfo
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

// SSL Certificate Struct
type Certificate struct {
	CertName         string       `json:"Certificate Name"`
	DNSNames         []string     `json:"DNS Names"`
	IssuerCommonName string       `json:"Issuer"`
	Organizations    []string     `json:"Issuer Organizations"`
	CertDate         time.Time    `json:"Certificate Expiration Date"`
	IsValid          bool         `json:"IsValid"`
	Location         CertLocation `json:"Location,omitempty"`
}

// SSL Cert Location Struct
type CertLocation struct {
	Secret          string `json:"Secret Name,omitempty"`
	SecretNamespace string `json:"Secret Namespace,omitempty"`
}

func (c *CollectInClusterCertificateInfo) Title() string {
	return getCollectorName(c)
}

func (c *CollectInClusterCertificateInfo) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectInClusterCertificateInfo) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	/* Go Client Config -- start
	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, err
	} */ //Go Client Config -- end

	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()
	// uses the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}
	// creates the clientsets
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	output := NewResult()
	// Json object initilization - start
	var certInfo []Certificate
	var certJson = []byte("[]")
	errJson := json.Unmarshal(certJson, &certInfo)
	if errJson != nil {
		log.Println(errJson)
	} // Json object initilization - end

	// Collects SSL certificate data from "kubelet-client-cert" secret (Opaque) associated with deployment.apps/kotsadm.
	kubeletclientcertCerts := OpaqueSecretCertCollector("kubelet-client-cert", client)

	// Collects SSL certificate data from "registry-pki" secret (Opaque) associated with deployment.apps/registry.
	registrypkiCerts := OpaqueSecretCertCollector("registry-pki", client)

	// Collects SSL certificate data from all Secrets in a k8s cluster that are of type "kubernetes.io/tls".
	tlsSecretsCerts := TLSSecretCertCollector("type=kubernetes.io/tls", client)

	// Appends SSL certificate "kubelet-client-cert" and "registry-pki" collections to results Json.
	results := append(kubeletclientcertCerts, registrypkiCerts...)

	// Appends collections of SSL certs of Secrets with type "kubernetes.io/tls" to results Json.
	results = append(results, tlsSecretsCerts...)

	output.SaveResult(c.BundlePath, "certificates/incluster_ssl_certificates.json", bytes.NewBuffer(results))

	return output, err
}

// This function collects information for all certificates in the named Secret (secretName).
// This function should be used when a Secret is of type Opaque (NOT type of "kubernetes.io/tls").
// SecretName == name of secret to collect SSL certificates from.
func OpaqueSecretCertCollector(secretName string, client kubernetes.Interface) []byte {

	currentTime := time.Now()
	var certInfo []Certificate
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

					certInfo = append(certInfo, Certificate{
						CertName:         certName,
						DNSNames:         parsedCert.DNSNames,
						IssuerCommonName: parsedCert.Issuer.CommonName,
						Organizations:    parsedCert.Issuer.Organization,
						CertDate:         parsedCert.NotAfter,
						IsValid:          currentTime.Before(parsedCert.NotAfter),
						Location: CertLocation{
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
	var certInfo []Certificate

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

				certInfo = append(certInfo, Certificate{
					CertName:         certName,
					DNSNames:         parsedCert.DNSNames,
					IssuerCommonName: parsedCert.Issuer.CommonName,
					Organizations:    parsedCert.Issuer.Organization,
					CertDate:         parsedCert.NotAfter,
					IsValid:          currentTime.Before(parsedCert.NotAfter),
					Location: CertLocation{
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
