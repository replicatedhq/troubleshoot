package analyzer

import (
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeMssql struct {
	analyzer *troubleshootv1beta2.DatabaseAnalyze
}

func (a *AnalyzeMssql) Title() string {
	title := a.analyzer.CheckName
	if title == "" {
		title = a.collectorName()
	}

	return title
}

func (a *AnalyzeMssql) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeMssql) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	result, err := analyzeMssql(a.analyzer, getFile)
	if err != nil {
		return nil, err
	}
	result.Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	return []*AnalyzeResult{result}, nil
}

func (a *AnalyzeMssql) collectorName() string {
	if a.analyzer.CollectorName != "" {
		return a.analyzer.CollectorName
	}
	return "mssql"
}

func compareMssqlConditionalToActual(conditional string, result *collect.DatabaseConnection) (bool, error) {
	parts := strings.Split(strings.TrimSpace(conditional), " ")

	if len(parts) != 3 {
		return false, errors.New("unable to parse conditional")
	}

	switch parts[0] {
	case "connected":
		expected, err := strconv.ParseBool(parts[2])
		if err != nil {
			return false, errors.Wrap(err, "failed to parse bool")
		}

		switch parts[1] {
		case "=", "==", "===":
			return expected == result.IsConnected, nil
		case "!=", "!==":
			return expected != result.IsConnected, nil

		}

		return false, errors.New("unable to parse postgres connected analyzer")

	case "version":
		expected, err := version.NewVersion(strings.Replace(parts[2], "x", "0", -1))
		if err != nil {
			return false, errors.Wrap(err, "failed to parse expected version")
		}

		operation := parts[1]
		switch operation {
		case "=", "==", "===":
			operation = "="
		case "!=", "!==":
			operation = "!="
		}

		actual, err := version.NewVersion(strings.Replace(result.Version, "x", "0", -1))
		if err != nil {
			return false, errors.Wrap(err, "failed to parse mssql db actual version")
		}

		constraints, err := version.NewConstraint(fmt.Sprintf("%s %s", operation, expected))
		if err != nil {
			return false, errors.Wrap(err, "failed to create constraint")
		}
		return constraints.Check(actual), nil
	}

	return false, nil
}

func analyzeMssql(analyzer *troubleshootv1beta2.DatabaseAnalyze, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	collectorName := analyzer.CollectorName
	if collectorName == "" {
		collectorName = "mssql"
	}

	fullPath := path.Join("mssql", fmt.Sprintf("%s.json", collectorName))

	collected, err := getCollectedFileContents(fullPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read collected file name: %s", fullPath)
	}

	databaseConnection := collect.DatabaseConnection{}
	if err := json.Unmarshal(collected, &databaseConnection); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal databased connection result")
	}

	title := analyzer.CheckName
	if title == "" {
		title = collectorName
	}

	result := &AnalyzeResult{
		Title: title,
	}

	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}

			isMatch, err := compareMssqlConditionalToActual(outcome.Fail.When, &databaseConnection)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare MS SQL Server database conditional")
			}

			if isMatch {

				if databaseConnection.Error != "" {
					result.Message = outcome.Fail.Message + " " + databaseConnection.Error
				} else {
					result.Message = outcome.Fail.Message
				}

				result.IsFail = true
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

			isMatch, err := compareMssqlConditionalToActual(outcome.Warn.When, &databaseConnection)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare MS SQL Server database conditional")
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

			isMatch, err := compareMssqlConditionalToActual(outcome.Pass.When, &databaseConnection)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare MS SQL Server database conditional")
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
