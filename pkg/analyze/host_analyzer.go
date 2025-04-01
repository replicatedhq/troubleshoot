package analyzer

import (
	"fmt"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type HostAnalyzer interface {
	Title() string
	IsExcluded() (bool, error)
	Analyze(getFile func(string) ([]byte, error), findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error)
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
	case analyzer.UDPPortStatus != nil:
		return &AnalyzeHostUDPPortStatus{analyzer.UDPPortStatus}, true
	case analyzer.HTTP != nil:
		return &AnalyzeHostHTTP{analyzer.HTTP}, true
	case analyzer.Time != nil:
		return &AnalyzeHostTime{analyzer.Time}, true
	case analyzer.BlockDevices != nil:
		return &AnalyzeHostBlockDevices{analyzer.BlockDevices}, true
	case analyzer.SystemPackages != nil:
		return &AnalyzeHostSystemPackages{analyzer.SystemPackages}, true
	case analyzer.KernelModules != nil:
		return &AnalyzeHostKernelModules{analyzer.KernelModules}, true
	case analyzer.TCPConnect != nil:
		return &AnalyzeHostTCPConnect{analyzer.TCPConnect}, true
	case analyzer.IPV4Interfaces != nil:
		return &AnalyzeHostIPV4Interfaces{analyzer.IPV4Interfaces}, true
	case analyzer.SubnetAvailable != nil:
		return &AnalyzeHostSubnetAvailable{analyzer.SubnetAvailable}, true
	case analyzer.SubnetContainsIP != nil:
		return &AnalyzeHostSubnetContainsIP{analyzer.SubnetContainsIP}, true
	case analyzer.FilesystemPerformance != nil:
		return &AnalyzeHostFilesystemPerformance{analyzer.FilesystemPerformance}, true
	case analyzer.Certificate != nil:
		return &AnalyzeHostCertificate{analyzer.Certificate}, true
	case analyzer.CertificatesCollection != nil:
		return &AnalyzeHostCertificatesCollection{analyzer.CertificatesCollection}, true
	case analyzer.HostServices != nil:
		return &AnalyzeHostServices{analyzer.HostServices}, true
	case analyzer.HostOS != nil:
		return &AnalyzeHostOS{analyzer.HostOS}, true
	case analyzer.TextAnalyze != nil:
		return &AnalyzeHostTextAnalyze{analyzer.TextAnalyze}, true
	case analyzer.KernelConfigs != nil:
		return &AnalyzeHostKernelConfigs{analyzer.KernelConfigs}, true
	case analyzer.JsonCompare != nil:
		return &AnalyzeHostJsonCompare{analyzer.JsonCompare}, true
	case analyzer.NetworkNamespaceConnectivity != nil:
		return &AnalyzeHostNetworkNamespaceConnectivity{analyzer.NetworkNamespaceConnectivity}, true
	case analyzer.Sysctl != nil:
		return &AnalyzeHostSysctl{analyzer.Sysctl}, true
	case analyzer.TLS != nil:
		return &AnalyzeHostTLS{analyzer.TLS}, true
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

func analyzeHostCollectorResults(collectedContent []collectedContent, outcomes []*troubleshootv1beta2.Outcome, checkCondition func(string, []byte) (bool, error), title string) ([]*AnalyzeResult, error) {
	var results []*AnalyzeResult
	for _, content := range collectedContent {
		currentTitle := title
		if content.NodeName != "" {
			currentTitle = fmt.Sprintf("%s - Node %s", title, content.NodeName)
		}

		analyzeResult, err := evaluateOutcomes(outcomes, checkCondition, content.Data, currentTitle)
		if err != nil {
			return nil, errors.Wrap(err, "failed to evaluate outcomes")
		}
		if analyzeResult != nil {
			results = append(results, analyzeResult...)
		}
	}
	return results, nil
}

func evaluateOutcomes(outcomes []*troubleshootv1beta2.Outcome, checkCondition func(string, []byte) (bool, error), data []byte, title string) ([]*AnalyzeResult, error) {
	var results []*AnalyzeResult

	for _, outcome := range outcomes {
		result := AnalyzeResult{
			Title: title,
		}

		switch {
		case outcome.Fail != nil:
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI
				results = append(results, &result)
				return results, nil
			}

			isMatch, err := checkCondition(outcome.Fail.When, data)
			if err != nil {
				return []*AnalyzeResult{&result}, errors.Wrapf(err, "failed to compare %s", outcome.Fail.When)
			}

			if isMatch {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI
				results = append(results, &result)
				return results, nil
			}

		case outcome.Warn != nil:
			if outcome.Warn.When == "" {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI
				results = append(results, &result)
				return results, nil
			}

			isMatch, err := checkCondition(outcome.Warn.When, data)
			if err != nil {
				return []*AnalyzeResult{&result}, errors.Wrapf(err, "failed to compare %s", outcome.Warn.When)
			}

			if isMatch {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI
				results = append(results, &result)
				return results, nil
			}

		case outcome.Pass != nil:
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI
				results = append(results, &result)
				return results, nil
			}

			isMatch, err := checkCondition(outcome.Pass.When, data)
			if err != nil {
				return []*AnalyzeResult{&result}, errors.Wrapf(err, "failed to compare %s", outcome.Pass.When)
			}

			if isMatch {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI
				results = append(results, &result)
				return results, nil
			}
		}
	}

	return nil, nil
}
