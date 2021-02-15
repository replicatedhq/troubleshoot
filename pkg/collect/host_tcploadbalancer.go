package collect

import (
	"encoding/json"
	"fmt"
	"path"
	"time"

	"github.com/pkg/errors"
)

func HostTCPLoadBalancer(c *HostCollector) (map[string][]byte, error) {
	listenAddress := fmt.Sprintf("0.0.0.0:%d", c.Collect.TCPLoadBalancer.Port)
	dialAddress := c.Collect.TCPLoadBalancer.Address

	timeout := 60 * time.Minute
	if c.Collect.TCPLoadBalancer.Timeout != "" {
		var err error
		timeout, err = time.ParseDuration(c.Collect.TCPLoadBalancer.Timeout)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse durection")
		}
	}

	networkStatus, err := checkTCPConnection(listenAddress, dialAddress, timeout)
	if err != nil {
		return nil, err
	}

	result := NetworkStatusResult{
		Status: networkStatus,
	}

	b, err := json.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal result")
	}

	name := path.Join("tcpLoadBalancer", "tcpLoadBalancer.json")
	if c.Collect.TCPLoadBalancer.CollectorName != "" {
		name = path.Join("tcpLoadBalancer", fmt.Sprintf("%s.json", c.Collect.TCPLoadBalancer.CollectorName))
	}

	return map[string][]byte{
		name: b,
	}, nil
}
