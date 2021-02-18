package v1beta2

import (
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
)

type HostCollectorMeta struct {
	CollectorName string `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	// +optional
	Exclude multitype.BoolOrString `json:"exclude,omitempty" yaml:"exclude,omitempty"`
}

type CPU struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
}

type Memory struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
}

type TCPLoadBalancer struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
	Address           string `json:"address"`
	Port              int    `json:"port"`
	Timeout           string `json:"timeout,omitempty"`
}

type HTTPLoadBalancer struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
	Address           string `json:"address"`
	Port              int    `json:"port"`
	Path              string `json:"path"`
	Timeout           string `json:"timeout,omitempty"`
}

type TCPPortStatus struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
	Interface         string `json:"interface,omitempty"`
	Port              int    `json:"port"`
}

type Kubernetes struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
}

type IPV4Interfaces struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
}

type DiskUsage struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
	Path              string `json:"path"`
}

type HostHTTP struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
	Get               *Get  `json:"get,omitempty" yaml:"get,omitempty"`
	Post              *Post `json:"post,omitempty" yaml:"post,omitempty"`
	Put               *Put  `json:"put,omitempty" yaml:"put,omitempty"`
}

type HostTime struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
}

type HostBlockDevices struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
}

type TCPConnect struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
	Address           string `json:"address"`
	Timeout           string `json:"timeout,omitempty"`
}

type FilesystemPerformance struct {
	HostCollectorMeta  `json:",inline" yaml:",inline"`
	OperationSizeBytes uint64 `json:"operationSize,omitempty"`
	Directory          string `json:"directory,omitempty"`
	FileSize           string `json:"fileSize,omitempty"`
	Sync               bool   `json:"sync,omitempty"`
	Datasync           bool   `json:"datasync,omitempty"`
}

type HostCollect struct {
	CPU                   *CPU                   `json:"cpu,omitempty" yaml:"cpu,omitempty"`
	Memory                *Memory                `json:"memory,omitempty" yaml:"memory,omitempty"`
	TCPLoadBalancer       *TCPLoadBalancer       `json:"tcpLoadBalancer,omitempty" yaml:"tcpLoadBalancer,omitempty"`
	HTTPLoadBalancer      *HTTPLoadBalancer      `json:"httpLoadBalancer,omitempty" yaml:"httpLoadBalancer,omitempty"`
	TCPPortStatus         *TCPPortStatus         `json:"tcpPortStatus,omitempty" yaml:"tcpPortStatus,omitempty"`
	Kubernetes            *Kubernetes            `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty"`
	IPV4Interfaces        *IPV4Interfaces        `json:"ipv4Interfaces,omitempty" yaml:"ipv4Interfaces,omitempty"`
	DiskUsage             *DiskUsage             `json:"diskUsage,omitempty" yaml:"diskUsage,omitempty"`
	HTTP                  *HostHTTP              `json:"http,omitempty" yaml:"http,omitempty"`
	Time                  *HostTime              `json:"time,omitempty" yaml:"time,omitempty"`
	BlockDevices          *HostBlockDevices      `json:"blockDevices,omitempty" yaml:"blockDevices,omitempty"`
	TCPConnect            *TCPConnect            `json:"tcpConnect,omitempty" yaml:"tcpConnect,omitempty"`
	FilesystemPerformance *FilesystemPerformance `json:"filesystemPerformance" yaml:"filesystemPerformance"`
}

func (c *HostCollect) GetName() string {
	var collector string
	if c.CPU != nil {
		collector = "cpu"
	}
	if c.Memory != nil {
		collector = "memory"
	}

	if collector == "" {
		return "<none>"
	}

	return collector
}
