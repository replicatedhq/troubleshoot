package collect

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"log"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/analyze/types"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

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
	tlsInfo := types.TLSInfo{}

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
	cleanedCerts := make([]types.CertInfo, len(certs))
	for i, cert := range certs {
		cleanedCerts[i] = types.CertInfo{
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
