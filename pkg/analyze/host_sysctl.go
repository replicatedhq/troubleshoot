package analyzer

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

// Ensure `AnalyzeHostSysctl` implements `HostAnalyzer` interface at compile time.
var _ HostAnalyzer = (*AnalyzeHostSysctl)(nil)

type AnalyzeHostSysctl struct {
	hostAnalyzer *troubleshootv1beta2.HostSysctlAnalyze
}

func (a *AnalyzeHostSysctl) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Sysctl")
}

func (a *AnalyzeHostSysctl) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostSysctl) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	result := AnalyzeResult{Title: a.Title()}

	// Use the generic function to collect both local and remote data
	collectedContents, err := retrieveCollectedContents(
		getCollectedFileContents,
		collect.HostSysctlPath,     // Local path
		collect.NodeInfoBaseDir,    // Remote base directory
		collect.HostSysctlFileName, // Remote file name
	)
	if err != nil {
		return []*AnalyzeResult{&result}, err
	}

	results, err := analyzeHostCollectorResults(collectedContents, a.hostAnalyzer.Outcomes, a.CheckCondition, a.Title())
	if err != nil {
		return nil, errors.Wrap(err, "failed to analyze sysctl output")
	}

	return results, nil
}

// checkCondition checks the condition of the when clause
func (a *AnalyzeHostSysctl) CheckCondition(when string, data []byte) (bool, error) {

	sysctl := map[string]string{}
	if err := json.Unmarshal(data, &sysctl); err != nil {
		return false, errors.Wrap(err, "failed to unmarshal data")
	}

	// <1:key> <2:operator> <3:value>
	matches := sysctlWhenRX.FindStringSubmatch(when)
	if len(matches) < 4 {
		return false, fmt.Errorf("expected 3 parts in when %q", when)
	}

	param := matches[1]
	expected := matches[3]
	opString := matches[2]
	operator, err := ParseComparisonOperator(opString)
	if err != nil {
		return false, errors.Wrap(err, fmt.Sprintf("failed to parse comparison operator %q", opString))
	}

	if _, ok := sysctl[param]; !ok {
		return false, fmt.Errorf("kernel parameter %q does not exist on collected sysctl output", param)
	}

	switch operator {
	case Equal:
		return expected == sysctl[param], nil
	}

	// operator used is an inequality operator, the only valid inputs should be ints, if not we'll error out
	value, err := strconv.Atoi(sysctl[param])
	if err != nil {
		return false, fmt.Errorf("collected sysctl param %q has value %q, cannot be used with provided operator %q", param, sysctl[param], opString)
	}
	expectedInt, err := strconv.Atoi(expected)
	if err != nil {
		return false, fmt.Errorf("expected value for sysctl param %q has value %q, cannot be used with provided operator %q", param, expected, opString)
	}

	switch operator {
	case LessThan:
		return value < expectedInt, nil
	case LessThanOrEqual:
		return value <= expectedInt, nil
	case GreaterThan:
		return value > expectedInt, nil
	case GreaterThanOrEqual:
		return value >= expectedInt, nil
	default:
		return false, fmt.Errorf("unsupported operator %q", opString)
	}

}
