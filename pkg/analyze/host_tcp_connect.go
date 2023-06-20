package analyzer

import (
	"encoding/json"
	"path/filepath"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeHostTCPConnect struct {
	hostAnalyzer *troubleshootv1beta2.TCPConnectAnalyze
}

func (a *AnalyzeHostTCPConnect) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "TCP Connection Attempt")
}

func (a *AnalyzeHostTCPConnect) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostTCPConnect) Analyze(getCollectedFileContents func(string) ([]byte, error)) ([]*AnalyzeResult, error) {
	hostAnalyzer := a.hostAnalyzer

	collectorName := hostAnalyzer.CollectorName
	if collectorName == "" {
		collectorName = "connect"
	}
	fullPath := filepath.Join("host-collectors/connect", collectorName+".json")

	collected, err := getCollectedFileContents(fullPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read collected file name: %s", fullPath)
	}
	actual := collect.NetworkStatusResult{}
	if err := json.Unmarshal(collected, &actual); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal collected")
	}

	var coll resultCollector
	result := &AnalyzeResult{Title: a.Title()}

	for _, outcome := range hostAnalyzer.Outcomes {

		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				coll.push(result)
				break
			}

			if string(actual.Status) == outcome.Fail.When {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				coll.push(result)
				break
			}
		} else if outcome.Warn != nil {
			if outcome.Warn.When == "" {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				coll.push(result)
				break
			}

			if string(actual.Status) == outcome.Warn.When {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				coll.push(result)
				break
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				coll.push(result)
				break
			}

			if string(actual.Status) == outcome.Pass.When {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				coll.push(result)
				break
			}
		}
	}

	return coll.get(a.Title()), nil
}
