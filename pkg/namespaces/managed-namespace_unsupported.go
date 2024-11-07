//go:build !linux

package namespaces

import (
	"fmt"
	"net"
	"runtime"
)

type NamespacePinger struct {
	InternalIP net.IP
	ExternalIP net.IP
}

func (n *NamespacePinger) Close() error {
	return fmt.Errorf("namespaces not supported on %s platform", runtime.GOOS)
}

func (n *NamespacePinger) PingUDP(_ net.IP) error {
	return fmt.Errorf("namespaces not supported on %s platform", runtime.GOOS)
}

func (n *NamespacePinger) PingTCP(_ net.IP) error {
	return fmt.Errorf("namespaces not supported on %s platform", runtime.GOOS)
}

func (n *NamespacePinger) StartTCPEchoServer(errors chan error) {
	go func() {
		errors <- fmt.Errorf("namespaces not supported on %s platform", runtime.GOOS)
	}()
}

func (n *NamespacePinger) StartUDPEchoServer(errors chan error) {
	go func() {
		errors <- fmt.Errorf("namespaces not supported on %s platform", runtime.GOOS)
	}()
}

func NewNamespacePinger(_, _ string, _ ...Option) (*NamespacePinger, error) {
	return nil, fmt.Errorf("namespaces not supported on %s platform", runtime.GOOS)
}
