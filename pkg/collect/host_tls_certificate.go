package collect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/analyze/types"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type CollectHostTLSCertificate struct {
	hostCollector *troubleshootv1beta2.HostTLSCertificate
	BundlePath    string
}

func (c *CollectHostTLSCertificate) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "TCP Port Status")
}

func (c *CollectHostTLSCertificate) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostTLSCertificate) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	tlsInfo := types.TLSInfo{}

	resp, err := doRequest("GET", fmt.Sprintf("https://%s", c.hostCollector.Address), nil, "", true, "", nil, c.hostCollector.HttpsProxy)
	if err != nil {
		tlsInfo.Error = err.Error()
	} else {
		defer resp.Body.Close()
		certs := resp.TLS.PeerCertificates
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
	}

	b, err := json.Marshal(tlsInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal tls info")
	}

	collectorName := c.hostCollector.CollectorName
	if collectorName == "" {
		collectorName = strings.ReplaceAll(c.hostCollector.Address, ":", "-")
	}
	name := filepath.Join("host-collectors/tls-certificate", collectorName+".json")

	output := NewResult()
	err = output.SaveResult(c.BundlePath, name, bytes.NewBuffer(b))
	if err != nil {
		return nil, errors.Wrap(err, "failed to save result")
	}

	return map[string][]byte{
		name: b,
	}, nil
}

func (c *CollectHostTLSCertificate) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}
