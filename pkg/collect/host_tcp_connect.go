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

	status, message := attemptConnect(address, timeout)
	result := NetworkStatusResult{
		Status:  status,
		Message: message,
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

	var collectorErr error
	if status != NetworkStatusConnected && message != "" {
		collectorErr = errors.Errorf("failed to connect to %s: %s", address, message)
	}

	return map[string][]byte{
		name: b,
	}, collectorErr
}

func attemptConnect(address string, timeout time.Duration) (NetworkStatus, string) {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		errorMessage := err.Error()
		if strings.Contains(err.Error(), "i/o timeout") {
			return NetworkStatusConnectionTimeout, errorMessage
		}
		if strings.Contains(err.Error(), "connection refused") {
			return NetworkStatusConnectionRefused, errorMessage
		}
		return NetworkStatusErrorOther, errorMessage
	}

	conn.Close()
	return NetworkStatusConnected, ""
}

func (c *CollectHostTCPConnect) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}
