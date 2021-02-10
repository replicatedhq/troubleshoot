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

type HostAnalyze struct {
	CPU *CPUAnalyze `json:"cpu,omitempty" yaml:"cpu,omitempty"`
	//
	TCPLoadBalancer *TCPLoadBalancerAnalyze `json:"tcpLoadBalancer,omitempty" yaml:"tcpLoadBalancer,omitempty"`

	DiskUsage *DiskUsageAnalyze `json:"diskUsage,omitempty" yaml:"diskUsage,omitempty"`

	Memory *MemoryAnalyze `json:"memory,omitempty" yaml:"memory,omitempty"`

	TCPPortStatus *TCPPortStatusAnalyze `json:"tcpPortStatus,omitempty" yaml:"tcpPortStatus,omitempty"`

	HTTP *HTTPAnalyze `json:"http" yaml:"http"`
}
