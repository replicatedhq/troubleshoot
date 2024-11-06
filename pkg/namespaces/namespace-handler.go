//go:build linux

package namespaces

import "github.com/vishvananda/netns"

// NamespaceHandler is an interface that represents the netns functions that
// we need to mock. This only exists for test purposes.
type NamespaceHandler interface {
	DeleteNamed(string) error
	Set(netns.NsHandle) error
	Get() (netns.NsHandle, error)
	NewNamed(string) (netns.NsHandle, error)
}

// NamespaceHandle is a struct that exists solely for the purpose of mocking
// netns functions on tests. It just wraps calls to the netns package.
type NamespaceHandle struct{}

// DeleteNamed calls netns.DeleteNamed.
func (n NamespaceHandle) DeleteNamed(name string) error {
	return netns.DeleteNamed(name)
}

// Set calls netns.Set.
func (n NamespaceHandle) Set(ns netns.NsHandle) error {
	return netns.Set(ns)
}

// Get calls netns.Get.
func (n NamespaceHandle) Get() (netns.NsHandle, error) {
	return netns.Get()
}

// NewNamed calls netns.NewNamed.
func (n NamespaceHandle) NewNamed(name string) (netns.NsHandle, error) {
	return netns.NewNamed(name)
}
