package collect

import (
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type HostCollector interface {
	Title() string
	IsExcluded() (bool, error)
	Collect(progressChan chan<- interface{}) (map[string][]byte, error)
}

func GetHostCollector(collector *troubleshootv1beta2.HostCollect, bundlePath string) (HostCollector, bool) {
	switch {
	case collector.CPU != nil:
		return &CollectHostCPU{collector.CPU, bundlePath}, true
	case collector.Memory != nil:
		return &CollectHostMemory{collector.Memory, bundlePath}, true
	case collector.TCPLoadBalancer != nil:
		return &CollectHostTCPLoadBalancer{collector.TCPLoadBalancer, bundlePath}, true
	case collector.HTTPLoadBalancer != nil:
		return &CollectHostHTTPLoadBalancer{collector.HTTPLoadBalancer, bundlePath}, true
	case collector.DiskUsage != nil:
		return &CollectHostDiskUsage{collector.DiskUsage, bundlePath}, true
	case collector.TCPPortStatus != nil:
		return &CollectHostTCPPortStatus{collector.TCPPortStatus, bundlePath}, true
	case collector.UDPPortStatus != nil:
		return &CollectHostUDPPortStatus{collector.UDPPortStatus, bundlePath}, true
	case collector.HTTP != nil:
		return &CollectHostHTTP{collector.HTTP, bundlePath}, true
	case collector.Time != nil:
		return &CollectHostTime{collector.Time, bundlePath}, true
	case collector.BlockDevices != nil:
		return &CollectHostBlockDevices{collector.BlockDevices, bundlePath}, true
	case collector.SystemPackages != nil:
		return &CollectHostSystemPackages{collector.SystemPackages, bundlePath}, true
	case collector.KernelModules != nil:
		return &CollectHostKernelModules{
			hostCollector: collector.KernelModules,
			BundlePath:    bundlePath,
			loadable:      kernelModulesLoadable{},
			loaded:        kernelModulesLoaded{},
		}, true
	case collector.TCPConnect != nil:
		return &CollectHostTCPConnect{collector.TCPConnect, bundlePath}, true
	case collector.IPV4Interfaces != nil:
		return &CollectHostIPV4Interfaces{collector.IPV4Interfaces, bundlePath}, true
	case collector.SubnetAvailable != nil:
		return &CollectHostSubnetAvailable{collector.SubnetAvailable, bundlePath}, true
	case collector.FilesystemPerformance != nil:
		return &CollectHostFilesystemPerformance{collector.FilesystemPerformance, bundlePath}, true
	case collector.Certificate != nil:
		return &CollectHostCertificate{collector.Certificate, bundlePath}, true
	case collector.CertificatesCollection != nil:
		return &CollectHostCertificatesCollection{collector.CertificatesCollection, bundlePath}, true
	case collector.HostServices != nil:
		return &CollectHostServices{collector.HostServices, bundlePath}, true
	case collector.HostOS != nil:
		return &CollectHostOS{collector.HostOS, bundlePath}, true
	case collector.HostRun != nil:
		return &CollectHostRun{collector.HostRun, bundlePath}, true
	case collector.HostCopy != nil:
		return &CollectHostCopy{collector.HostCopy, bundlePath}, true
	case collector.HostKernelConfigs != nil:
		return &CollectHostKernelConfigs{collector.HostKernelConfigs, bundlePath}, true
	case collector.HostJournald != nil:
		return &CollectHostJournald{collector.HostJournald, bundlePath}, true
	case collector.HostCGroups != nil:
		return &CollectHostCGroups{collector.HostCGroups, bundlePath}, true
	default:
		return nil, false
	}
}

func hostCollectorTitleOrDefault(meta troubleshootv1beta2.HostCollectorMeta, defaultTitle string) string {
	if meta.CollectorName != "" {
		return meta.CollectorName
	}
	return defaultTitle
}
