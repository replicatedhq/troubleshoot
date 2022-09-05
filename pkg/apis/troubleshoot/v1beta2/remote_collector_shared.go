package v1beta2

import (
	"fmt"

	"github.com/replicatedhq/troubleshoot/pkg/multitype"
	authorizationv1 "k8s.io/api/authorization/v1"
)

type RemoteCollectorMeta struct {
	CollectorName string `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	// +optional
	Exclude *multitype.BoolOrString `json:"exclude,omitempty" yaml:"exclude,omitempty"`
}

type RemoteCPU struct {
	RemoteCollectorMeta `json:",inline" yaml:",inline"`
}

type RemoteMemory struct {
	RemoteCollectorMeta `json:",inline" yaml:",inline"`
}

type RemoteTCPLoadBalancer struct {
	RemoteCollectorMeta `json:",inline" yaml:",inline"`
	Address             string `json:"address"`
	Port                int    `json:"port"`
	Timeout             string `json:"timeout,omitempty"`
}

type RemoteHTTPLoadBalancer struct {
	RemoteCollectorMeta `json:",inline" yaml:",inline"`
	Address             string `json:"address"`
	Port                int    `json:"port"`
	Path                string `json:"path"`
	Timeout             string `json:"timeout,omitempty"`
}

type RemoteTCPPortStatus struct {
	RemoteCollectorMeta `json:",inline" yaml:",inline"`
	Interface           string `json:"interface,omitempty"`
	Port                int    `json:"port"`
}

type RemoteKubernetes struct {
	RemoteCollectorMeta `json:",inline" yaml:",inline"`
}

type RemoteIPV4Interfaces struct {
	RemoteCollectorMeta `json:",inline" yaml:",inline"`
}

type RemoteDiskUsage struct {
	RemoteCollectorMeta `json:",inline" yaml:",inline"`
	Path                string `json:"path"`
}

type RemoteHTTP struct {
	RemoteCollectorMeta `json:",inline" yaml:",inline"`
	Get                 *Get  `json:"get,omitempty" yaml:"get,omitempty"`
	Post                *Post `json:"post,omitempty" yaml:"post,omitempty"`
	Put                 *Put  `json:"put,omitempty" yaml:"put,omitempty"`
}

type RemoteTime struct {
	RemoteCollectorMeta `json:",inline" yaml:",inline"`
}

type RemoteBlockDevices struct {
	RemoteCollectorMeta `json:",inline" yaml:",inline"`
}

type RemoteSystemPackages struct {
	RemoteCollectorMeta `json:",inline" yaml:",inline"`
}

type RemoteKernelModules struct {
	RemoteCollectorMeta `json:",inline" yaml:",inline"`
}

type RemoteTCPConnect struct {
	RemoteCollectorMeta `json:",inline" yaml:",inline"`
	Address             string `json:"address"`
	Timeout             string `json:"timeout,omitempty"`
}

// RemoteFilesystemPerformance benchmarks sequential write latency on a single file.
// The optional background IOPS feature attempts to mimic real-world conditions by running read and
// write workloads prior to and during benchmark execution.
type RemoteFilesystemPerformance struct {
	RemoteCollectorMeta `json:",inline" yaml:",inline"`
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

type RemoteCertificate struct {
	RemoteCollectorMeta `json:",inline" yaml:",inline"`
	CertificatePath     string `json:"certificatePath" yaml:"certificatepath"`
	KeyPath             string `json:"keyPath" yaml:"keyPath"`
}

type RemoteServices struct {
	RemoteCollectorMeta `json:",inline" yaml:",inline"`
}

type RemoteCollect struct {
	CPU                   *RemoteCPU                   `json:"cpu,omitempty" yaml:"cpu,omitempty"`
	Memory                *RemoteMemory                `json:"memory,omitempty" yaml:"memory,omitempty"`
	TCPLoadBalancer       *RemoteTCPLoadBalancer       `json:"tcpLoadBalancer,omitempty" yaml:"tcpLoadBalancer,omitempty"`
	HTTPLoadBalancer      *RemoteHTTPLoadBalancer      `json:"httpLoadBalancer,omitempty" yaml:"httpLoadBalancer,omitempty"`
	TCPPortStatus         *RemoteTCPPortStatus         `json:"tcpPortStatus,omitempty" yaml:"tcpPortStatus,omitempty"`
	IPV4Interfaces        *RemoteIPV4Interfaces        `json:"ipv4Interfaces,omitempty" yaml:"ipv4Interfaces,omitempty"`
	DiskUsage             *RemoteDiskUsage             `json:"diskUsage,omitempty" yaml:"diskUsage,omitempty"`
	HTTP                  *RemoteHTTP                  `json:"http,omitempty" yaml:"http,omitempty"`
	Time                  *RemoteTime                  `json:"time,omitempty" yaml:"time,omitempty"`
	BlockDevices          *RemoteBlockDevices          `json:"blockDevices,omitempty" yaml:"blockDevices,omitempty"`
	SystemPackages        *RemoteSystemPackages        `json:"systemPackages,omitempty" yaml:"systemPackages,omitempty"`
	KernelModules         *RemoteKernelModules         `json:"kernelModules,omitempty" yaml:"kernelModules,omitempty"`
	TCPConnect            *RemoteTCPConnect            `json:"tcpConnect,omitempty" yaml:"tcpConnect,omitempty"`
	FilesystemPerformance *RemoteFilesystemPerformance `json:"filesystemPerformance,omitempty" yaml:"filesystemPerformance,omitempty"`
	Certificate           *RemoteCertificate           `json:"certificate,omitempty" yaml:"certificate,omitempty"`
	HostServices          *RemoteServices              `json:"hostServices,omitempty" yaml:"hostServices,omitempty"`
}

func (c *RemoteCollect) AccessReviewSpecs(overrideNS string) []authorizationv1.SelfSubjectAccessReviewSpec {
	return []authorizationv1.SelfSubjectAccessReviewSpec{
		{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace:   overrideNS,
				Verb:        "create,delete",
				Group:       "",
				Version:     "",
				Resource:    "pods,configmap",
				Subresource: "",
				Name:        "",
			},
			NonResourceAttributes: nil,
		},
	}
}

func (c *RemoteCollect) GetName() string {
	var collector, name, selector string
	if c.CPU != nil {
		collector = "cpu"
	}
	if c.Memory != nil {
		collector = "memory"
	}
	if c.KernelModules != nil {
		collector = "kernel-modules"
	}

	if collector == "" {
		return "<none>"
	}
	if name != "" {
		return fmt.Sprintf("%s/%s", collector, name)
	}
	if selector != "" {
		return fmt.Sprintf("%s/%s", collector, selector)
	}
	return collector
}
