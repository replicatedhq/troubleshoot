package analyzer

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeHostSubnetAvailable struct {
	hostAnalyzer *troubleshootv1beta2.SubnetAvailableAnalyze
}

func (a *AnalyzeHostSubnetAvailable) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Subnet Available")
}

func (a *AnalyzeHostSubnetAvailable) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostSubnetAvailable) Analyze(getCollectedFileContents func(string) ([]byte, error)) ([]*AnalyzeResult, error) {
	hostAnalyzer := a.hostAnalyzer

	name := filepath.Join("host-collectors/subnetAvailable", "result.json")
	if hostAnalyzer.CollectorName != "" {
		name = filepath.Join("host-collectors/subnetAvailable", hostAnalyzer.CollectorName+".json")
	}
	contents, err := getCollectedFileContents(name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get collected file")
	}

	isSubnetAvailable := &collect.SubnetAvailableResult{}
	if err := json.Unmarshal(contents, isSubnetAvailable); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal subnetAvailable result")
	}

	fmt.Printf("%+v\n", isSubnetAvailable)

	result := &AnalyzeResult{
		Title: a.Title(),
	}

	for _, outcome := range hostAnalyzer.Outcomes {
		fmt.Printf("fail: %+v pass: %+v\n", outcome.Fail, outcome.Pass)
		if outcome.Fail != nil {
			fmt.Printf("fail.when: %s\n", outcome.Fail.When)
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return []*AnalyzeResult{result}, nil
			}

			if string(isSubnetAvailable.Status) == outcome.Fail.When {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return []*AnalyzeResult{result}, nil
			}
		} else if outcome.Pass != nil {
			fmt.Printf("pass.when: %s\n", outcome.Pass.When)
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return []*AnalyzeResult{result}, nil
			}

			if string(isSubnetAvailable.Status) == outcome.Pass.When {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return []*AnalyzeResult{result}, nil
			}
		}
	}

	return []*AnalyzeResult{result}, nil
}
