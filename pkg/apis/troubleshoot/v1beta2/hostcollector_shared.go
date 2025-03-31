package v1beta2

import (
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
)

type HostCollectorMeta struct {
	CollectorName string `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	// +optional
	Exclude *multitype.BoolOrString `json:"exclude,omitempty" yaml:"exclude,omitempty"`
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

type UDPPortStatus struct {
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

type SubnetAvailable struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
	CIDRRangeAlloc    string `json:"CIDRRangeAlloc" yaml:"CIDRRangeAlloc"`
	DesiredCIDR       int    `json:"desiredCIDR" yaml:"desiredCIDR"`
}

type DiskUsage struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
	Path              string `json:"path" yaml:"path"`
}

type HostHTTP struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
	Get               *Get  `json:"get,omitempty" yaml:"get,omitempty"`
	Post              *Post `json:"post,omitempty" yaml:"post,omitempty"`
	Put               *Put  `json:"put,omitempty" yaml:"put,omitempty"`
}

type HostCopy struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
	Path              string `json:"path" yaml:"path"`
}

type HostNetworkNamespaceConnectivity struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
	FromCIDR          string `json:"fromCIDR" yaml:"fromCIDR"`
	ToCIDR            string `json:"toCIDR" yaml:"toCIDR"`
	Timeout           string `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Port              int    `json:"port" yaml:"port"`
}

type HostCGroups struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
	MountPoint        string `json:"mountPoint,omitempty" yaml:"mountPoint,omitempty"`
}

type HostTime struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
}

type HostBlockDevices struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
}

type HostSystemPackages struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
	Ubuntu            []string `json:"ubuntu,omitempty"`
	Ubuntu16          []string `json:"ubuntu16,omitempty"`
	Ubuntu18          []string `json:"ubuntu18,omitempty"`
	Ubuntu20          []string `json:"ubuntu20,omitempty"`
	RHEL              []string `json:"rhel,omitempty"`
	RHEL7             []string `json:"rhel7,omitempty"`
	RHEL8             []string `json:"rhel8,omitempty"`
	RHEL9             []string `json:"rhel9,omitempty"`
	RockyLinux        []string `json:"rocky,omitempty"`
	RockyLinux8       []string `json:"rocky8,omitempty"`
	RockyLinux9       []string `json:"rocky9,omitempty"`
	CentOS            []string `json:"centos,omitempty"`
	CentOS7           []string `json:"centos7,omitempty"`
	CentOS8           []string `json:"centos8,omitempty"`
	CentOS9           []string `json:"centos9,omitempty"`
	OracleLinux       []string `json:"ol,omitempty"`
	OracleLinux7      []string `json:"ol7,omitempty"`
	OracleLinux8      []string `json:"ol8,omitempty"`
	OracleLinux9      []string `json:"ol9,omitempty"`
	AmazonLinux       []string `json:"amzn,omitempty"`
	AmazonLinux2      []string `json:"amzn2,omitempty"`
}

type HostKernelModules struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
}

type HostOS struct {
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
	// Limit runtime. The test will run until it completes the configured I/O workload or until it
	// has run for this specified amount of time, whichever occurs first. When the unit is omitted,
	// the value is interpreted in seconds. Defaults to 120 seconds. Set to "0" to disable.
	RunTime *string `json:"runTime,omitempty"`

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

type HostCertificatesCollection struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
	Paths             []string `json:"paths" yaml:"paths"`
}

type HostServices struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
}

type HostRun struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
	Command           string            `json:"command"`
	Args              []string          `json:"args"`
	OutputDir         string            `json:"outputDir,omitempty" yaml:"outputDir,omitempty"`
	Input             map[string]string `json:"input,omitempty" yaml:"input,omitempty"`
	Env               []string          `json:"env,omitempty" yaml:"env,omitempty"`
	InheritEnvs       []string          `json:"inheritEnvs,omitempty" yaml:"inheritEnvs,omitempty"`
	IgnoreParentEnvs  bool              `json:"ignoreParentEnvs,omitempty" yaml:"ignoreParentEnvs,omitempty"`
	Timeout           string            `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

type HostKernelConfigs struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
}

type HostJournald struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
	System            bool     `json:"system,omitempty" yaml:"system,omitempty"`
	Dmesg             bool     `json:"dmesg,omitempty" yaml:"dmesg,omitempty"`
	Units             []string `json:"units,omitempty" yaml:"units,omitempty"`
	Since             string   `json:"since,omitempty" yaml:"since,omitempty"`
	Until             string   `json:"until,omitempty" yaml:"until,omitempty"`
	Output            string   `json:"output,omitempty" yaml:"output,omitempty"`
	Lines             int      `json:"lines,omitempty" yaml:"lines,omitempty"`
	Reverse           bool     `json:"reverse,omitempty" yaml:"reverse,omitempty"`
	Utc               bool     `json:"utc,omitempty" yaml:"utc,omitempty"`
	Timeout           string   `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

