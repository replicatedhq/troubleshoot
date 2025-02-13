package analyzer

import (
	"net"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type AnalyzeHostSubnetContainsIP struct {
	hostAnalyzer *troubleshootv1beta2.SubnetContainsIPAnalyze
}

func (a *AnalyzeHostSubnetContainsIP) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Subnet Contains IP")
}

func (a *AnalyzeHostSubnetContainsIP) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostSubnetContainsIP) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	_, ipNet, err := net.ParseCIDR(a.hostAnalyzer.CIDR)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse CIDR %s", a.hostAnalyzer.CIDR)
	}

	ip := net.ParseIP(a.hostAnalyzer.IP)
	if ip == nil {
		return nil, errors.Errorf("failed to parse IP address %s", a.hostAnalyzer.IP)
	}

	contains := ipNet.Contains(ip)

	results, err := analyzeHostCollectorResults([]collectedContent{{Data: []byte(contains)}}, a.hostAnalyzer.Outcomes, a.CheckCondition, a.Title())
	if err != nil {
		return nil, errors.Wrap(err, "failed to analyze Subnet Contains IP")
	}

	return results, nil
}

func (a *AnalyzeHostSubnetContainsIP) CheckCondition(when string, data []byte) (bool, error) {
	switch when {
	case "true":
		return string(data) == "true", nil
	case "false":
		return string(data) == "false", nil
	}

	return false, errors.Errorf("unknown condition: %q", when)
}
