package discovery

import (
	"k8s.io/client-go/discovery"
)

// HasResource takes an api version and a kind of a resource and checks if the resource
// is supported by the k8s api server.
func HasResource(dc discovery.DiscoveryInterface, apiVersion, kind string) (bool, error) {
	_, apiLists, err := dc.ServerGroupsAndResources()
	if err != nil {
		return false, err
	}
	// Compare the resource api version and kind and find the resource.
	for _, apiList := range apiLists {
		if apiList.GroupVersion == apiVersion {
			for _, r := range apiList.APIResources {
				if r.Kind == kind {
					return true, nil
				}
			}
		}
	}
	return false, nil
}
