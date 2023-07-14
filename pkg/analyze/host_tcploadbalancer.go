package analyzer

import (
	"encoding/json"
	"fmt"
	"path"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeHostTCPLoadBalancer struct {
	hostAnalyzer *troubleshootv1beta2.TCPLoadBalancerAnalyze
}

func (a *AnalyzeHostTCPLoadBalancer) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "TCP Load Balancer")
}

func (a *AnalyzeHostTCPLoadBalancer) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostTCPLoadBalancer) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	hostAnalyzer := a.hostAnalyzer

	collectorName := hostAnalyzer.CollectorName
	if collectorName == "" {
		collectorName = "tcpLoadBalancer"
	}

	fullPath := path.Join("host-collectors/tcpLoadBalancer", fmt.Sprintf("%s.json", collectorName))

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
