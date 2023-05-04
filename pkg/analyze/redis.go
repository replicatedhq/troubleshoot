package analyzer

import (
	"encoding/json"
	"fmt"
	"path"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeRedis struct {
	analyzer *troubleshootv1beta2.DatabaseAnalyze
}

func (a *AnalyzeRedis) Title() string {
	title := a.analyzer.CheckName
	if title == "" {
		title = a.collectorName()
	}

	return title
}

func (a *AnalyzeRedis) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeRedis) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	result, err := a.analyzeRedis(a.analyzer, getFile)
	if err != nil {
		return nil, err
	}
	result.Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	return []*AnalyzeResult{result}, nil
}

func (a *AnalyzeRedis) collectorName() string {
	collectorName := a.analyzer.CollectorName
	if collectorName == "" {
		collectorName = "redis"
	}

	return collectorName
}

func (a *AnalyzeRedis) analyzeRedis(analyzer *troubleshootv1beta2.DatabaseAnalyze, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	fullPath := path.Join("redis", fmt.Sprintf("%s.json", a.collectorName()))

	collected, err := getCollectedFileContents(fullPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read collected file name: %s", fullPath)
	}

	databaseConnection := collect.DatabaseConnection{}
	if err := json.Unmarshal(collected, &databaseConnection); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal database connection result")
	}

	result := &AnalyzeResult{
		Title:   a.Title(),
		IconKey: "kubernetes_redis_analyze",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/redis-analyze.svg",
	}

	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}

			isMatch, err := compareDatabaseConditionalToActual(outcome.Fail.When, &databaseConnection)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare redis database conditional")
			}

			if isMatch {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}
		} else if outcome.Warn != nil {
			if outcome.Warn.When == "" {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return result, nil
			}

			isMatch, err := compareDatabaseConditionalToActual(outcome.Warn.When, &databaseConnection)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare redis database conditional")
			}

			if isMatch {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return result, nil
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return result, nil
			}

			isMatch, err := compareDatabaseConditionalToActual(outcome.Pass.When, &databaseConnection)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare redis database conditional")
			}

			if isMatch {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return result, nil
			}
		}
	}

	return result, nil
}
