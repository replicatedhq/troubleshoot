package v1beta2

type CPUAnalyze struct {
	AnalyzeMeta `json:",inline" yaml:",inline"`
	Outcomes    []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type TCPLoadBalancerAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type HostAnalyze struct {
	CPU *CPUAnalyze `json:"cpu,omitempty" yaml:"cpu,omitempty"`
	//
	TCPLoadBalancer *TCPLoadBalancerAnalyze `json:"tcpLoadBalancer,omitempty" yaml:"tcpLoadBalancer,omitempty"`
}
