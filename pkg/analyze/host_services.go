package analyzer

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeHostServices struct {
	hostAnalyzer *troubleshootv1beta2.HostServicesAnalyze
}

func (a *AnalyzeHostServices) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Host Services")
}

func (a *AnalyzeHostServices) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostServices) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	result := AnalyzeResult{Title: a.Title()}

	collectedContents, err := retrieveCollectedContents(
		getCollectedFileContents,
		collect.HostServicesPath,
		collect.NodeInfoBaseDir,
		collect.HostServicesFileName,
	)
	if err != nil {
		return []*AnalyzeResult{&result}, err
	}

	results, err := analyzeHostCollectorResults(collectedContents, a.hostAnalyzer.Outcomes, a.CheckCondition, a.Title())
	if err != nil {
		return nil, errors.Wrap(err, "failed to analyze host services")
	}

	return results, nil
}

// <service> <op> <state>
// example: ufw.service = active
func compareHostServicesConditionalToActual(conditional string, services []collect.ServiceInfo) (res bool, err error) {
	parts := strings.Split(conditional, " ")
	if len(parts) != 3 {
		return false, fmt.Errorf("expected exactly 3 parts, got %d", len(parts))
	}

	matchParams := strings.Split(parts[2], ",")
	activeMatch := matchParams[0]
	subMatch := ""
	loadMatch := ""
	if len(matchParams) > 1 {
		subMatch = matchParams[1]
	}
	if len(matchParams) > 2 {
		loadMatch = matchParams[2]
	}

	switch parts[1] {
	case "=", "==":
		for _, service := range services {
			if isServiceMatch(service.Unit, parts[0]) {
				isMatch := true
				if activeMatch != "" && activeMatch != "*" {
					isMatch = isMatch && (activeMatch == service.Active)
				}
				if subMatch != "" && subMatch != "*" {
					isMatch = isMatch && (subMatch == service.Sub)
				}
				if loadMatch != "" && loadMatch != "*" {
					isMatch = isMatch && (loadMatch == service.Load)
				}

				return isMatch, nil
			}
		}
		return false, nil
	case "!=", "<>":
		for _, service := range services {
			if isServiceMatch(service.Unit, parts[0]) {
				isMatch := false
				if activeMatch != "" && activeMatch != "*" {
					isMatch = isMatch || (activeMatch != service.Active)
				}
				if subMatch != "" && subMatch != "*" {
					isMatch = isMatch || (subMatch != service.Sub)
				}
				if loadMatch != "" && loadMatch != "*" {
					isMatch = isMatch || (loadMatch != service.Load)
				}

				return isMatch, nil
			}
		}
		return false, nil
	}

	return false, fmt.Errorf("unexpected operator %q", parts[1])
}

func isServiceMatch(serviceName string, matchName string) bool {
	if serviceName == matchName {
		return true
	}

	if strings.HasPrefix(serviceName, matchName) {
		return true
	}

	return false
}

func (a *AnalyzeHostServices) CheckCondition(when string, data collectorData) (bool, error) {
	rawData, ok := data.([]byte)
	if !ok {
		return false, fmt.Errorf("expected data to be []uint8 (raw bytes), got: %v", reflect.TypeOf(data))
	}

	var services []collect.ServiceInfo
	if err := json.Unmarshal(rawData, &services); err != nil {
		return false, fmt.Errorf("failed to unmarshal data into ServiceInfo: %v", err)
	}

	return compareHostServicesConditionalToActual(when, services)
}
