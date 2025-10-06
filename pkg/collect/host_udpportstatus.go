package collect

import (
	"bytes"
	"encoding/json"
	"net"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type CollectHostUDPPortStatus struct {
	hostCollector *troubleshootv1beta2.UDPPortStatus
	BundlePath    string
}

func (c *CollectHostUDPPortStatus) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "UDP Port Status")
}

func (c *CollectHostUDPPortStatus) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostUDPPortStatus) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	listenAddress := net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: c.hostCollector.Port,
	}

	if c.hostCollector.Interface != "" {
		iface, err := net.InterfaceByName(c.hostCollector.Interface)
		if err != nil {
			return nil, errors.Wrapf(err, "lookup interface %s", c.hostCollector.Interface)
		}
		ip, err := getIPv4FromInterface(iface)
		if err != nil {
			return nil, errors.Wrapf(err, "get ipv4 address for interface %s", c.hostCollector.Interface)
		}
		listenAddress.IP = ip
	}

	var networkStatus NetworkStatus
	var errorMessage string
	var listenErr error
	lstn, err := net.ListenUDP("udp", &listenAddress)
	if err != nil {
		errorMessage = err.Error()
		listenErr = errors.Wrap(err, "failed to listen on UDP port")
		if strings.Contains(err.Error(), "address already in use") {
			networkStatus = NetworkStatusAddressInUse
		} else {
			networkStatus = NetworkStatusErrorOther
		}
	} else {
		networkStatus = NetworkStatusConnected
		lstn.Close()
	}

	result := NetworkStatusResult{
		Status:  networkStatus,
		Message: errorMessage,
	}
	b, err := json.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal result")
	}

	collectorName := c.hostCollector.CollectorName
	if collectorName == "" {
		collectorName = "udpPortStatus"
	}
	name := filepath.Join("host-collectors/udpPortStatus", collectorName+".json")

	output := NewResult()
	output.SaveResult(c.BundlePath, name, bytes.NewBuffer(b))

	return map[string][]byte{
		name: b,
	}, listenErr
}

func (c *CollectHostUDPPortStatus) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}
