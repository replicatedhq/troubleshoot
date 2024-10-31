package namespaces

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/vishvananda/netlink"
)

// MockNetlink is a mock for netlink handler used in InterfacePair.
type MockNetlink struct {
	mock.Mock
}

// Mock methods
func (m *MockNetlink) ParseAddr(addr string) (*netlink.Addr, error) {
	args := m.Called(addr)
	return args.Get(0).(*netlink.Addr), args.Error(1)
}

func (m *MockNetlink) AddrAdd(link netlink.Link, addr *netlink.Addr) error {
	args := m.Called(link, addr)
	return args.Error(0)
}

func (m *MockNetlink) LinkSetUp(link netlink.Link) error {
	args := m.Called(link)
	return args.Error(0)
}

func (m *MockNetlink) LinkDel(link netlink.Link) error {
	args := m.Called(link)
	return args.Error(0)
}

func (m *MockNetlink) LinkAdd(link netlink.Link) error {
	args := m.Called(link)
	return args.Error(0)
}

func (m *MockNetlink) LinkByName(name string) (netlink.Link, error) {
	args := m.Called(name)
	return args.Get(0).(netlink.Link), args.Error(1)
}

func (m *MockNetlink) LinkSetNsFd(link netlink.Link, fd int) error {
	args := m.Called(link, fd)
	return args.Error(0)
}

func (m *MockNetlink) RouteAdd(route *netlink.Route) error {
	args := m.Called(route)
	return args.Error(0)
}

func TestInterfacePairSetExternalIP(t *testing.T) {
	mockNetlink := &MockNetlink{}
	pair := InterfacePair{
		prefix:     "test",
		nethandler: mockNetlink,
	}

	result := &netlink.Addr{
		IPNet: &net.IPNet{
			IP: net.ParseIP("10.0.0.1"),
		},
	}

	mockNetlink.On("ParseAddr", "10.0.0.1").Return(result, nil)
	mockNetlink.On("AddrAdd", mock.Anything, mock.Anything).Return(nil)
	mockNetlink.On("LinkSetUp", mock.Anything).Return(nil)

	err := pair.SetExternalIP("10.0.0.1")
	assert.NoError(t, err)
	mockNetlink.AssertCalled(t, "ParseAddr", "10.0.0.1")
	mockNetlink.AssertCalled(t, "AddrAdd", mock.Anything, mock.Anything)
	mockNetlink.AssertCalled(t, "LinkSetUp", mock.Anything, mock.Anything)

}

func TestInterfacePairClose(t *testing.T) {
	mockNetlink := &MockNetlink{}
	pair := InterfacePair{nethandler: mockNetlink}

	mockNetlink.On("LinkDel", mock.Anything).Return(nil)

	err := pair.Close()
	assert.NoError(t, err)
	mockNetlink.AssertCalled(t, "LinkDel", mock.Anything)
}

func TestInterfacePairSetup(t *testing.T) {
	mockNetlink := &MockNetlink{}
	pair := InterfacePair{
		prefix:     "test",
		nethandler: mockNetlink,
		cfg:        NewConfiguration(),
	}

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: "test-in"},
		PeerName:  "test-out",
	}

	mockNetlink.On("LinkAdd", veth).Return(nil)
	mockNetlink.On("LinkByName", "test-in").Return(&netlink.Device{}, nil)
	mockNetlink.On("LinkByName", "test-out").Return(&netlink.Device{}, nil)

	err := pair.Setup()
	assert.NoError(t, err)

	mockNetlink.AssertCalled(t, "LinkAdd", veth)
	mockNetlink.AssertCalled(t, "LinkByName", "test-in")
	mockNetlink.AssertCalled(t, "LinkByName", "test-out")
}
