package analyzer

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeHostKernelModules struct {
	hostAnalyzer *troubleshootv1beta2.KernelModulesAnalyze
}

func (a *AnalyzeHostKernelModules) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Kernel Modules")
}

func (a *AnalyzeHostKernelModules) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostKernelModules) Analyze(getCollectedFileContents func(string) ([]byte, error)) ([]*AnalyzeResult, error) {
	hostAnalyzer := a.hostAnalyzer
	contents, err := getCollectedFileContents("system/kernel_modules.json")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get collected file")
	}
	modules := make(map[string]collect.KernelModuleInfo)
	if err := json.Unmarshal(contents, &modules); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal kernel modules")
	}

	var coll resultCollector

	for _, outcome := range hostAnalyzer.Outcomes {
		result := &AnalyzeResult{Title: a.Title()}

		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				coll.push(result)
				continue
			}

			isMatch, err := compareKernelModuleConditionalToActual(outcome.Fail.When, modules)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %s", outcome.Fail.When)
			}

			if isMatch {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				coll.push(result)
			}
		} else if outcome.Warn != nil {
			if outcome.Warn.When == "" {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				coll.push(result)
				continue
			}

			isMatch, err := compareKernelModuleConditionalToActual(outcome.Warn.When, modules)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %s", outcome.Warn.When)
			}

			if isMatch {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				coll.push(result)
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				coll.push(result)
				continue
			}

			isMatch, err := compareKernelModuleConditionalToActual(outcome.Pass.When, modules)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %s", outcome.Pass.When)
			}

			if isMatch {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				coll.push(result)
			}
		}
	}

	return coll.get(a.Title()), nil
}

func compareKernelModuleConditionalToActual(conditional string, modules map[string]collect.KernelModuleInfo) (res bool, err error) {
	parts := strings.Split(conditional, " ")
	if len(parts) != 3 {
		return false, fmt.Errorf("Expected exactly 3 parts in conditional, got %d", len(parts))
	}

	matchModules := strings.Split(parts[0], ",")
	matchStatuses := strings.Split(parts[2], ",")

	switch parts[1] {
	case "=", "==":
		for _, name := range matchModules {
			module, ok := modules[name]
			if !ok {
				return false, nil
			}
			moduleOK := false
			// Only one status must be true.
			for _, status := range matchStatuses {
				if module.Status == collect.KernelModuleStatus(status) {
					moduleOK = true
					continue
				}
			}
			if !moduleOK {
				return false, nil
			}
		}
		return true, nil
	case "!=", "<>":
		for _, name := range matchModules {
			module, ok := modules[name]
			if !ok {
				return true, nil
			}

			for _, status := range matchStatuses {
				if module.Status == collect.KernelModuleStatus(status) {
					return false, nil
				}
			}
		}
		return true, nil
	}

	return false, fmt.Errorf("unexpected operator %q", parts[1])
}
