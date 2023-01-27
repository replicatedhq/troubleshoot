package collect

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type CollectKubeSSLCertInfo struct {
	hostCollector *troubleshootv1beta2.KubeSSLCertCollect
	BundlePath    string
}

func (c *CollectKubeSSLCertInfo) Title() string {
	return getCollectorName(c)
}

func (c *CollectKubeSSLCertInfo) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

// SSL Certificate Struct
type KubeSSLCertCollect struct {
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
	FilePath string `json:"File Path,omitempty"`
}

func (c *CollectKubeSSLCertInfo) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {

	output := NewResult()

	// Json object initilization - start
	var certInfo []KubeSSLCertCollect
	var certJson = []byte("[]")
	errJson := json.Unmarshal(certJson, &certInfo)
	if errJson != nil {
		log.Println("error unmarshalling certJson: ", errJson)
	}
	// Json object initilization - end

	//file path & extension variables
	dirPath := "/etc/kubernetes/pki"
	ext := "*.crt"

	GetCertificatesNames, err := GetKubeCertsFromFilePath(dirPath, ext)
	if err != nil {
		log.Println("error retrieving ssl certificate names:", err)
	}

	results := KubeCertCollector(GetCertificatesNames)

	output.SaveResult(c.BundlePath, "ssl_certificates/kube_ssl_certificates.json", bytes.NewBuffer(results))

	//log.Println(string(results))
	return output, err

}

// This function collects certificate names with a .crt extension within the defined dirPath.
// Output is a slice of certificate names (certNames) that are fed into the KubeCertCollector function.
func GetKubeCertsFromFilePath(dirPath, ext string) ([]string, error) {
	var matches []string
	err := filepath.Walk(dirPath, func(dirPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if matched, err := filepath.Match(ext, filepath.Base(dirPath)); err != nil {
			return err
		} else if matched {
			matches = append(matches, dirPath)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matches, nil
}

// This function collects information for all SSL certificates (CertNames) located in the dirPath;
// includes all sub directories.
func KubeCertCollector(certNames []string) []byte {
	currentTime := time.Now()
	var certInfo []KubeSSLCertCollect
	var certJson = []byte("[]")
	err := json.Unmarshal(certJson, &certInfo)
	if err != nil {
		log.Println(err)
	}

	for _, cert := range certNames {

		path, file := filepath.Split(cert)
		filePath := path + file

		certFile, err := ioutil.ReadFile(filePath)
		if err != nil {
			panic(err)
		}

		block, _ := pem.Decode(certFile)
		if block == nil {
			panic("Failed to parse certificate file")
		}

		//parsed SSL certificate
		parsedCert, errParse := x509.ParseCertificate(block.Bytes)
		if errParse != nil {
			log.Println(errParse)
		}
		certInfo = append(certInfo, KubeSSLCertCollect{
			CertName:         file,
			DNSNames:         parsedCert.DNSNames,
			IssuerCommonName: parsedCert.Issuer.CommonName,
			Organizations:    parsedCert.Issuer.Organization,
			CertDate:         parsedCert.NotAfter,
			IsValid:          currentTime.Before(parsedCert.NotAfter),
			Location: location{
				FilePath: path,
			},
		})
		certJson, _ = json.MarshalIndent(certInfo, "", "\t")
	}
	return certJson
}
