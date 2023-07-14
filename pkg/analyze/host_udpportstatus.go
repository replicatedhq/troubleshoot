package analyzer

import (
	"encoding/json"
	"fmt"
	"path"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

// AnalyzeHostUDPPortStatus is an analyzer that will return only one matching result, or a warning if nothing matches. The first
// match that is encountered is the one that is returned.
type AnalyzeHostUDPPortStatus struct {
	hostAnalyzer *troubleshootv1beta2.UDPPortStatusAnalyze
}

func (a *AnalyzeHostUDPPortStatus) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "UDP Port Status")
}

func (a *AnalyzeHostUDPPortStatus) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostUDPPortStatus) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	hostAnalyzer := a.hostAnalyzer

	fullPath := path.Join("host-collectors/udpPortStatus", "udpPortStatus.json")
	if hostAnalyzer.CollectorName != "" {
		fullPath = path.Join("host-collectors/udpPortStatus", fmt.Sprintf("%s.json", hostAnalyzer.CollectorName))
	}

	collected, err := getCollectedFileContents(fullPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read collected file name: %s", fullPath)
	}
	actual := collect.NetworkStatusResult{}
	if err := json.Unmarshal(collected, &actual); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal collected")
	}

	result := &AnalyzeResult{Title: a.Title()}

	for _, outcome := range hostAnalyzer.Outcomes {

		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return []*AnalyzeResult{result}, nil
			}

			if string(actual.Status) == outcome.Fail.When {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return []*AnalyzeResult{result}, nil
			}
		} else if outcome.Warn != nil {
			if outcome.Warn.When == "" {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return []*AnalyzeResult{result}, nil
			}

			if string(actual.Status) == outcome.Warn.When {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return []*AnalyzeResult{result}, nil
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return []*AnalyzeResult{result}, nil
			}

			if string(actual.Status) == outcome.Pass.When {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return []*AnalyzeResult{result}, nil
			}
		}
	}

	return []*AnalyzeResult{result}, nil
}
