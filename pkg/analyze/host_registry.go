package analyzer

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeHostRegistryImages struct {
	hostAnalyzer *troubleshootv1beta2.HostRegistryImagesAnalyze
}

func (a *AnalyzeHostRegistryImages) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Registry Images")
}

func (a *AnalyzeHostRegistryImages) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostRegistryImages) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	collectorName := a.hostAnalyzer.CollectorName
	if collectorName == "" {
		collectorName = "images"
	}

	const nodeBaseDir = "host-collectors/registry-images"
	localPath := fmt.Sprintf("%s/%s.json", nodeBaseDir, collectorName)
	fileName := fmt.Sprintf("%s.json", collectorName)

	collectedContents, err := retrieveCollectedContents(
		getCollectedFileContents,
		localPath,
		nodeBaseDir,
		fileName,
	)
	if err != nil {
		return []*AnalyzeResult{{Title: a.Title()}}, err
	}

	results, err := analyzeHostCollectorResults(
		collectedContents,
		a.hostAnalyzer.Outcomes,
		a.CheckCondition,
		a.Title(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to analyze host registry images")
	}

	return results, nil
}

func (a *AnalyzeHostRegistryImages) CheckCondition(when string, data []byte) (bool, error) {
	var registryInfo collect.RegistryInfo
	if err := json.Unmarshal(data, &registryInfo); err != nil {
		return false, errors.Wrap(err, "failed to unmarshal registry info")
	}

	numMissing, numVerified, numErrors := 0, 0, 0
	for _, img := range registryInfo.Images {
		if img.Error != "" {
			numErrors++
		} else if !img.Exists {
			numMissing++
		} else {
			numVerified++
		}
	}

	return compareRegistryConditionalToActual(when, numVerified, numMissing, numErrors)
}
