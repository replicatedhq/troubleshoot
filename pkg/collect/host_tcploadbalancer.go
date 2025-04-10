package collect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type CollectHostTCPLoadBalancer struct {
	hostCollector *troubleshootv1beta2.TCPLoadBalancer
	BundlePath    string
}

func (c *CollectHostTCPLoadBalancer) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "TCP Load Balancer")
}

func (c *CollectHostTCPLoadBalancer) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostTCPLoadBalancer) SkipRedaction() bool {
	return c.hostCollector.SkipRedaction
}

func (c *CollectHostTCPLoadBalancer) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	listenAddress := fmt.Sprintf("0.0.0.0:%d", c.hostCollector.Port)
	dialAddress := c.hostCollector.Address

	collectorName := c.hostCollector.CollectorName
	if collectorName == "" {
		collectorName = "tcpLoadBalancer"
	}
	name := filepath.Join("host-collectors/tcpLoadBalancer", collectorName+".json")

	output := NewResult()

	timeout := 60 * time.Minute
	if c.hostCollector.Timeout != "" {
		var err error
		timeout, err = time.ParseDuration(c.hostCollector.Timeout)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse duration")
		}
	}
	networkStatus, err := checkTCPConnection(progressChan, listenAddress, dialAddress, timeout)
	if err != nil {
		result := NetworkStatusResult{
			Status:  networkStatus,
			Message: err.Error(),
		}
		b, err := json.Marshal(result)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal result")
		}

		output.SaveResult(c.BundlePath, name, bytes.NewBuffer(b))

		return map[string][]byte{
			name: b,
		}, err
	}
	result := NetworkStatusResult{
		Status: networkStatus,
	}

	b, err := json.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal result")
	}

	output.SaveResult(c.BundlePath, name, bytes.NewBuffer(b))

	return map[string][]byte{
		name: b,
	}, nil
}

func (c *CollectHostTCPLoadBalancer) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}
