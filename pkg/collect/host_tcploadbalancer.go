package collect

import (
	"encoding/json"
	"fmt"
	"net"
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/segmentio/ksuid"
)

type HostTCPLoadBalancerResult struct {
	Status string `json:"status"`
}

type ConnectionResult int

const (
	ConnectionRefused ConnectionResult = iota
	Connected
	ConnectionTimeout
	ConnectionAddressInUse
	ErrorOther
)

func HostTCPLoadBalancer(c *HostCollector) (map[string][]byte, error) {
	result := HostTCPLoadBalancerResult{}

	name := path.Join("tcpLoadBalancer", "tcpLoadBalancer.json")
	if c.Collect.TCPLoadBalancer.CollectorName != "" {
		name = path.Join("tcpLoadBalancer", fmt.Sprintf("%s.json", c.Collect.TCPLoadBalancer.CollectorName))
	}

	connectionResult, err := doAttemptTCPConnection(c.Collect.TCPLoadBalancer.Address, c.Collect.TCPLoadBalancer.Port, 5*time.Millisecond, "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to check tcp connection in lb")
	}

	if connectionResult == Connected {
		result.Status = "already"

		b, err := json.Marshal(result)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal result")
		}

		return map[string][]byte{
			name: b,
		}, nil
	}

	payload := ksuid.New().String()

	// if there's not a connection already, create a tcp server and start listening
	lstn, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", c.Collect.TCPLoadBalancer.Port))
	if err != nil {
		if strings.Contains(err.Error(), "address already in use") {
			result.Status = "port-unavailable"

			b, err := json.Marshal(result)
			if err != nil {
				return nil, errors.Wrap(err, "failed to marshal result")
			}

			return map[string][]byte{
				name: b,
			}, nil
		}

		return nil, errors.Wrap(err, "failed to create listener")
	}
	defer lstn.Close()

	go func() {
		for {
			conn, err := lstn.Accept()
			if err != nil {
				fmt.Printf("%s\n", err.Error())
			}

			buf := make([]byte, 1024)
			_, err = conn.Read(buf)
			if err != nil {
				fmt.Printf("%s\n", err.Error())
			}

			fmt.Printf("%s\n", buf)
			conn.Close()
		}
	}()

	timeout := 60 * time.Minute
	if c.Collect.TCPLoadBalancer.Timeout != "" {
		timeout, err = time.ParseDuration(c.Collect.TCPLoadBalancer.Timeout)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse durection")
		}
	}

	connectionResult, err = doAttemptTCPConnection(c.Collect.TCPLoadBalancer.Address, c.Collect.TCPLoadBalancer.Port, timeout, payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to attempt connection with listener running")
	}

	switch connectionResult {
	case Connected:
		result.Status = "connected"
		break
	case ConnectionRefused:
		result.Status = "refused"
		break
	case ConnectionTimeout:
		result.Status = "timeout"
		break
	}

	b, err := json.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal result")
	}

	return map[string][]byte{
		name: b,
	}, nil
}

func doAttemptTCPConnection(address string, port int, timeout time.Duration, payload string) (ConnectionResult, error) {
	stopAfter := time.Now().Add(timeout)

	for {
		if time.Now().After(stopAfter) {
			return ConnectionTimeout, nil
		}

		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", address, port), timeout)
		if err != nil {
			if strings.Contains(err.Error(), "connection refused") {
				return ConnectionRefused, nil
			}
			if !strings.Contains(err.Error(), "i/o timeout") {
				return ErrorOther, errors.Wrap(err, "failed to dial")
			}
		}

		if err == nil {
			if payload != "" {
				_, err := conn.Write([]byte(payload))
				if err != nil {
					return ErrorOther, errors.Wrap(err, "failed to write")
				}
			}
			return Connected, nil
		}
		time.Sleep(time.Millisecond * 50)
	}
}
