package collect

import (
	"bytes"
	"encoding/json"
	"net"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type CollectHostTCPConnect struct {
	hostCollector *troubleshootv1beta2.TCPConnect
	BundlePath    string
}

func (c *CollectHostTCPConnect) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "TCP Connection Attempt")
}

func (c *CollectHostTCPConnect) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostTCPConnect) SkipRedaction() bool {
	return c.hostCollector.SkipRedaction
}

func (c *CollectHostTCPConnect) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	address := c.hostCollector.Address

	timeout := 10 * time.Second
	if c.hostCollector.Timeout != "" {
		var err error
		timeout, err = time.ParseDuration(c.hostCollector.Timeout)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse timeout %q", c.hostCollector.Timeout)
		}
	}

	result := NetworkStatusResult{
		Status: attemptConnect(address, timeout),
	}

	b, err := json.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal result")
	}

	collectorName := c.hostCollector.CollectorName
	if collectorName == "" {
		collectorName = "connect"
	}
	name := filepath.Join("host-collectors/connect", collectorName+".json")

	output := NewResult()
	output.SaveResult(c.BundlePath, name, bytes.NewBuffer(b))

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

func (c *CollectHostTCPConnect) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}
