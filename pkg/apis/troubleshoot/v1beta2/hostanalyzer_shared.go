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

	HTTP *HTTPAnalyze `json:"http" yaml:"http"`

	Time *TimeAnalyze `json:"time" yaml:"time"`

	BlockDevices *BlockDevicesAnalyze `json:"blockDevices" yaml:"blockDevices"`
}