type HostDNS struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
	Hostnames         []string `json:"hostnames" yaml:"hostnames"`
}

type HostSysctl struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
}

type HostTLS struct {
	HostCollectorMeta `json:",inline" yaml:",inline"`
	Address           string `json:"address"`
	ExpectedIssuer    string `json:"expectedIssuer,omitempty"`
}

type HostCollect struct {
	CPU                          *CPU                              `json:"cpu,omitempty" yaml:"cpu,omitempty"`
	Memory                       *Memory                           `json:"memory,omitempty" yaml:"memory,omitempty"`
	TCPLoadBalancer              *TCPLoadBalancer                  `json:"tcpLoadBalancer,omitempty" yaml:"tcpLoadBalancer,omitempty"`
	HTTPLoadBalancer             *HTTPLoadBalancer                 `json:"httpLoadBalancer,omitempty" yaml:"httpLoadBalancer,omitempty"`
	TCPPortStatus                *TCPPortStatus                    `json:"tcpPortStatus,omitempty" yaml:"tcpPortStatus,omitempty"`
	UDPPortStatus                *UDPPortStatus                    `json:"udpPortStatus,omitempty" yaml:"udpPortStatus,omitempty"`
	Kubernetes                   *Kubernetes                       `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty"`
	IPV4Interfaces               *IPV4Interfaces                   `json:"ipv4Interfaces,omitempty" yaml:"ipv4Interfaces,omitempty"`
	SubnetAvailable              *SubnetAvailable                  `json:"subnetAvailable,omitempty" yaml:"subnetAvailable,omitempty"`
	DiskUsage                    *DiskUsage                        `json:"diskUsage,omitempty" yaml:"diskUsage,omitempty"`
	HTTP                         *HostHTTP                         `json:"http,omitempty" yaml:"http,omitempty"`
	Time                         *HostTime                         `json:"time,omitempty" yaml:"time,omitempty"`
	BlockDevices                 *HostBlockDevices                 `json:"blockDevices,omitempty" yaml:"blockDevices,omitempty"`
	SystemPackages               *HostSystemPackages               `json:"systemPackages,omitempty" yaml:"systemPackages,omitempty"`
	KernelModules                *HostKernelModules                `json:"kernelModules,omitempty" yaml:"kernelModules,omitempty"`
	TCPConnect                   *TCPConnect                       `json:"tcpConnect,omitempty" yaml:"tcpConnect,omitempty"`
	FilesystemPerformance        *FilesystemPerformance            `json:"filesystemPerformance,omitempty" yaml:"filesystemPerformance,omitempty"`
	Certificate                  *Certificate                      `json:"certificate,omitempty" yaml:"certificate,omitempty"`
	CertificatesCollection       *HostCertificatesCollection       `json:"certificatesCollection,omitempty" yaml:"certificatesCollection,omitempty"`
	HostServices                 *HostServices                     `json:"hostServices,omitempty" yaml:"hostServices,omitempty"`
	HostOS                       *HostOS                           `json:"hostOS,omitempty" yaml:"hostOS,omitempty"`
	HostRun                      *HostRun                          `json:"run,omitempty" yaml:"run,omitempty"`
	HostCopy                     *HostCopy                         `json:"copy,omitempty" yaml:"copy,omitempty"`
	HostKernelConfigs            *HostKernelConfigs                `json:"kernelConfigs,omitempty" yaml:"kernelConfigs,omitempty"`
	HostJournald                 *HostJournald                     `json:"journald,omitempty" yaml:"journald,omitempty"`
	HostCGroups                  *HostCGroups                      `json:"cgroups,omitempty" yaml:"cgroups,omitempty"`
	HostDNS                      *HostDNS                          `json:"dns,omitempty" yaml:"dns,omitempty"`
	NetworkNamespaceConnectivity *HostNetworkNamespaceConnectivity `json:"networkNamespaceConnectivity,omitempty" yaml:"networkNamespaceConnectivity,omitempty"`
	HostSysctl                   *HostSysctl                       `json:"sysctl,omitempty" yaml:"sysctl,omitempty"`
	HostTLS                      *HostTLS                          `json:"tls,omitempty" yaml:"tls,omitempty"`
}

// GetName gets the name of the collector
// Deprecated: This function is not used anywhere and should be removed. Do not use it.
func (c *HostCollect) GetName() string {
	// TODO: Is this used anywhere? Should we just remove it?
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
