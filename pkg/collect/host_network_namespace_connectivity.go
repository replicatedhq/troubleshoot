package collect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/namespaces"
)

// NetworkNamespaceConnectivityInfo is the output of this collector, here we
// have the logs, the information from the source and destination namespaces,
// errors and a success flag.
type NetworkNamespaceConnectivityInfo struct {
	FromCIDR string                             `json:"from_cidr"`
	ToCIDR   string                             `json:"to_cidr"`
	Errors   NetworkNamespaceConnectivityErrors `json:"errors"`
	Output   NetworkNamespaceConnectivityOutput `json:"output"`
	Success  bool                               `json:"success"`
}

// ErrorMessage returns the error message from the errors field.
func (n *NetworkNamespaceConnectivityInfo) ErrorMessage() string {
	return n.Errors.Errors()
}

// NetworkNamespaceConnectivityErrors is a struct that contains the errors that
// occurred during the network namespace connectivity test
type NetworkNamespaceConnectivityErrors struct {
	FromCIDRCreation string `json:"from_cidr_creation"`
	ToCIDRCreation   string `json:"to_cidr_creation"`
	UDPClient        string `json:"udp_client"`
	UDPServer        string `json:"udp_server"`
	TCPClient        string `json:"tcp_client"`
	TCPServer        string `json:"tcp_server"`
}

// Errors returns a string representation of the errors found during the
// network namespace connectivity test.
func (e NetworkNamespaceConnectivityErrors) Errors() string {
	var sb strings.Builder
	if e.FromCIDRCreation != "" {
		sb.WriteString("Failed to create 'from' namespace: ")
		sb.WriteString(e.FromCIDRCreation + "\n")
	}

	if e.ToCIDRCreation != "" {
		sb.WriteString("Failed to create 'to' namespace: ")
		sb.WriteString(e.ToCIDRCreation + "\n")
	}

	if e.UDPClient != "" {
		sb.WriteString("UDP connection failed with: ")
		sb.WriteString(e.UDPClient + "\n")
	}

	if e.UDPServer != "" {
		sb.WriteString("UDP server failed with: ")
		sb.WriteString(e.UDPServer + "\n")
	}

	if e.TCPClient != "" {
		sb.WriteString("TCP connection failed with: ")
		sb.WriteString(e.TCPClient + "\n")
	}

	if e.TCPServer != "" {
		sb.WriteString("TCP server failed with: ")
		sb.WriteString(e.TCPServer + "\n")
	}
	return sb.String()
}

// NetworkNamespaceConnectivityOutput is a struct that contains the logs from
// the network namespace connectivity collector.
type NetworkNamespaceConnectivityOutput struct {
	mtx  sync.Mutex
	Logs []string `json:"logs"`
}

// Printf is a method that allows us to print the logs directly into a slice.
func (l *NetworkNamespaceConnectivityOutput) Printf(format string, v ...interface{}) {
	l.mtx.Lock()
	defer l.mtx.Unlock()

	format = fmt.Sprintf("[%s] %s", time.Now().Format(time.RFC3339), format)
	l.Logs = append(l.Logs, fmt.Sprintf(format, v...))
}

// CollectHostNetworkNamespaceConnectivity collects information about the
// capability of the host to route traffic between two different network
// namespaces. This collector will create two network namespaces and attempt to
// issue TCP and UDP requests between them.
type CollectHostNetworkNamespaceConnectivity struct {
	hostCollector *troubleshootv1beta2.HostNetworkNamespaceConnectivity
	BundlePath    string
}

// Title returns the title of the collector.
func (c *CollectHostNetworkNamespaceConnectivity) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "Host Network Namespace Connectivity")
}

// IsExcluded returns true if the collector should be excluded.
func (c *CollectHostNetworkNamespaceConnectivity) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostNetworkNamespaceConnectivity) SkipRedaction() bool {
	return c.hostCollector.SkipRedaction
}

