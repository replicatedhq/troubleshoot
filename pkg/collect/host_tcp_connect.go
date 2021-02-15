package collect

import (
	"encoding/json"
	"fmt"
	"net"
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"
)

func HostTCPConnect(c *HostCollector) (map[string][]byte, error) {
	address := c.Collect.TCPConnect.Address

	timeout := 10 * time.Second
	if c.Collect.TCPConnect.Timeout != "" {
		var err error
		timeout, err = time.ParseDuration(c.Collect.TCPConnect.Timeout)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse timeout %q", c.Collect.TCPConnect.Timeout)
		}
	}

	result := NetworkStatusResult{
		Status: attemptConnect(address, timeout),
	}

	b, err := json.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal result")
	}

	name := path.Join("connect", fmt.Sprintf("%s.json", c.Collect.TCPConnect.CollectorName))

	return map[string][]byte{
		name: b,
	}, nil
}

func attemptConnect(address string, timeout time.Duration) NetworkStatus {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		if strings.Contains(err.Error(), "i/o timeout") {
			return NetworkStatusConnectionTimeout
		}
		if strings.Contains(err.Error(), "connection refused") {
			return NetworkStatusConnectionRefused
		}
		return NetworkStatusErrorOther
	}

	conn.Close()
	return NetworkStatusConnected
}
