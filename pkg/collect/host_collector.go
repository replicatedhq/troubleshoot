package collect

import (
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
)

type HostCollector struct {
	Collect *troubleshootv1beta2.HostCollect
}

type HostCollectors []*HostCollector

func (c *HostCollector) RunCollectorSync() (result map[string][]byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.Errorf("recovered rom panic: %v", r)
		}
	}()

	if c.IsExcluded() {
		return
	}

	if c.Collect.CPU != nil {
		result, err = HostCPU(c)
	} else if c.Collect.Memory != nil {
		result, err = HostMemory(c)
	} else if c.Collect.TCPLoadBalancer != nil {
		result, err = HostTCPLoadBalancer(c)
	} else if c.Collect.HTTPLoadBalancer != nil {
		result, err = HostHTTPLoadBalancer(c)
	} else if c.Collect.DiskUsage != nil {
		result, err = HostDiskUsage(c)
	} else if c.Collect.TCPPortStatus != nil {
		result, err = HostTCPPortStatus(c)
	} else if c.Collect.HTTP != nil {
		result, err = HostHTTP(c)
	} else if c.Collect.Time != nil {
		result, err = HostTime(c)
	} else if c.Collect.BlockDevices != nil {
		result, err = HostBlockDevices(c)
	} else if c.Collect.TCPConnect != nil {
		result, err = HostTCPConnect(c)
	} else if c.Collect.IPV4Interfaces != nil {
		result, err = HostIPV4Interfaces(c)
	} else if c.Collect.FilesystemPerformance != nil {
		result, err = HostFilesystemPerformance(c)
	} else if c.Collect.Certificate != nil {
		result, err = HostCertificate(c)
	} else {
		err = errors.New("no spec found to run")
		return
	}
	if err != nil {
		return
	}

	return
}

func (c *HostCollector) IsExcluded() bool {
	exclude := multitype.BoolOrString{}
	if c.Collect.CPU != nil {
		exclude = c.Collect.CPU.Exclude
	} else if c.Collect.Memory != nil {
		exclude = c.Collect.Memory.Exclude
	} else if c.Collect.TCPLoadBalancer != nil {
		exclude = c.Collect.TCPLoadBalancer.Exclude
	} else if c.Collect.HTTPLoadBalancer != nil {
		exclude = c.Collect.HTTPLoadBalancer.Exclude
	} else if c.Collect.DiskUsage != nil {
		exclude = c.Collect.DiskUsage.Exclude
	} else if c.Collect.TCPPortStatus != nil {
		exclude = c.Collect.TCPPortStatus.Exclude
	} else if c.Collect.HTTP != nil {
		exclude = c.Collect.HTTP.Exclude
	} else if c.Collect.Time != nil {
		exclude = c.Collect.Time.Exclude
	} else if c.Collect.BlockDevices != nil {
		exclude = c.Collect.BlockDevices.Exclude
	} else if c.Collect.TCPConnect != nil {
		exclude = c.Collect.TCPConnect.Exclude
	} else if c.Collect.IPV4Interfaces != nil {
		exclude = c.Collect.IPV4Interfaces.Exclude
	} else if c.Collect.FilesystemPerformance != nil {
		exclude = c.Collect.FilesystemPerformance.Exclude
	} else if c.Collect.Certificate != nil {
		exclude = c.Collect.Certificate.Exclude
	} else {
		return true
	}

	isExcludedResult, err := isExcluded(exclude)
	if err != nil {
		return true
	}
	return isExcludedResult
}

func (c *HostCollector) GetDisplayName() string {
	return c.Collect.GetName()
}
