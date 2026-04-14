package analyzer

import (
	"encoding/json"
	"fmt"
	"path"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeS3Status struct {
	analyzer *troubleshootv1beta2.DatabaseAnalyze
}

func (a *AnalyzeS3Status) Title() string {
	title := a.analyzer.CheckName
	if title == "" {
		title = a.collectorName()
	}

	return title
}

func (a *AnalyzeS3Status) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeS3Status) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	result, err := a.analyzeS3Status(a.analyzer, getFile)
	if err != nil {
		return nil, err
	}
	result.Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	return []*AnalyzeResult{result}, nil
}

func (a *AnalyzeS3Status) collectorName() string {
	collectorName := a.analyzer.CollectorName
	if collectorName == "" {
		collectorName = "s3Status"
	}

	return collectorName
}

func (a *AnalyzeS3Status) analyzeS3Status(analyzer *troubleshootv1beta2.DatabaseAnalyze, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	fullPath := path.Join("s3Status", fmt.Sprintf("%s.json", a.collectorName()))

	collected, err := getCollectedFileContents(fullPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read collected file name: %s", fullPath)
	}

	databaseConnection := collect.DatabaseConnection{}
	if err := json.Unmarshal(collected, &databaseConnection); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal s3 status result")
	}

	result := &AnalyzeResult{
		Title:   a.Title(),
		IconKey: "kubernetes_s3_analyze",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/s3-analyze.svg",
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
				return result, errors.Wrap(err, "failed to compare s3 status conditional")
			}

			if isMatch {
				result.IsFail = true
				if databaseConnection.Error != "" {
					result.Message = outcome.Fail.Message + " " + databaseConnection.Error
				} else {
					result.Message = outcome.Fail.Message
				}
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
				return result, errors.Wrap(err, "failed to compare s3 status conditional")
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
				return result, errors.Wrap(err, "failed to compare s3 status conditional")
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
