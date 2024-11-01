//go:build linux

package namespaces

import (
	"fmt"
	"net"
	"runtime"
	"sync"
	"syscall"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// NetworkNamespace represents a network namespace.
type NetworkNamespace struct {
	nethandler NetlinkHandler
	nshandler  NamespaceHandler
	handle     netns.NsHandle
	mutex      sync.Mutex
	origins    map[int]netns.NsHandle
	name       string
	cfg        Configuration
}

// Close closes and deletes the network namespace.
func (n *NetworkNamespace) Close() error {
	if err := n.handle.Close(); err != nil {
		return fmt.Errorf("error closing namespace: %w", err)
	}
	if err := n.nshandler.DeleteNamed(n.name); err != nil {
		return fmt.Errorf("error deleting namespace: %w", err)
	}
	return nil
}

// AttachInterface attaches the the provided interface into the namespace. This
// function does not bring the interface up.
func (n *NetworkNamespace) AttachInterface(ifname string) error {
	n.cfg.Logf("attaching interface %q to namespace %q", ifname, n.name)
	iface, err := n.nethandler.LinkByName(ifname)
	if err != nil {
		return fmt.Errorf("error finding interface: %w", err)
	}

	// put the `in` interface into the namespace.
	if err := n.nethandler.LinkSetNsFd(iface, int(n.handle)); err != nil {
		return fmt.Errorf("error moving peer into namespace: %w", err)
	}

	return nil
}

// Leave makes the thread leave the namespace. This function returns the thread
// to the previous namespace. Leaves() can't be called without Joining first.
// This function unlocks the current OS thread so it can be used again by
// multiple goroutines.
func (n *NetworkNamespace) Leave() error {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	var origin netns.NsHandle
	var ok bool

	threadID := syscall.Gettid()
	if origin, ok = n.origins[threadID]; !ok {
		return fmt.Errorf("error leaving namespace: namespace not joined")
	}

	if err := n.nshandler.Set(origin); err != nil {
		return fmt.Errorf("error switching to original namespace: %w", err)
	}

	if err := origin.Close(); err != nil {
		return fmt.Errorf("error closing original namespace: %w", err)
	}

	delete(n.origins, threadID)
	runtime.UnlockOSThread()
	return nil
}

// Join makes the thread join the namespace. The current thread is saved in the
// origin field. Callers are responsible for calling Laeave() once they are
// done. This namespace can only be joined once and this is by design. You need
// to Leave() before Joining again. The current OS thread will be locked to the
// namespace.
func (n *NetworkNamespace) Join() (err error) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	runtime.LockOSThread()
	defer func() {
		if err != nil {
			runtime.UnlockOSThread()
		}
	}()

	threadID := syscall.Gettid()
	if _, ok := n.origins[threadID]; ok {
		return fmt.Errorf("error joining namespace: namespace already joined")
	}

	origin, err := n.nshandler.Get()
	if err != nil {
		return fmt.Errorf("error getting current namespace: %w", err)
	}

	if err := n.nshandler.Set(n.handle); err != nil {
		return fmt.Errorf("error switching to the namespace: %w", err)
	}

	n.origins[threadID] = origin
	return nil
}

// SetInterfaceIP sets the ip address for the provided interface.
func (n *NetworkNamespace) SetInterfaceIP(ifname, ipaddr string) error {
	addr, err := n.nethandler.ParseAddr(ipaddr)
	if err != nil {
		return fmt.Errorf("error parsing ip: %w", err)
	}

	// this function will be executed inside the namespace.
	fn := func() error {
		iface, err := n.nethandler.LinkByName(ifname)
		if err != nil {
			return err
		}
		return n.nethandler.AddrAdd(iface, addr)
	}

	if err := n.Run(fn); err != nil {
		return fmt.Errorf("error setting interface ip: %w", err)
	}

	return nil
}

// BringInterfaceUp brings the provided interface up inside the namespace.
func (n *NetworkNamespace) BringInterfaceUp(ifname string) error {
	fn := func() error {
		iface, err := n.nethandler.LinkByName(ifname)
		if err != nil {
			return err
		}
		return n.nethandler.LinkSetUp(iface)
	}

	if err := n.Run(fn); err != nil {
		return fmt.Errorf("error bringing interface up: %w", err)
	}

	return nil
}

// SetDefaultGateway sets the default gateway for the namespace.
func (n *NetworkNamespace) SetDefaultGateway(addr string) error {
	n.cfg.Logf("setting default gateway %q for namespace %q", addr, n.name)
	gw := net.ParseIP(addr)
	if gw == nil {
		return fmt.Errorf("error parsing invalid gateway: %s", addr)
	}

	if err := n.Run(
		func() error {
			route := netlink.Route{Gw: gw}
			return n.nethandler.RouteAdd(&route)
		},
	); err != nil {
		return fmt.Errorf("error setting default gateway: %w", err)
	}

	return nil
}

// Run runs the provided function inside the namespace. Restores the original
// namespace once the function has finished.
func (n *NetworkNamespace) Run(f func() error) (err error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var origin netns.NsHandle
	if origin, err = n.nshandler.Get(); err != nil {
		return fmt.Errorf("error getting current namespace: %w", err)
	}

	defer func() {
		err = WrapIfFail("error closing namespace", err, origin.Close)
	}()

	if err := n.nshandler.Set(n.handle); err != nil {
		return fmt.Errorf("error switching to namespace: %w", err)
	}

	defer func() {
		setter := func() error { return n.nshandler.Set(origin) }
		err = WrapIfFail("error exiting namespace", err, setter)
	}()

	return f()
}

func (n *NetworkNamespace) Setup() (err error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var origin netns.NsHandle
	if origin, err = n.nshandler.Get(); err != nil {
		return fmt.Errorf("error getting current namespace: %w", err)
	}

	defer func() {
		err = WrapIfFail("error closing original namespace", err, origin.Close)
	}()

	var handle netns.NsHandle
	if handle, err = n.nshandler.NewNamed(n.name); err != nil {
		return fmt.Errorf("error creating network namespace: %w", err)
	}

	defer func() {
		setter := func() error { return n.nshandler.Set(origin) }
		err = WrapIfFail("error exiting namespace", err, setter)
	}()

	n.handle = handle
	return nil
}

// NewNetworkNamespace creates a new network namespace. once the namespace is
// created this function restores the thread to the original namespace.
func NewNetworkNamespace(name string, options ...Option) *NetworkNamespace {
	return &NetworkNamespace{
		nethandler: NetlinkHandle{},
		nshandler:  NamespaceHandle{},
		name:       name,
		cfg:        NewConfiguration(options...),
		origins:    map[int]netns.NsHandle{},
	}
}
