package collect

import (
	"encoding/json"
	"fmt"
	"net"
	"path"
	"time"

	"github.com/pkg/errors"
)

func HostTCPPortStatus(c *HostCollector) (map[string][]byte, error) {
	dialAddress := ""
	listenAddress := fmt.Sprintf("0.0.0.0:%d", c.Collect.TCPPortStatus.Port)

	if c.Collect.TCPPortStatus.Interface != "" {
		iface, err := net.InterfaceByName(c.Collect.TCPPortStatus.Interface)
		if err != nil {
			return nil, errors.Wrapf(err, "lookup interface %s", c.Collect.TCPPortStatus.Interface)
		}
		ip, err := getIPv4FromInterface(iface)
		if err != nil {
			return nil, errors.Wrapf(err, "get ipv4 address for interface %s", c.Collect.TCPPortStatus.Interface)
		}
		listenAddress = fmt.Sprintf("%s:%d", ip, c.Collect.TCPPortStatus.Port)
		dialAddress = listenAddress
	}

	if dialAddress == "" {
		ip, err := getLocalIPv4()
		if err != nil {
			return nil, err
		}
		dialAddress = fmt.Sprintf("%s:%d", ip, c.Collect.TCPPortStatus.Port)
	}

	networkStatus, err := checkTCPConnection(listenAddress, dialAddress, 10*time.Second)
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

	name := path.Join("tcpPortStatus", "tcpPortStatus.json")
	if c.Collect.TCPPortStatus.CollectorName != "" {
		name = path.Join("tcpPortStatus", fmt.Sprintf("%s.json", c.Collect.TCPPortStatus.CollectorName))
	}
	return map[string][]byte{
		name: b,
	}, nil
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
