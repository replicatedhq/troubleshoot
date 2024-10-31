package namespaces

import "github.com/vishvananda/netlink"

// NetlinkHandler is an interface that represents the netlink functions that
// we need to mock. This only exists for test purposes.
type NetlinkHandler interface {
	ParseAddr(string) (*netlink.Addr, error)
	AddrAdd(netlink.Link, *netlink.Addr) error
	LinkSetUp(netlink.Link) error
	LinkDel(netlink.Link) error
	LinkAdd(netlink.Link) error
	LinkByName(string) (netlink.Link, error)
	LinkSetNsFd(netlink.Link, int) error
	RouteAdd(*netlink.Route) error
}

// NetlinkHandle is a struct that exists solely for the purpose of mocking
// netlink functions on tests.
type NetlinkHandle struct{}

// ParseAddr calls netlink.ParseAddr.
func (n NetlinkHandle) ParseAddr(s string) (*netlink.Addr, error) {
	return netlink.ParseAddr(s)
}

// AddrAdd calls netlink.AddrAdd.
func (n NetlinkHandle) AddrAdd(l netlink.Link, a *netlink.Addr) error {
	return netlink.AddrAdd(l, a)
}

// LinkSetUp calls netlink.LinkSetUp.
func (n NetlinkHandle) LinkSetUp(link netlink.Link) error {
	return netlink.LinkSetUp(link)
}

// LinkDel calls netlink.LinkDel.
func (n NetlinkHandle) LinkDel(link netlink.Link) error {
	return netlink.LinkDel(link)
}

// LinkAdd calls netlink.LinkAdd.
func (n NetlinkHandle) LinkAdd(link netlink.Link) error {
	return netlink.LinkAdd(link)
}

// LinkByName calls netlink.LinkByName.
func (n NetlinkHandle) LinkByName(name string) (netlink.Link, error) {
	return netlink.LinkByName(name)
}

// LinkSetNsFd calls netlink.LinkSetNsFd.
func (n NetlinkHandle) LinkSetNsFd(link netlink.Link, fd int) error {
	return netlink.LinkSetNsFd(link, fd)
}

// RouteAdd calls netlink.RouteAdd.
func (n NetlinkHandle) RouteAdd(route *netlink.Route) error {
	return netlink.RouteAdd(route)
}
