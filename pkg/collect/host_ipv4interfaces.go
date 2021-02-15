package collect

import (
	"encoding/json"
	"net"

	"github.com/pkg/errors"
)

func HostIPV4Interfaces(c *HostCollector) (map[string][]byte, error) {
	var ipv4Interfaces []net.Interface

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
		if ip == nil {
			continue
		}
		ipv4Interfaces = append(ipv4Interfaces, iface)
	}

	b, err := json.Marshal(ipv4Interfaces)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal network interfaces")
	}

	return map[string][]byte{
		"system/ipv4Interfaces.json": b,
	}, nil
}
