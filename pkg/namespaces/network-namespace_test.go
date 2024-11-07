//go:build linux

package namespaces

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

type MockNamespace struct {
	mock.Mock
}

func (m *MockNamespace) DeleteNamed(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *MockNamespace) Set(ns netns.NsHandle) error {
	args := m.Called(ns)
	return args.Error(0)
}

func (m *MockNamespace) Get() (netns.NsHandle, error) {
	args := m.Called()
	return args.Get(0).(netns.NsHandle), args.Error(1)
}

func (m *MockNamespace) NewNamed(name string) (netns.NsHandle, error) {
	args := m.Called(name)
	return args.Get(0).(netns.NsHandle), args.Error(1)
}

func TestNetworkNamespaceAttachInterface(t *testing.T) {
	mockNetlink := &MockNetlink{}

	ns := NewNetworkNamespace("test")
	ns.nethandler = mockNetlink

	mockNetlink.On("LinkByName", "test-in").Return(&netlink.Device{}, nil)
	mockNetlink.On("LinkSetNsFd", mock.Anything, mock.Anything).Return(nil)

	err := ns.AttachInterface("test-in")
	assert.NoError(t, err, "error attaching interface")

	mockNetlink.AssertCalled(t, "LinkByName", "test-in")
	mockNetlink.AssertCalled(t, "LinkSetNsFd", mock.Anything, mock.Anything)
}

func TestNetworkNamespaceJoin(t *testing.T) {
	mockNetlink := &MockNetlink{}
	mockNamespace := &MockNamespace{}

	ns := NewNetworkNamespace("test")
	ns.nethandler = mockNetlink
	ns.nshandler = mockNamespace

	mockNamespace.On("Get").Return(netns.NsHandle(0), nil)
	mockNamespace.On("Set", mock.Anything).Return(nil)

	err := ns.Join()
	assert.NoError(t, err, "error joining namespace")
	assert.NotEmpty(t, ns.origins)

	mockNamespace.AssertCalled(t, "Get")
	mockNamespace.AssertCalled(t, "Set", mock.Anything)
}

func TestNetworkNamespaceLeave(t *testing.T) {
	mockNetlink := &MockNetlink{}
	mockNamespace := &MockNamespace{}

	ns := NewNetworkNamespace("test")
	ns.nethandler = mockNetlink
	ns.nshandler = mockNamespace

	mockNamespace.On("Get").Return(netns.NsHandle(1), nil)
	mockNamespace.On("Set", mock.Anything).Return(nil)

	err := ns.Join()
	assert.NoError(t, err, "error joining namespace")
	assert.NotEmpty(t, ns.origins)

	err = ns.Leave()
	assert.NoError(t, err, "error leaving namespace")
	assert.Empty(t, ns.origins)

	mockNamespace.AssertCalled(t, "Get")
	mockNamespace.AssertCalled(t, "Set", mock.Anything)
}

func TestNetworkNamespaceSetInterfaceIP(t *testing.T) {
	mockNetlink := &MockNetlink{}
	mockNamespace := &MockNamespace{}

	ns := NewNetworkNamespace("test")
	ns.nethandler = mockNetlink
	ns.nshandler = mockNamespace

	mockNetlink.On("ParseAddr", "10.0.0.1").Return(&netlink.Addr{}, nil)
	mockNetlink.On("LinkByName", "test-in").Return(&netlink.Device{}, nil)
	mockNetlink.On("AddrAdd", mock.Anything, mock.Anything).Return(nil)

	mockNamespace.On("Get").Return(netns.NsHandle(0), nil)
	mockNamespace.On("Set", mock.Anything).Return(nil)

	err := ns.SetInterfaceIP("test-in", "10.0.0.1")
	assert.NoError(t, err, "error setting interface ip")

	mockNetlink.AssertCalled(t, "ParseAddr", "10.0.0.1")
	mockNetlink.AssertCalled(t, "LinkByName", "test-in")
	mockNetlink.AssertCalled(t, "AddrAdd", mock.Anything, mock.Anything)
	mockNamespace.AssertCalled(t, "Get")
	mockNamespace.AssertCalled(t, "Set", mock.Anything)
}

