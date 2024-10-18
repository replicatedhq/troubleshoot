package analyzer

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"k8s.io/apimachinery/pkg/api/resource"
)

type AnalyzeHostMemory struct {
	hostAnalyzer *troubleshootv1beta2.MemoryAnalyze
}

func (a *AnalyzeHostMemory) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Amount of Memory")
}

func (a *AnalyzeHostMemory) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostMemory) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	result := AnalyzeResult{Title: a.Title()}

	// Use the generic function to collect both local and remote data
	collectedContents, err := retrieveCollectedContents(
		getCollectedFileContents,
		collect.HostMemoryPath,     // Local path
		collect.NodeInfoBaseDir,    // Remote base directory
		collect.HostMemoryFileName, // Remote file name
	)
	if err != nil {
		return []*AnalyzeResult{&result}, err
	}

	results, err := analyzeHostCollectorResults(collectedContents, a.hostAnalyzer.Outcomes, a.CheckCondition, a.Title())
	if err != nil {
		return nil, errors.Wrap(err, "failed to analyze OS version")
	}

	return results, nil
}

// checkCondition checks the condition of the when clause
func (a *AnalyzeHostMemory) CheckCondition(when string, data CollectorData) (bool, error) {
	rawData, ok := data.([]byte)
	if !ok {
		return false, fmt.Errorf("expected data to be []uint8 (raw bytes), got: %v", reflect.TypeOf(data))
	}

	var memInfo collect.MemoryInfo
	if err := json.Unmarshal(rawData, &memInfo); err != nil {
		return false, fmt.Errorf("failed to unmarshal data into MemoryInfo: %v", err)
	}

	parts := strings.Split(when, " ")
	if len(parts) != 2 {
		return false, fmt.Errorf("Expected 2 parts in conditional, got %d", len(parts))
	}

	operator := parts[0]
	desired := parts[1]
	quantity, err := resource.ParseQuantity(desired)
	if err != nil {
		return false, fmt.Errorf("could not parse quantity %q", desired)
	}
	desiredInt, ok := quantity.AsInt64()
	if !ok {
		return false, fmt.Errorf("could not parse quantity %q", desired)
	}
	if desiredInt < 0 {
		return false, fmt.Errorf("desired value must be a positive integer, got %d", desiredInt)
	}

	switch operator {
	case "<":
		return memInfo.Total < uint64(desiredInt), nil
	case "<=":
		return memInfo.Total <= uint64(desiredInt), nil
	case ">":
		return memInfo.Total > uint64(desiredInt), nil
	case ">=":
		return memInfo.Total >= uint64(desiredInt), nil
	case "=", "==", "===":
		return memInfo.Total == uint64(desiredInt), nil
	default:
		return false, fmt.Errorf("unsupported operator: %q", operator)
	}

}
