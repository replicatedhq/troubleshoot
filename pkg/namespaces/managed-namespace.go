//go:build linux

package namespaces

import (
	"fmt"
	"net"

	"github.com/apparentlymart/go-cidr/cidr"
)

// ManagedNetworkNamespace is a struct that helps up setting up a namespace
// with a pre-defined configuration. See NewManagedNetworkNamespace for more
// information on how the namespace is configured.
type ManagedNetworkNamespace struct {
	*NetworkNamespace
	*InterfacePair

	InternalIP net.IP
	ExternalIP net.IP
	cfg        Configuration
}

// NewManagedNetworkNamespace creates a new configured network namespace. This
// network namespace will have an interface configurwed with the first ip
// address of the provided cidr. The external interface (living in the default
// namespace) will be configured with the last ip address of the provided cidr
// and will be set as the default gateway for the namespace.
func NewManagedNetworkNamespace(name, cidraddr string, options ...Option) (*ManagedNetworkNamespace, error) {
	config := NewConfiguration(options...)
	config.Logf("creating network namespace %q with cidr %q", name, cidraddr)

	_, netaddr, err := net.ParseCIDR(cidraddr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cidr: %w", err)
	}

	// AddressRange() returns the first and the last addresses of the cidrs.
	// Those aren't useful as the first is the network address and the last
	// is the broadcast address. We need to adjust them here.
	netsize, _ := netaddr.Mask.Size()
	first, last := cidr.AddressRange(netaddr)
	first, last = cidr.Inc(first), cidr.Dec(last)
	config.Logf("network namespace %q address range: %q - %q", name, first, last)

	pair := NewInterfacePair(name, options...)
	if err := pair.Setup(); err != nil {
		return nil, fmt.Errorf("error creating interface pair: %w", err)
	}

	fulladdr := fmt.Sprintf("%s/%d", last, netsize)
	if err := pair.SetExternalIP(fulladdr); err != nil {
		pair.Close()
		return nil, fmt.Errorf("error setting external interface: %w", err)
	}

	namespace := NewNetworkNamespace(name, options...)
	if err := namespace.Setup(); err != nil {
		pair.Close()
		return nil, fmt.Errorf("error creating namespace: %w", err)
	}

	if err := namespace.AttachInterface(pair.in.Attrs().Name); err != nil {
		pair.Close()
		namespace.Close()
		return nil, fmt.Errorf("error attaching interface pair: %w", err)
	}

	fulladdr = fmt.Sprintf("%s/%d", first, netsize)
	if err := namespace.SetInterfaceIP(pair.in.Attrs().Name, fulladdr); err != nil {
		pair.Close()
		namespace.Close()
		return nil, fmt.Errorf("error setting interface ip: %w", err)
	}

	for _, ifname := range []string{"lo", pair.in.Attrs().Name} {
		if err := namespace.BringInterfaceUp(ifname); err != nil {
			pair.Close()
			namespace.Close()
			return nil, fmt.Errorf("error bringing %s interface up: %w", ifname, err)
		}
	}

	if err := namespace.SetDefaultGateway(last.String()); err != nil {
		pair.Close()
		namespace.Close()
		return nil, fmt.Errorf("error setting default gateway: %w", err)
	}

	return &ManagedNetworkNamespace{
		NetworkNamespace: namespace,
		InterfacePair:    pair,
		InternalIP:       first,
		ExternalIP:       last,
		cfg:              config,
	}, nil
}

// Close destroys both the interface pair and the namespace. Here we only need
// to worry about deleting the namespace as the veth pair will be deleted
// automatically.
func (n *ManagedNetworkNamespace) Close() error {
	if err := n.InterfacePair.Close(); err != nil {
		return fmt.Errorf("error closing interface pair: %w", err)
	}
	if err := n.NetworkNamespace.Close(); err != nil {
		return fmt.Errorf("error closing namespace: %w", err)
	}
	return nil
}
