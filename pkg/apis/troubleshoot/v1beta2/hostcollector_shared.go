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

type HostKernelModules struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
}

type TCPConnect struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
	Address           string `json:"address"`
	Timeout           string `json:"timeout,omitempty"`
}

// FilesystemPerformance benchmarks sequential write latency on a single file.
// The optional background IOPS feature attempts to mimic real-world conditions by running read and
// write workloads prior to and during benchmark execution.
type FilesystemPerformance struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
	// The size of each write operation performed while benchmarking. This does not apply to the
	// background IOPS feature if enabled, since those must be fixed at 4096.
	OperationSizeBytes uint64 `json:"operationSize,omitempty"`
	// The directory where the benchmark will create files.
	Directory string `json:"directory,omitempty"`
	// The size of the file used in the benchmark. The number of IO operations for the benchmark
	// will be FileSize / OperationSizeBytes. Accepts valid Kubernetes resource units such as Mi.
	FileSize string `json:"fileSize,omitempty"`
	// Whether to call sync on the file after each write. Does not apply to background IOPS task.
	Sync bool `json:"sync,omitempty"`
	// Whether to call datasync on the file after each write. Skipped if Sync is also true. Does not
	// apply to background IOPS task.
	Datasync bool `json:"datasync,omitempty"`
	// Total timeout, including background IOPS setup and warmup if enabled.
	Timeout string `json:"timeout,omitempty"`

	// Enable the background IOPS feature.
	EnableBackgroundIOPS bool `json:"enableBackgroundIOPS"`
	// How long to run the background IOPS read and write workloads prior to starting the benchmarks.
	BackgroundIOPSWarmupSeconds int `json:"backgroundIOPSWarmupSeconds"`
	// The target write IOPS to run while benchmarking. This is a limit and there is no guarantee
	// it will be reached. This is the total IOPS for all background write jobs.
	BackgroundWriteIOPS int `json:"backgroundWriteIOPS"`
	// The target read IOPS to run while benchmarking. This is a limit and there is no guarantee
	// it will be reached. This is the total IOPS for all background read jobs.
	BackgroundReadIOPS int `json:"backgroundReadIOPS"`
	// Number of threads to use for background write IOPS. This should be set high enough to reach
	// the target specified in BackgroundWriteIOPS.
	// Example: If BackgroundWriteIOPS is 100 and write latency is 10ms then a single job would
	// barely be able to reach 100 IOPS so this should be at least 2.
	BackgroundWriteIOPSJobs int `json:"backgroundWriteIOPSJobs"`
	// Number of threads to use for background read IOPS. This should be set high enough to reach
	// the target specified in BackgrounReadIOPS.
	BackgroundReadIOPSJobs int `json:"backgroundReadIOPSJobs"`
}

type Certificate struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
	CertificatePath   string `json:"certificatePath" yaml:"certificatepath"`
	KeyPath           string `json:"keyPath" yaml:"keyPath"`
}

type HostServices struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
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
	KernelModules         *HostKernelModules     `json:"kernelModules,omitempty" yaml:"kernelModules,omitempty"`
	TCPConnect            *TCPConnect            `json:"tcpConnect,omitempty" yaml:"tcpConnect,omitempty"`
	FilesystemPerformance *FilesystemPerformance `json:"filesystemPerformance,omitempty" yaml:"filesystemPerformance,omitempty"`
	Certificate           *Certificate           `json:"certificate,omitempty" yaml:"certificate,omitempty"`
	HostServices          *HostServices          `json:"hostServices,omitempty" yaml:"hostServices,omitempty"`
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
