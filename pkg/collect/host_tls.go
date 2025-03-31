package collect

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"log"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type CertInfo struct {
	Issuer    string `json:"issuer"`
	Subject   string `json:"subject"`
	Serial    string `json:"serial"`
	NotBefore string `json:"not_before"`
	NotAfter  string `json:"not_after"`
	IsCA      bool   `json:"is_ca"`
	Raw       []byte `json:"raw"`
}

type TLSInfo struct {
	PeerCertificates      []CertInfo `json:"peer_certificates"`
	MatchesExpectedIssuer bool       `json:"matches_expected_issuer"`
}

type CollectHostTLS struct {
	hostCollector *troubleshootv1beta2.HostTLS
	BundlePath    string
}

func (c *CollectHostTLS) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "TCP Port Status")
}

func (c *CollectHostTLS) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostTLS) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	tlsInfo := TLSInfo{}

	conf := &tls.Config{
		InsecureSkipVerify: true,
	}

	conn, err := tls.Dial("tcp", c.hostCollector.Address, conf)
	if err != nil {
		log.Println("Error in Dial", err)
		return nil, errors.Wrap(err, "failed to dial tls")
	}
	defer conn.Close()
	certs := conn.ConnectionState().PeerCertificates
	cleanedCerts := make([]CertInfo, len(certs))
	for i, cert := range certs {
		cleanedCerts[i] = CertInfo{
			Issuer:    cert.Issuer.CommonName,
			Subject:   cert.Subject.CommonName,
			Serial:    cert.SerialNumber.String(),
			NotBefore: cert.NotBefore.Format(time.RFC3339),
			NotAfter:  cert.NotAfter.Format(time.RFC3339),
			IsCA:      cert.IsCA,
			Raw:       cert.Raw,
		}
	}

	tlsInfo.PeerCertificates = cleanedCerts

	if c.hostCollector.ExpectedIssuer != "" {
		if len(certs) == 0 {
			tlsInfo.MatchesExpectedIssuer = false
		} else {
			tlsInfo.MatchesExpectedIssuer = c.hostCollector.ExpectedIssuer == certs[0].Issuer.CommonName
		}
	}

	b, err := json.Marshal(tlsInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal tls info")
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, HostTimePath, bytes.NewBuffer(b))

	collectorName := c.hostCollector.CollectorName
	if collectorName == "" {
		collectorName = "result"
	}
	name := filepath.Join("host-collectors/tls", collectorName+".json")

	return map[string][]byte{
		name: b,
	}, nil
}

func (c *CollectHostTLS) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}
