//go:build linux

package namespaces

import (
	"fmt"

	"github.com/vishvananda/netlink"
)

// InterfacePair represents a pair of virtual ethernets that are connected to
// each other. these are used to connect a network namespace to the outside
// world.
type InterfacePair struct {
	nethandler NetlinkHandler
	prefix     string
	in         netlink.Link
	out        netlink.Link
	cfg        Configuration
}

// SetExternalIP assigns an ip address to the interface living in the default
// namespace (outside interface).
func (p *InterfacePair) SetExternalIP(outaddr string) error {
	addr, err := p.nethandler.ParseAddr(outaddr)
	if err != nil {
		return fmt.Errorf("error parsing ip: %w", err)
	}

	if err := p.nethandler.AddrAdd(p.out, addr); err != nil {
		return fmt.Errorf("error assigning ip: %w", err)
	}

	if err := p.nethandler.LinkSetUp(p.out); err != nil {
		return fmt.Errorf("error bringing up: %w", err)
	}
	return nil
}

// Close deletes the interface pair. by deleting one of the interfaces, the
// other is deleted as well.
func (p *InterfacePair) Close() error {
	if err := p.nethandler.LinkDel(p.out); err != nil {
		return fmt.Errorf("error deleting veth pair: %w", err)
	}
	return nil
}

// Setup sets up the interface pair. this function will create the veth pair
// and bring the interfaces up.
func (p *InterfacePair) Setup() (err error) {
	in := fmt.Sprintf("%s-in", p.prefix)
	out := fmt.Sprintf("%s-out", p.prefix)
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: in},
		PeerName:  out,
	}

	p.cfg.Logf("creating interface pair %q and %q", in, out)
	if err := p.nethandler.LinkAdd(veth); err != nil {
		return fmt.Errorf("error creating veth pair: %w", err)
	}

	if p.in, err = p.nethandler.LinkByName(in); err != nil {
		return fmt.Errorf("error finding %s: %w", in, err)
	}

	if p.out, err = p.nethandler.LinkByName(out); err != nil {
		return fmt.Errorf("error finding %s: %w", out, err)
	}
	return nil
}

// NewInterfacePair creates a pair of connected virtual ethernets. interfaces
// are named `prefix-in` and `prefix-out`.
func NewInterfacePair(prefix string, options ...Option) *InterfacePair {
	config := NewConfiguration(options...)
	return &InterfacePair{
		nethandler: NetlinkHandle{},
		prefix:     prefix,
		cfg:        config,
	}
}
