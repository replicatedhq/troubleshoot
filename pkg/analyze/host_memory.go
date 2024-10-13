package analyzer

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
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
	var results []*AnalyzeResult
	var memoryInfo collect.MemoryInfo
	var remoteCollectContents []RemoteCollectContent
	result := AnalyzeResult{}
	result.Title = a.Title()
	//hostAnalyzer := a.hostAnalyzer

	// check if the host os info file exists (local mode)
	contents, err := getCollectedFileContents(collect.HostMemoryPath)
	if err == nil {
		if err := json.Unmarshal(contents, &memoryInfo); err != nil {
			return []*AnalyzeResult{&result}, errors.Wrap(err, "failed to unmarshal host os info")
		}
		remoteCollectContents = append(remoteCollectContents, RemoteCollectContent{NodeName: "", Content: memoryInfo})
		return analyzeOSVersionResult(remoteCollectContents, a.hostAnalyzer.Outcomes, a.Title())
	} else {
		// check if the node list file exists (remote mode)
		contents, err := getCollectedFileContents(constants.NODE_LIST_FILE)
		if err != nil {
			return []*AnalyzeResult{&result}, errors.Wrap(err, "failed to get collected file")
		}

		var nodeNames NodesInfo
		if err := json.Unmarshal(contents, &nodeNames); err != nil {
			return []*AnalyzeResult{&result}, errors.Wrap(err, "failed to unmarshal node names")
		}

		for _, nodeName := range nodeNames.Nodes {
			contents, err := getCollectedFileContents(fmt.Sprintf("%s/%s/%s", collect.NodeInfoBaseDir, nodeName, collect.HostMemoryFileName))
			if err != nil {
				return []*AnalyzeResult{&result}, errors.Wrap(err, "failed to get collected file")
			}

			if err := json.Unmarshal(contents, &memoryInfo); err != nil {
				return []*AnalyzeResult{&result}, errors.Wrap(err, "failed to unmarshal host os info")
			}

			remoteCollectContents = append(remoteCollectContents, RemoteCollectContent{NodeName: nodeName, Content: memoryInfo})
		}

		for _, memoryInfo := range remoteCollectContents {

			currentTitle := a.Title()
			if memoryInfo.NodeName != "" {
				currentTitle = fmt.Sprintf("%s - Node %s", a.Title(), memoryInfo.NodeName)
			}

			memoryInfo, ok := memoryInfo.Content.(collect.MemoryInfo)
			if !ok {
				return nil, errors.New("failed to convert interface to memory info")
			}

			checkCondition := func(when string) (bool, error) {
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

				switch operator {
				case "<":
					return memoryInfo.Total < uint64(desiredInt), nil
				case "<=":
					return memoryInfo.Total <= uint64(desiredInt), nil
				case ">":
					return memoryInfo.Total > uint64(desiredInt), nil
				case ">=":
					return memoryInfo.Total >= uint64(desiredInt), nil
				case "=", "==", "===":
					return memoryInfo.Total == uint64(desiredInt), nil
				}

				return false, errors.New("unknown operator")
			}

			analyzeResult, err := evaluateOutcomes(a.hostAnalyzer.Outcomes, checkCondition, currentTitle)
			if err != nil {
				return nil, errors.Wrap(err, "failed to evaluate outcomes")
			}
			results = append(results, analyzeResult...)
		}
	}
	return results, nil
}
