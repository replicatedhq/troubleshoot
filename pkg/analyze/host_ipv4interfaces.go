package analyzer

import (
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeHostIPV4Interfaces struct {
	hostAnalyzer *troubleshootv1beta2.IPV4InterfacesAnalyze
}

func (a *AnalyzeHostIPV4Interfaces) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "IPv4 Interfaces")
}

func (a *AnalyzeHostIPV4Interfaces) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostIPV4Interfaces) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	result := AnalyzeResult{Title: a.Title()}

	collectedContents, err := retrieveCollectedContents(
		getCollectedFileContents,
		collect.HostIPV4InterfacesPath,
		collect.NodeInfoBaseDir,
		collect.HostIPV4FileName,
	)
	if err != nil {
		return []*AnalyzeResult{&result}, err
	}

	results, err := analyzeHostCollectorResults(collectedContents, a.hostAnalyzer.Outcomes, a.CheckCondition, a.Title())
	if err != nil {
		return nil, errors.Wrap(err, "failed to analyze IPv4 interfaces")
	}

	return results, nil
}

func compareHostIPV4InterfacesConditionalToActual(conditional string, ipv4Interfaces []net.Interface) (res bool, err error) {
	parts := strings.Split(conditional, " ")
	if len(parts) != 3 {
		return false, fmt.Errorf("Expected exactly 3 parts in conditional, got %d", len(parts))
	}

	keyword := parts[0]
	operator := parts[1]
	desired := parts[2]

	if keyword != "count" {
		return false, fmt.Errorf(`Only supported keyword is "count", got %q`, keyword)
	}

	desiredInt, err := strconv.ParseInt(desired, 10, 64)
	if err != nil {
		return false, errors.Wrapf(err, "failed to parse %q as int", desired)
	}

	actualCount := len(ipv4Interfaces)

	switch operator {
	case "<":
		return actualCount < int(desiredInt), nil
	case "<=":
		return actualCount <= int(desiredInt), nil
	case ">":
		return actualCount > int(desiredInt), nil
	case ">=":
		return actualCount >= int(desiredInt), nil
	case "=", "==", "===":
		return actualCount == int(desiredInt), nil
	}

	return false, fmt.Errorf("Unknown operator %q. Supported operators are: <, <=, ==, >=, >", operator)
}

func (a *AnalyzeHostIPV4Interfaces) CheckCondition(when string, data collectorData) (bool, error) {
	rawData, ok := data.([]byte)
	if !ok {
		return false, fmt.Errorf("expected data to be []uint8 (raw bytes), got: %v", reflect.TypeOf(data))
	}

	var ipv4Interfaces []net.Interface
	if err := json.Unmarshal(rawData, &ipv4Interfaces); err != nil {
		return false, fmt.Errorf("failed to unmarshal data into []net.Interface: %v", err)
	}

	return compareHostIPV4InterfacesConditionalToActual(when, ipv4Interfaces)
}
