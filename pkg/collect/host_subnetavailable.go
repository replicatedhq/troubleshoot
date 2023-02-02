package collect

import (
	"bytes"
	"encoding/json"
	"path/filepath"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/debug"
	"github.com/vishvananda/netlink"
)

type CollectHostSubnetAvailable struct {
	hostCollector *troubleshootv1beta2.SubnetAvailable
	BundlePath    string
}

func (c *CollectHostSubnetAvailable) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "Subnet Available")
}

func (c *CollectHostSubnetAvailable) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostSubnetAvailable) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {

	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list routes")
	}

	debug.Printf("Routes: %+v\n", routes)

	result := []byte{}

	b, err := json.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal result")
	}

	collectorName := c.hostCollector.CollectorName
	if collectorName == "" {
		collectorName = "subnetAvailable"
	}
	name := filepath.Join("host-collectors/subnetAvailable", collectorName+".json")

	output := NewResult()
	output.SaveResult(c.BundlePath, name, bytes.NewBuffer(b))

	return map[string][]byte{
		name: b,
	}, nil
}
