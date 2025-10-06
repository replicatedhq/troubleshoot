package collect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type CollectHostTCPPortStatus struct {
	hostCollector *troubleshootv1beta2.TCPPortStatus
	BundlePath    string
}

func (c *CollectHostTCPPortStatus) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "TCP Port Status")
}

func (c *CollectHostTCPPortStatus) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostTCPPortStatus) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	dialAddress := ""
	listenAddress := fmt.Sprintf("0.0.0.0:%d", c.hostCollector.Port)

	if c.hostCollector.Interface != "" {
		iface, err := net.InterfaceByName(c.hostCollector.Interface)
		if err != nil {
			return nil, errors.Wrapf(err, "lookup interface %s", c.hostCollector.Interface)
		}
		ip, err := getIPv4FromInterface(iface)
		if err != nil {
			return nil, errors.Wrapf(err, "get ipv4 address for interface %s", c.hostCollector.Interface)
		}
		listenAddress = fmt.Sprintf("%s:%d", ip, c.hostCollector.Port)
		dialAddress = listenAddress
	}

	if dialAddress == "" {
		ip, err := getLocalIPv4()
		if err != nil {
			return nil, err
		}
		dialAddress = fmt.Sprintf("%s:%d", ip, c.hostCollector.Port)
	}

	networkStatus, errorMessage, checkErr := checkTCPConnection(progressChan, listenAddress, dialAddress, 10*time.Second)

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
		collectorName = "tcpPortStatus"
	}
	name := filepath.Join("host-collectors/tcpPortStatus", collectorName+".json")

	output := NewResult()
	output.SaveResult(c.BundlePath, name, bytes.NewBuffer(b))

	return map[string][]byte{
		name: b,
	}, checkErr
}

func getIPv4FromInterface(iface *net.Interface) (net.IP, error) {
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, errors.Wrap(err, "list interface addresses")
	}

	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			return nil, errors.Wrapf(err, "parse interface address %q", addr.String())
		}
		ip = ip.To4()
		if ip != nil {
			return ip, nil
		}
	}

	return nil, errors.New("interface does not have an ipv4 address")
}

func getLocalIPv4() (net.IP, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, errors.Wrap(err, "list host network interfaces")
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		ip, _ := getIPv4FromInterface(&iface)
		if ip != nil {
			return ip, nil
		}
	}

	return nil, errors.New("No network interface has an IPv4 address")
}

func (c *CollectHostTCPPortStatus) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}
