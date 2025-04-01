package v1beta2

type CPUAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type NetworkNamespaceConnectivityAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type MemoryAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
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

type UDPPortStatusAnalyze struct {
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
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type TLSAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type BlockDevicesAnalyze struct {
	AnalyzeMeta                `json:",inline" yaml:",inline"`
	CollectorName              string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	MinimumAcceptableSize      uint64     `json:"minimumAcceptableSize" yaml:"minimumAcceptableSize"`
	IncludeUnmountedPartitions bool       `json:"includeUnmountedPartitions" yaml:"includeUnmountedPartitions"`
	Outcomes                   []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type SystemPackagesAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type KernelModulesAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type TCPConnectAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type IPV4InterfacesAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type SubnetAvailableAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type SubnetContainsIPAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	CIDR          string     `json:"cidr" yaml:"cidr"`
	IP            string     `json:"ip" yaml:"ip"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
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

type HostCertificatesCollectionAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type HostServicesAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type HostOSAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type KernelConfigsAnalyze struct {
	AnalyzeMeta     `json:",inline" yaml:",inline"`
	CollectorName   string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	SelectedConfigs []string   `json:"selectedConfigs" yaml:"selectedConfigs"`
	Outcomes        []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type HostSysctlAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type HostAnalyze struct {
	CPU                          *CPUAnalyze                          `json:"cpu,omitempty" yaml:"cpu,omitempty"`
	TCPLoadBalancer              *TCPLoadBalancerAnalyze              `json:"tcpLoadBalancer,omitempty" yaml:"tcpLoadBalancer,omitempty"`
	HTTPLoadBalancer             *HTTPLoadBalancerAnalyze             `json:"httpLoadBalancer,omitempty" yaml:"httpLoadBalancer,omitempty"`
	DiskUsage                    *DiskUsageAnalyze                    `json:"diskUsage,omitempty" yaml:"diskUsage,omitempty"`
	Memory                       *MemoryAnalyze                       `json:"memory,omitempty" yaml:"memory,omitempty"`
	TCPPortStatus                *TCPPortStatusAnalyze                `json:"tcpPortStatus,omitempty" yaml:"tcpPortStatus,omitempty"`
	UDPPortStatus                *UDPPortStatusAnalyze                `json:"udpPortStatus,omitempty" yaml:"udpPortStatus,omitempty"`
	HTTP                         *HTTPAnalyze                         `json:"http,omitempty" yaml:"http,omitempty"`
	Time                         *TimeAnalyze                         `json:"time,omitempty" yaml:"time,omitempty"`
	BlockDevices                 *BlockDevicesAnalyze                 `json:"blockDevices,omitempty" yaml:"blockDevices,omitempty"`
	SystemPackages               *SystemPackagesAnalyze               `json:"systemPackages,omitempty" yaml:"systemPackages,omitempty"`
	KernelModules                *KernelModulesAnalyze                `json:"kernelModules,omitempty" yaml:"kernelModules,omitempty"`
	TCPConnect                   *TCPConnectAnalyze                   `json:"tcpConnect,omitempty" yaml:"tcpConnect,omitempty"`
	IPV4Interfaces               *IPV4InterfacesAnalyze               `json:"ipv4Interfaces,omitempty" yaml:"ipv4Interfaces,omitempty"`
	SubnetAvailable              *SubnetAvailableAnalyze              `json:"subnetAvailable,omitempty" yaml:"subnetAvailable,omitempty"`
	SubnetContainsIP             *SubnetContainsIPAnalyze             `json:"subnetContainsIP,omitempty" yaml:"subnetContainsIP,omitempty"`
	FilesystemPerformance        *FilesystemPerformanceAnalyze        `json:"filesystemPerformance,omitempty" yaml:"filesystemPerformance,omitempty"`
	Certificate                  *CertificateAnalyze                  `json:"certificate,omitempty" yaml:"certificate,omitempty"`
	CertificatesCollection       *HostCertificatesCollectionAnalyze   `json:"certificatesCollection,omitempty" yaml:"certificatesCollection,omitempty"`
	HostServices                 *HostServicesAnalyze                 `json:"hostServices,omitempty" yaml:"hostServices,omitempty"`
	HostOS                       *HostOSAnalyze                       `json:"hostOS,omitempty" yaml:"hostOS,omitempty"`
	TextAnalyze                  *TextAnalyze                         `json:"textAnalyze,omitempty" yaml:"textAnalyze,omitempty"`
	KernelConfigs                *KernelConfigsAnalyze                `json:"kernelConfigs,omitempty" yaml:"kernelConfigs,omitempty"`
	JsonCompare                  *JsonCompare                         `json:"jsonCompare,omitempty" yaml:"jsonCompare,omitempty"`
	NetworkNamespaceConnectivity *NetworkNamespaceConnectivityAnalyze `json:"networkNamespaceConnectivity,omitempty" yaml:"networkNamespaceConnectivity,omitempty"`
	Sysctl                       *HostSysctlAnalyze                   `json:"sysctl,omitempty" yaml:"sysctl,omitempty"`
}
