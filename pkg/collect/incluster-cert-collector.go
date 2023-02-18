package collect

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"log"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

type Secret struct {
	Name      string `json:"secret Name,omitempty"`
	Namespace string `json:"secret Namespace,omitempty"`
}

// Certificate Struct
type ParsedCertificate struct {
	CertName         string    `json:"Certificate Name"`
	DNSNames         []string  `json:"DNS Names"`
	IssuerCommonName string    `json:"Issuer"`
	Organizations    []string  `json:"Issuer Organizations"`
	CertDate         time.Time `json:"Certificate Expiration Date"`
	IsValid          bool      `json:"IsValid"`
	SecretInfo       Secret    `json:"Secret"`
}

func (c *CollectInClusterCertificateInfo) Title() string {
	return getCollectorName(c)
}

func (c *CollectInClusterCertificateInfo) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectInClusterCertificateInfo) Collect(progressChan chan<- interface{}) (CollectorResult, error) {

	output := NewResult()
	// Json object initilization - start
	var certInfo []ParsedCertificate
	var certJson = []byte("[]")
	errJson := json.Unmarshal(certJson, &certInfo)
	if errJson != nil {
		return nil, errors.Wrap(errJson, "failed to umarshal Json")
	} // Json object initilization - end

	// Collects SSL certificate data from "registry-pki" secret (Opaque) associated with deployment.apps/registry.
	certificates := OpaqueSecretCertCollector(c.Collector.Name, c.Client)

	// Appends SSL certificate "kubelet-client-cert" and "registry-pki" collections to results Json.
	results := certificates

	filePath := "certificates/" + c.Collector.Name + "incluster-certificates.json"

	output.SaveResult(c.BundlePath, filePath, bytes.NewBuffer(results))

	return output, errors.New("collector name is:" + c.Collector.Name)
}

// This function collects information for all certificates in the named Secret (secretName).
// This function should be used when a Secret is of type Opaque (NOT type of "kubernetes.io/tls").
// SecretName == name of secret to collect SSL certificates from.
func OpaqueSecretCertCollector(secretName string, client kubernetes.Interface) []byte {

	currentTime := time.Now()
	var certInfo []ParsedCertificate
	var certJson = []byte("[]")
	err := json.Unmarshal(certJson, &certInfo)
	if err != nil {
		log.Println(err)
	}

	listOptions := metav1.ListOptions{}
	secrets, _ := client.CoreV1().Secrets("").List(context.Background(), listOptions)

	for _, secret := range secrets.Items {
		if secretName == secret.Name {

			for certName, cert := range secret.Data {
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
						CertName:         certName,
						DNSNames:         parsedCert.DNSNames,
						IssuerCommonName: parsedCert.Issuer.CommonName,
						Organizations:    parsedCert.Issuer.Organization,
						CertDate:         parsedCert.NotAfter,
						IsValid:          currentTime.Before(parsedCert.NotAfter),
						SecretInfo: Secret{
							Name:      secret.Name,
							Namespace: secret.Namespace,
						},
					})
					certJson, _ = json.MarshalIndent(certInfo, "", "\t")
				}
			}
		}
	}
	return certJson
}
