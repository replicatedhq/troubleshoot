package analyzer

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
)

type AnalyzeHostKernelConfigs struct {
	hostAnalyzer *troubleshootv1beta2.KernelConfigsAnalyze
}

func (a *AnalyzeHostKernelConfigs) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Kernel Configs")
}

func (a *AnalyzeHostKernelConfigs) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostKernelConfigs) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	hostAnalyzer := a.hostAnalyzer

	contents, err := getCollectedFileContents(collect.HostKernelConfigsPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get collected file")
	}

	kConfigs := collect.KConfigs{}
	if err := json.Unmarshal(contents, &kConfigs); err != nil {
		return nil, errors.Wrap(err, "failed to read kernel configs")
	}

	var results []*AnalyzeResult
	for _, outcome := range hostAnalyzer.Outcomes {
		result := &AnalyzeResult{
			Title:  a.Title(),
			Strict: hostAnalyzer.Strict.BoolOrDefaultFalse(),
		}

		if err := analyzeSingleOutcome(kConfigs, result, outcome.Pass, constants.OUTCOME_PASS); err != nil {
			return nil, errors.Wrap(err, "failed to analyze pass outcome")
		}

		if err := analyzeSingleOutcome(kConfigs, result, outcome.Fail, constants.OUTCOME_FAIL); err != nil {
			return nil, errors.Wrap(err, "failed to analyze fail outcome")
		}

		if err := analyzeSingleOutcome(kConfigs, result, outcome.Warn, constants.OUTCOME_WARN); err != nil {
			return nil, errors.Wrap(err, "failed to analyze warn outcome")
		}

		results = append(results, result)
	}

	return results, nil
}

func analyzeSingleOutcome(kConfigs collect.KConfigs, result *AnalyzeResult, outcome *troubleshootv1beta2.SingleOutcome, outcomeType string) error {
	if outcome == nil {
		return nil
	}

	if outcome.When == "" {
		return errors.New("when attribute is required")
	}

	isMatch, err := match(kConfigs, outcome.When)
	if err != nil {
		return errors.Wrap(err, "failed to match")
	}

	result.Message = outcome.Message
	result.URI = outcome.URI

	if !isMatch {
		return nil
	}

	switch outcomeType {
	case constants.OUTCOME_PASS:
		result.IsPass = true
	case constants.OUTCOME_FAIL:
		result.IsFail = true
	case constants.OUTCOME_WARN:
		result.IsWarn = true
	}

	return nil
}

func match(kConfigs collect.KConfigs, when string) (bool, error) {
	parts := strings.SplitN(when, "=", 2)
	if len(parts) != 2 {
		return false, errors.New("invalid when attribute")
	}
	key, value := parts[0], parts[1]

	// check if the key exists
	if kConfig, ok := kConfigs[key]; ok {
		return kConfig == strings.TrimSpace(value), nil
	}

	// kernel config not found
	return false, nil
}