// marshal marshals the network namespace connectivity info into a JSON file,
// writes it to the bundle path and returns the file name and the data.
func (c *CollectHostNetworkNamespaceConnectivity) marshal(info *NetworkNamespaceConnectivityInfo) (map[string][]byte, error) {
	collectorName := c.hostCollector.CollectorName
	if collectorName == "" {
		collectorName = "networkNamespaceConnectivity"
	}
	name := filepath.Join("host-collectors/system", collectorName+".json")

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal network namespace connectivity info: %w", err)
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, name, bytes.NewBuffer(data))

	return map[string][]byte{
		name: data,
	}, nil
}

// validateCIDRs validates both the from and to CIDRs. They must be provided,
// be different and be valid CIDRs.
func (c *CollectHostNetworkNamespaceConnectivity) validateCIDRs() error {
	if c.hostCollector.FromCIDR == "" || c.hostCollector.ToCIDR == "" {
		return fmt.Errorf("fromCIDR and toCIDR must be provided")
	}

	if c.hostCollector.FromCIDR == c.hostCollector.ToCIDR {
		return fmt.Errorf("fromCIDR and toCIDR must be different")
	}

	if _, _, err := net.ParseCIDR(c.hostCollector.FromCIDR); err != nil {
		return fmt.Errorf("%s is not a valid cidr: %w", c.hostCollector.FromCIDR, err)
	}

	if _, _, err := net.ParseCIDR(c.hostCollector.ToCIDR); err != nil {
		return fmt.Errorf("%s is not a valid cidr: %w", c.hostCollector.ToCIDR, err)
	}

	return nil
}

// Collect collects the network namespace connectivity information. This
// function expects both the from and to CIDRs to be provided and different
// from each other.
func (c *CollectHostNetworkNamespaceConnectivity) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	if err := c.validateCIDRs(); err != nil {
		return nil, err
	}

	result := &NetworkNamespaceConnectivityInfo{
		FromCIDR: c.hostCollector.FromCIDR,
		ToCIDR:   c.hostCollector.ToCIDR,
	}

	opts := []namespaces.Option{namespaces.WithLogf(result.Output.Printf)}

	// if user has chosen to use a specific port, use it.
	if c.hostCollector.Port != 0 {
		opts = append(opts, namespaces.WithPort(c.hostCollector.Port))
	}

	// if user has chosen to use a specific timeout and it is valid, use it.
	if c.hostCollector.Timeout != "" {
		timeout, err := time.ParseDuration(c.hostCollector.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout %s", c.hostCollector.Timeout)
		}
		result.Output.Printf("using user provided timeout of %q", c.hostCollector.Timeout)
		opts = append(opts, namespaces.WithTimeout(timeout))
	}

	fromNS, err := namespaces.NewNamespacePinger("from", c.hostCollector.FromCIDR, opts...)
	if err != nil {
		result.Errors.ToCIDRCreation = err.Error()
		return c.marshal(result)
	}
	defer fromNS.Close()

	toNS, err := namespaces.NewNamespacePinger("to", c.hostCollector.ToCIDR, opts...)
	if err != nil {
		result.Errors.FromCIDRCreation = err.Error()
		return c.marshal(result)
	}
	defer toNS.Close()

	udpErrors, tcpErrors := make(chan error), make(chan error)
	toNS.StartUDPEchoServer(udpErrors)
	toNS.StartTCPEchoServer(tcpErrors)

	success := true
	if err := fromNS.PingUDP(toNS.InternalIP); err != nil {
		result.Errors.UDPClient = err.Error()
		success = false
	}

	if err := <-udpErrors; err != nil {
		result.Errors.UDPServer = err.Error()
		success = false
	}

	if err := fromNS.PingTCP(toNS.InternalIP); err != nil {
		result.Errors.TCPClient = err.Error()
		success = false
	}

	if err := <-tcpErrors; err != nil {
		result.Errors.TCPServer = err.Error()
		success = false
	}

	result.Success = success
	result.Output.Printf("network namespace connectivity test finished")
	return c.marshal(result)
}
