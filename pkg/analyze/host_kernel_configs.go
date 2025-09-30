package analyzer

import (
	"encoding/json"
	"fmt"
	"regexp"

	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"k8s.io/klog/v2"
)

type AnalyzeHostKernelConfigs struct {
	hostAnalyzer *troubleshootv1beta2.KernelConfigsAnalyze
}

var kConfigRegex = regexp.MustCompile("^(CONFIG_[A-Z0-9_]+)=([ymn]+)$")

func (a *AnalyzeHostKernelConfigs) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Kernel Configs")
}

func (a *AnalyzeHostKernelConfigs) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostKernelConfigs) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	collectedContents, err := retrieveCollectedContents(
		getCollectedFileContents,
		collect.HostKernelConfigsPath,
		collect.NodeInfoBaseDir,
		collect.HostKernelConfigsFileName,
	)
	if err != nil {
		return []*AnalyzeResult{{Title: a.Title()}}, err
	}

	var results []*AnalyzeResult

	for _, content := range collectedContents {
		currentTitle := a.Title()
		if content.NodeName != "" {
			currentTitle = fmt.Sprintf("%s - Node %s", a.Title(), content.NodeName)
		}
		result, err := a.analyzeSingleNode(content, currentTitle)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to analyze kernel configs for %s", currentTitle)
		}
		if result != nil {
			results = append(results, result...)
		}
	}

	return results, nil
}

func addMissingKernelConfigs(message string, missingConfigs []string) string {
	if message == "" && len(missingConfigs) == 0 {
		return message
	}
	return strings.ReplaceAll(message, "{{ .ConfigsNotFound }}", strings.Join(missingConfigs, ", "))
}

func (a *AnalyzeHostKernelConfigs) analyzeSingleNode(content collectedContent, currentTitle string) ([]*AnalyzeResult, error) {
	hostAnalyzer := a.hostAnalyzer
	kConfigs := collect.KConfigs{}
	if err := json.Unmarshal(content.Data, &kConfigs); err != nil {
		return nil, errors.Wrap(err, "failed to read kernel configs")
	}

	var configsNotFound []string
	for _, config := range hostAnalyzer.SelectedConfigs {
		matches := kConfigRegex.FindStringSubmatch(config)
		// zero tolerance for invalid kernel config
		if len(matches) < 3 {
			return nil, errors.Errorf("invalid kernel config: %s", config)
		}

		key := matches[1]
		values := matches[2] // values can contain multiple values in any order y, m, n

		// check if the kernel config exists
		if _, ok := kConfigs[key]; !ok {
			configsNotFound = append(configsNotFound, config)
			continue
		}
		// check if the kernel config value matches
		if !strings.Contains(values, kConfigs[key]) {
			klog.V(2).Infof("collected kernel config %s=%s does not in expected values %s", key, kConfigs[key], values)
			configsNotFound = append(configsNotFound, config)
		}
	}

	var results []*AnalyzeResult
	for _, outcome := range hostAnalyzer.Outcomes {
		result := &AnalyzeResult{
			Title:  currentTitle,
			Strict: a.hostAnalyzer.Strict.BoolOrDefaultFalse(),
		}

		if outcome.Pass != nil && len(configsNotFound) == 0 {
			result.IsPass = true
			result.Message = outcome.Pass.Message
			results = append(results, result)
			break
		}

		if outcome.Fail != nil && len(configsNotFound) > 0 {
			result.IsFail = true
			result.Message = addMissingKernelConfigs(outcome.Fail.Message, configsNotFound)
			results = append(results, result)
			break
		}

	}

	return results, nil
}
