//go:build linux

package namespaces

import (
	"fmt"
	"net"
	"strconv"
	"time"
)

type NamespacePinger struct {
	*ManagedNetworkNamespace
	cfg Configuration
}

// PingUDP communicates with the provided IP address from within the namespace.
// This functions sends an UDP packet and expects to receive an echo back.
func (n *NamespacePinger) PingUDP(dst net.IP) error {
	n.cfg.Logf("reaching to %q from %q with udp", dst, n.InternalIP)
	pinger := func() error {
		addr := &net.UDPAddr{IP: dst, Port: n.cfg.Port}
		conn, err := net.DialUDP("udp", nil, addr)
		if err != nil {
			return fmt.Errorf("error dialing udp: %w", err)
		}
		defer conn.Close()

		if _, err = conn.Write([]byte("echo")); err != nil {
			return fmt.Errorf("error writing to udp socket: %w", err)
		}

		deadline := time.Now().Add(n.cfg.Timeout)
		if err := conn.SetReadDeadline(deadline); err != nil {
			return fmt.Errorf("error setting udp read deadline: %w", err)
		}

		// XXX: review this buffer size and validation.
		buffer := make([]byte, 6)
		if _, _, err = conn.ReadFromUDP(buffer); err != nil {
			return fmt.Errorf("error reading from udp socket: %w", err)
		}

		return nil
	}

	return n.Run(pinger)
}

// PingTCP communicates with the provided IP address from within the namespace.
// This functions sends an TCP packet and expects to receive an echo back.
func (n *NamespacePinger) PingTCP(dst net.IP) error {
	n.cfg.Logf("reaching to %q from %q with tcp", dst, n.InternalIP)
	pinger := func() error {
		addr := net.JoinHostPort(dst.String(), strconv.Itoa(n.cfg.Port))
		conn, err := net.DialTimeout("tcp", addr, n.cfg.Timeout)
		if err != nil {
			return fmt.Errorf("error dialing tcp: %w", err)
		}
		defer conn.Close()

		if _, err = conn.Write([]byte("echo")); err != nil {
			return fmt.Errorf("error writing to tcp socket: %w", err)
		}

		// XXX: review this buffer size and validation.
		buffer := make([]byte, 6)
		if _, err = conn.Read(buffer); err != nil {
			return fmt.Errorf("error reading from tcp socket: %w", err)
		}

		return nil
	}
	return n.Run(pinger)
}

// StartTCPEchoServer is a helper to run startTCPEchoServer inside a goroutine.
// This function blocks until the server is ready to receive packets or failed
// to start. Errors are sent to the provided channel.
func (n *NamespacePinger) StartTCPEchoServer(errors chan error) {
	ready := make(chan struct{})
	go func() {
		errors <- n.startTCPEchoServer(ready)
	}()
	<-ready
}

// startTCPEchoServer starts a tcp server inside the namespace. The thread
// running the goroutine that process this call will be moved to the namespace.
// This echo servers just returns "echo" as a response. Once one packet is
// received, the server ends. Callers must wait until the ready channel is
// closed before they can start sending packets.
func (n *NamespacePinger) startTCPEchoServer(ready chan struct{}) (err error) {
	addr := net.JoinHostPort(n.InternalIP.String(), strconv.Itoa(n.cfg.Port))
	n.cfg.Logf("starting tcp echo server on namespace %q(%q)", n.name, addr)

	if err = n.Join(); err != nil {
		close(ready)
		return fmt.Errorf("error joining namespace: %w", err)
	}

	defer func() {
		err = WrapIfFail("error leaving namespace", err, n.Leave)
	}()

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		close(ready)
		return fmt.Errorf("error starting tcp server: %w", err)
	}
	defer listener.Close()

	deadline := time.Now().Add(n.cfg.Timeout)
	tcplistener := listener.(*net.TCPListener)
	if err = tcplistener.SetDeadline(deadline); err != nil {
		close(ready)
		return fmt.Errorf("error setting tcp listener deadline: %w", err)
	}

	go func() {
		// XXX: here be dragons. we can't signalize we are ready until
		// the call to read is done so we artificially sleep for a bit
		// here.
		time.Sleep(100 * time.Millisecond)
		close(ready)
	}()

	var conn net.Conn
	if conn, err = listener.Accept(); err != nil {
		return fmt.Errorf("error accepting connection: %w", err)
	}

	n.cfg.Logf("received tcp packet on %q from %q", n.InternalIP, conn.RemoteAddr())

	if _, err = conn.Write([]byte("echo\n")); err != nil {
		return fmt.Errorf("error writing to tcp socket: %w", err)
	}

	return nil
}

// StartUDPEchoServer is a helper to run startUDPTCPEchoServer inside a goroutine.
// This function blocks until the server is ready to receive packets or failed
// to start. Errors are sent to the provided channel.
func (n *NamespacePinger) StartUDPEchoServer(errors chan error) {
	ready := make(chan struct{})
	go func() {
		errors <- n.startUDPEchoServer(ready)
	}()
	<-ready
}

// startUDPEchoServers starts an udp server inside the namespace. The thread
// running the goroutine that process this call will be moved to the namespace.
// This echo servers just returns "echo" as a response. Once one packet is
// received, the server ends. Callers must wait until the ready channel is
// closed before they can start sending packets.
func (n *NamespacePinger) startUDPEchoServer(ready chan struct{}) (err error) {
	addr := net.UDPAddr{Port: n.cfg.Port, IP: n.InternalIP}
	n.cfg.Logf("starting udp echo server on namespace %q(%q)", n.name, addr.String())

	if err = n.Join(); err != nil {
		close(ready)
		return fmt.Errorf("error joining namespace: %w", err)
	}

	defer func() {
		err = WrapIfFail("error leaving namespace", err, n.Leave)
	}()

	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		close(ready)
		return fmt.Errorf("error starting udp server: %w", err)
	}
	defer conn.Close()

	deadline := time.Now().Add(n.cfg.Timeout)
	if err = conn.SetDeadline(deadline); err != nil {
		close(ready)
		return fmt.Errorf("error setting udp listener deadline: %w", err)
	}

	go func() {
		// XXX: here be dragons. we can't signalize we are ready until
		// the call to read is done so we artificially sleep for a bit
		// here.
		time.Sleep(100 * time.Millisecond)
		close(ready)
	}()

	var source *net.UDPAddr
	var buffer = make([]byte, 1024)
	if _, source, err = conn.ReadFromUDP(buffer); err != nil {
		return fmt.Errorf("error reading from udp socket: %w", err)
	}

	n.cfg.Logf("received udp packet on %q from %q", n.InternalIP, source.AddrPort())

	if _, err = conn.WriteToUDP([]byte("echo"), source); err != nil {
		return fmt.Errorf("error writing to udp socket: %w", err)
	}

	return nil
}

func NewNamespacePinger(name, cidraddr string, options ...Option) (*NamespacePinger, error) {
	config := NewConfiguration(options...)

	namespace, err := NewManagedNetworkNamespace(name, cidraddr, options...)
	if err != nil {
		return nil, fmt.Errorf("error creating network namespace: %w", err)
	}
	return &NamespacePinger{
		ManagedNetworkNamespace: namespace,
		cfg:                     config,
	}, nil
}
