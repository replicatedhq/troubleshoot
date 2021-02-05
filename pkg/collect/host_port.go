package collect

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/pkg/errors"
)

type PortStatus string

const (
	PortStatusUnavailable = "unavailable"
	PortStatusRefused     = "refused"
	PortStatusTimeout     = "timeout"
	PortStatusOpen        = "open"
	PortStatusOther       = "other"
)

type HostPortResult struct {
	Status PortStatus `json:"status"`
}

func HostTCPPortStatus(c *HostCollector) (map[string][]byte, error) {
	result := HostPortResult{}
	key := fmt.Sprintf("port/%s/tcp.json", c.Collect.TCPPortStatus.CollectorName)

	dialIP := ""
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
		dialIP = ip.String()
	}

	if dialIP == "" {
		ip, err := getLocalIPv4()
		if err != nil {
			return nil, err
		}
		dialIP = ip.String()
	}

	lstn, err := net.Listen("tcp", listenAddress)
	if err != nil {
		if strings.Contains(err.Error(), "address already in use") {
			result.Status = PortStatusUnavailable

			b, err := json.Marshal(result)
			if err != nil {
				return nil, errors.Wrap(err, "failed to marshal result")
			}

			return map[string][]byte{
				key: b,
			}, nil
		}

		return nil, errors.Wrap(err, "failed to create listener")
	}
	defer lstn.Close()

	payload := "tcp raw data"
	done := make(chan struct{})

	go func() {
		for {
			conn, err := lstn.Accept()
			if err != nil {
				return
			}

			buf := make([]byte, len([]byte(payload)))
			_, err = conn.Read(buf)
			if err != nil {
				fmt.Printf("%s\n", err.Error())
				return
			}

			conn.Close()

			if string(buf) == payload {
				done <- struct{}{}
				return
			}
		}
	}()

	connectionResult, err := doAttemptTCPConnection(dialIP, c.Collect.TCPPortStatus.Port, 5*time.Second, payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check tcp connection to port")
	}

	switch connectionResult {
	case Connected:
		println("blocked")
		<-done
		result.Status = PortStatusOpen
	case ConnectionRefused:
		result.Status = PortStatusRefused
	case ConnectionTimeout:
		result.Status = PortStatusTimeout
	case ConnectionAddressInUse:
		result.Status = PortStatusUnavailable
	case ErrorOther:
		result.Status = PortStatusOther
	}

	b, err := json.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal result")
	}

	return map[string][]byte{
		key: b,
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