func TestNetworkNamespaceBringInterfaceUp(t *testing.T) {
	mockNetlink := &MockNetlink{}
	mockNamespace := &MockNamespace{}

	ns := NewNetworkNamespace("test")
	ns.nethandler = mockNetlink
	ns.nshandler = mockNamespace

	mockNetlink.On("LinkByName", "test-in").Return(&netlink.Device{}, nil)
	mockNetlink.On("LinkSetUp", mock.Anything).Return(nil)

	fd, err := os.CreateTemp("", "test-in")
	assert.NoError(t, err, "error creating temporary file")
	defer os.Remove(fd.Name())

	mockNamespace.On("Get").Return(netns.NsHandle(fd.Fd()), nil)
	mockNamespace.On("Set", mock.Anything).Return(nil)

	err = ns.BringInterfaceUp("test-in")
	assert.NoError(t, err, "error bringing interface up")

	mockNetlink.AssertCalled(t, "LinkByName", "test-in")
	mockNetlink.AssertCalled(t, "LinkSetUp", mock.Anything)
	mockNamespace.AssertCalled(t, "Get")
	mockNamespace.AssertCalled(t, "Set", mock.Anything)
}

func TestNetworkManagerSetDefaultGateway(t *testing.T) {
	mockNetlink := &MockNetlink{}
	mockNamespace := &MockNamespace{}

	ns := NewNetworkNamespace("test")
	ns.nethandler = mockNetlink
	ns.nshandler = mockNamespace

	fd, err := os.CreateTemp("", "test-in")
	assert.NoError(t, err, "error creating temporary file")
	defer os.Remove(fd.Name())

	mockNetlink.On("RouteAdd", mock.Anything).Return(nil)
	mockNamespace.On("Get").Return(netns.NsHandle(fd.Fd()), nil)
	mockNamespace.On("Set", mock.Anything).Return(nil)

	err = ns.SetDefaultGateway("10.0.0.1")
	assert.NoError(t, err, "error setting default gateway")

	mockNetlink.AssertCalled(t, "RouteAdd", mock.Anything)
	mockNamespace.AssertCalled(t, "Get")
	mockNamespace.AssertCalled(t, "Set", mock.Anything)
}

func TestNetworkNamespaceRun(t *testing.T) {
	mockNamespace := &MockNamespace{}

	ns := NewNetworkNamespace("test")
	ns.nshandler = mockNamespace

	fd, err := os.CreateTemp("", "test-in")
	assert.NoError(t, err, "error creating temporary file")
	defer os.Remove(fd.Name())

	mockNamespace.On("Get").Return(netns.NsHandle(fd.Fd()), nil)
	mockNamespace.On("Set", mock.Anything).Return(nil)

	err = ns.Run(func() error { return nil })
	assert.NoError(t, err, "error running function")

	err = ns.Run(func() error { return fmt.Errorf("test error") })
	assert.Error(t, err, "error running function")

	mockNamespace.AssertCalled(t, "Get")
	mockNamespace.AssertCalled(t, "Set", mock.Anything)
}

func TestNetworkNamespaceSetup(t *testing.T) {
	// succeeds to create the namespace.
	mockNamespace := &MockNamespace{}

	ns := NewNetworkNamespace("test")
	ns.nshandler = mockNamespace

	fd, err := os.CreateTemp("", "test-in")
	assert.NoError(t, err, "error creating temporary file")
	defer os.Remove(fd.Name())

	fd2, err := os.CreateTemp("", "test-in")
	assert.NoError(t, err, "error creating temporary file")
	defer os.Remove(fd2.Name())

	mockNamespace.On("NewNamed", "test").Return(netns.NsHandle(fd2.Fd()), nil)
	mockNamespace.On("Get").Return(netns.NsHandle(fd.Fd()), nil)
	mockNamespace.On("Set", mock.Anything).Return(nil)

	err = ns.Setup()
	assert.NoError(t, err, "error setting up namespace")

	mockNamespace.AssertCalled(t, "NewNamed", "test")
	mockNamespace.AssertCalled(t, "Get")
	mockNamespace.AssertCalled(t, "Set", mock.Anything)

	// os fails to create the namespace.
	mockNamespace = &MockNamespace{}
	ns = NewNetworkNamespace("test")
	ns.nshandler = mockNamespace

	mockNamespace.On("NewNamed", "test").Return(netns.NsHandle(0), fmt.Errorf("test error"))
	mockNamespace.On("Get").Return(netns.NsHandle(fd.Fd()), nil)
	mockNamespace.On("Set", mock.Anything).Return(nil)

	err = ns.Setup()
	assert.Error(t, err, "expected error setting up namespace")

	mockNamespace.AssertCalled(t, "NewNamed", "test")

	// fail to open the default namespace.
	mockNamespace = &MockNamespace{}
	ns = NewNetworkNamespace("test")
	ns.nshandler = mockNamespace

	mockNamespace.On("Get").Return(netns.NsHandle(0), fmt.Errorf("test error"))

	err = ns.Setup()
	assert.Error(t, err, "expected error setting up namespace")

	mockNamespace.AssertCalled(t, "Get")
}
