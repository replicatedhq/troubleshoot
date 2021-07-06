package collect

import (
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type HostCollector interface {
	Title() string
	IsExcluded() (bool, error)
	Collect(progressChan chan<- interface{}) (map[string][]byte, error)
}

func GetHostCollector(collector *troubleshootv1beta2.HostCollect) (HostCollector, bool) {
	switch {
	case collector.CPU != nil:
		return &CollectHostCPU{collector.CPU}, true
	case collector.Memory != nil:
		return &CollectHostMemory{collector.Memory}, true
	case collector.TCPLoadBalancer != nil:
		return &CollectHostTCPLoadBalancer{collector.TCPLoadBalancer}, true
	case collector.HTTPLoadBalancer != nil:
		return &CollectHostHTTPLoadBalancer{collector.HTTPLoadBalancer}, true
	case collector.DiskUsage != nil:
		return &CollectHostDiskUsage{collector.DiskUsage}, true
	case collector.TCPPortStatus != nil:
		return &CollectHostTCPPortStatus{collector.TCPPortStatus}, true
	case collector.HTTP != nil:
		return &CollectHostHTTP{collector.HTTP}, true
	case collector.Time != nil:
		return &CollectHostTime{collector.Time}, true
	case collector.BlockDevices != nil:
		return &CollectHostBlockDevices{collector.BlockDevices}, true
	case collector.KernelModules != nil:
		return &CollectHostKernelModules{collector.KernelModules}, true
	case collector.TCPConnect != nil:
		return &CollectHostTCPConnect{collector.TCPConnect}, true
	case collector.IPV4Interfaces != nil:
		return &CollectHostIPV4Interfaces{collector.IPV4Interfaces}, true
	case collector.FilesystemPerformance != nil:
		return &CollectHostFilesystemPerformance{collector.FilesystemPerformance}, true
	case collector.Certificate != nil:
		return &CollectHostCertificate{collector.Certificate}, true
	case collector.HostServices != nil:
		return &CollectHostServices{collector.HostServices}, true
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
