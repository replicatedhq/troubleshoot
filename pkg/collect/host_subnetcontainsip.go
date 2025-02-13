package collect

import (
	"bytes"
	"encoding/json"
	"net"
	"path/filepath"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type SubnetContainsIPResult struct {
	CIDR     string `json:"cidr"`
	IP       string `json:"ip"`
	Contains bool   `json:"contains"`
}

type CollectHostSubnetContainsIP struct {
	hostCollector *troubleshootv1beta2.SubnetContainsIP
	BundlePath    string
}

func (c *CollectHostSubnetContainsIP) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "Subnet Contains IP")
}

func (c *CollectHostSubnetContainsIP) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostSubnetContainsIP) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	_, ipNet, err := net.ParseCIDR(c.hostCollector.CIDR)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse CIDR %s", c.hostCollector.CIDR)
	}

	ip := net.ParseIP(c.hostCollector.IP)
	if ip == nil {
		return nil, errors.Errorf("failed to parse IP address %s", c.hostCollector.IP)
	}

	result := SubnetContainsIPResult{
		CIDR:     c.hostCollector.CIDR,
		IP:       c.hostCollector.IP,
		Contains: ipNet.Contains(ip),
	}

	b, err := json.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal result")
	}

	collectorName := c.hostCollector.CollectorName
	if collectorName == "" {
		collectorName = "result"
	}
	name := filepath.Join("host-collectors/subnetContainsIP", collectorName+".json")

	output := NewResult()
	output.SaveResult(c.BundlePath, name, bytes.NewBuffer(b))

	return map[string][]byte{
		name: b,
	}, nil
}

func (c *CollectHostSubnetContainsIP) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}
