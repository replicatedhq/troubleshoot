package v1beta2

type CPUAnalyze struct {
	AnalyzeMeta `json:",inline" yaml:",inline"`
	Outcomes    []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type MemoryAnalyze struct {
	AnalyzeMeta `json:",inline" yaml:",inline"`
	Outcomes    []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type TCPLoadBalancerAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type HTTPLoadBalancerAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type TCPPortStatusAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type DiskUsageAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type HTTPAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type TimeAnalyze struct {
	AnalyzeMeta `json:",inline" yaml:",inline"`
	Outcomes    []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type BlockDevicesAnalyze struct {
	AnalyzeMeta                `json:",inline" yaml:",inline"`
	MinimumAcceptableSize      uint64     `json:"minimumAcceptableSize" yaml:"minimumAcceptableSize"`
	IncludeUnmountedPartitions bool       `json:"includeUnmountedPartitions" yaml:"includeUnmountedPartitions"`
	Outcomes                   []*Outcome `json:"outcomes" yaml:"outcomes"`
}
type KernelModulesAnalyze struct {
	AnalyzeMeta `json:",inline" yaml:",inline"`
	Outcomes    []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type TCPConnectAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type IPV4InterfacesAnalyze struct {
	AnalyzeMeta `json:",inline" yaml:",inline"`
	Outcomes    []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type FilesystemPerformanceAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type CertificateAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type HostServicesAnalyze struct {
	AnalyzeMeta `json:",inline" yaml:",inline"`
	Outcomes    []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type HostAnalyze struct {
	CPU *CPUAnalyze `json:"cpu,omitempty" yaml:"cpu,omitempty"`
	//
	TCPLoadBalancer  *TCPLoadBalancerAnalyze  `json:"tcpLoadBalancer,omitempty" yaml:"tcpLoadBalancer,omitempty"`
	HTTPLoadBalancer *HTTPLoadBalancerAnalyze `json:"httpLoadBalancer,omitempty" yaml:"httpLoadBalancer,omitempty"`

	DiskUsage *DiskUsageAnalyze `json:"diskUsage,omitempty" yaml:"diskUsage,omitempty"`

	Memory *MemoryAnalyze `json:"memory,omitempty" yaml:"memory,omitempty"`

	TCPPortStatus *TCPPortStatusAnalyze `json:"tcpPortStatus,omitempty" yaml:"tcpPortStatus,omitempty"`

	HTTP *HTTPAnalyze `json:"http,omitempty" yaml:"http,omitempty"`

	Time *TimeAnalyze `json:"time,omitempty" yaml:"time,omitempty"`

	BlockDevices *BlockDevicesAnalyze `json:"blockDevices,omitempty" yaml:"blockDevices,omitempty"`

	KernelModules *KernelModulesAnalyze `json:"kernelModules,omitempty" yaml:"kernelModules,omitempty"`

	TCPConnect *TCPConnectAnalyze `json:"tcpConnect,omitempty" yaml:"tcpConnect,omitempty"`

	IPV4Interfaces *IPV4InterfacesAnalyze `json:"ipv4Interfaces,omitempty" yaml:"ipv4Interfaces,omitempty"`

	FilesystemPerformance *FilesystemPerformanceAnalyze `json:"filesystemPerformance,omitempty" yaml:"filesystemPerformance,omitempty"`

	Certificate *CertificateAnalyze `json:"certificate,omitempty" yaml:"certificate,omitempty"`

	HostServices *HostServicesAnalyze `json:"hostServices,omitempty" yaml:"hostServices,omitempty"`
}
