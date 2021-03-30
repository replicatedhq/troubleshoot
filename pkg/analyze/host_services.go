package analyzer

import (
	"encoding/json"
	"fmt"
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

func (a *AnalyzeHostServices) Analyze(getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	hostAnalyzer := a.hostAnalyzer

	contents, err := getCollectedFileContents(collect.HostServicesPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get collected file")
	}

	var services []collect.ServiceInfo
	if err := json.Unmarshal(contents, &services); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal systemctl service info")
	}

	result := AnalyzeResult{}

	result.Title = a.Title()

	for _, outcome := range hostAnalyzer.Outcomes {
		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return &result, nil
			}

			isMatch, err := compareHostServicesConditionalToActual(outcome.Fail.When, services)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %s", outcome.Fail.When)
			}

			if isMatch {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return &result, nil
			}
		} else if outcome.Warn != nil {
			if outcome.Warn.When == "" {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return &result, nil
			}

			isMatch, err := compareHostServicesConditionalToActual(outcome.Warn.When, services)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %s", outcome.Warn.When)
			}

			if isMatch {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return &result, nil
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return &result, nil
			}

			isMatch, err := compareHostServicesConditionalToActual(outcome.Pass.When, services)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %s", outcome.Pass.When)
			}

			if isMatch {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return &result, nil
			}
		}
	}

	return &result, nil
}

// <service> <op> <state>
// example: ufw.service = active
func compareHostServicesConditionalToActual(conditional string, services []collect.ServiceInfo) (res bool, err error) {
	parts := strings.Split(conditional, " ")
	if len(parts) != 3 {
		return false, fmt.Errorf("expected exactly 3 parts, got %d", len(parts))
	}

	switch parts[1] {
	case "=", "==":
		for _, service := range services {
			if isServiceMatch(service.Unit, parts[0]) {
				return service.Active == parts[2], nil
			}
		}
		return false, nil
	case "!=", "<>":
		for _, service := range services {
			if isServiceMatch(service.Unit, parts[0]) {
				return service.Active != parts[2], nil
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
