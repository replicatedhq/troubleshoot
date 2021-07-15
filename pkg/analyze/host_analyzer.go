package analyzer

import troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"

type HostAnalyzer interface {
	Title() string
	IsExcluded() (bool, error)
	Analyze(getFile func(string) ([]byte, error)) ([]*AnalyzeResult, error)
}

func GetHostAnalyzer(analyzer *troubleshootv1beta2.HostAnalyze) (HostAnalyzer, bool) {
	switch {
	case analyzer.CPU != nil:
		return &AnalyzeHostCPU{analyzer.CPU}, true
	case analyzer.Memory != nil:
		return &AnalyzeHostMemory{analyzer.Memory}, true
	case analyzer.TCPLoadBalancer != nil:
		return &AnalyzeHostTCPLoadBalancer{analyzer.TCPLoadBalancer}, true
	case analyzer.HTTPLoadBalancer != nil:
		return &AnalyzeHostHTTPLoadBalancer{analyzer.HTTPLoadBalancer}, true
	case analyzer.DiskUsage != nil:
		return &AnalyzeHostDiskUsage{analyzer.DiskUsage}, true
	case analyzer.TCPPortStatus != nil:
		return &AnalyzeHostTCPPortStatus{analyzer.TCPPortStatus}, true
	case analyzer.HTTP != nil:
		return &AnalyzeHostHTTP{analyzer.HTTP}, true
	case analyzer.Time != nil:
		return &AnalyzeHostTime{analyzer.Time}, true
	case analyzer.BlockDevices != nil:
		return &AnalyzeHostBlockDevices{analyzer.BlockDevices}, true
	case analyzer.KernelModules != nil:
		return &AnalyzeHostKernelModules{analyzer.KernelModules}, true
	case analyzer.TCPConnect != nil:
		return &AnalyzeHostTCPConnect{analyzer.TCPConnect}, true
	case analyzer.IPV4Interfaces != nil:
		return &AnalyzeHostIPV4Interfaces{analyzer.IPV4Interfaces}, true
	case analyzer.FilesystemPerformance != nil:
		return &AnalyzeHostFilesystemPerformance{analyzer.FilesystemPerformance}, true
	case analyzer.Certificate != nil:
		return &AnalyzeHostCertificate{analyzer.Certificate}, true
	case analyzer.HostServices != nil:
		return &AnalyzeHostServices{analyzer.HostServices}, true
	default:
		return nil, false
	}
}

func hostAnalyzerTitleOrDefault(meta troubleshootv1beta2.AnalyzeMeta, defaultTitle string) string {
	if meta.CheckName != "" {
		return meta.CheckName
	}
	return defaultTitle
}

type resultCollector struct {
	results []*AnalyzeResult
}

func (c *resultCollector) push(result *AnalyzeResult) {
	c.results = append(c.results, result)
}

// We need to return at least one result with a title to preserve compatability
func (c *resultCollector) get(title string) []*AnalyzeResult {
	if len(c.results) > 0 {
		return c.results
	}
	return []*AnalyzeResult{{Title: title, IsWarn: true, Message: "no results"}}
}
