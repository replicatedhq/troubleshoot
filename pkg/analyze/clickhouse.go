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

type AnalyzeClickhouse struct {
	analyzer *troubleshootv1beta2.DatabaseAnalyze
}

func (a *AnalyzeClickhouse) Title() string {
	title := a.analyzer.CheckName
	if title == "" {
		title = a.collectorName()
	}

	return title
}

func (a *AnalyzeClickhouse) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeClickhouse) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	result, err := a.analyze(a.analyzer, getFile)
	if err != nil {
		return nil, err
	}
	result.Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	return []*AnalyzeResult{result}, nil
}

func (a *AnalyzeClickhouse) collectorName() string {
	if a.analyzer.CollectorName != "" {
		return a.analyzer.CollectorName
	}
	return "clickhouse"
}

func compareClickhouseConditionalToActual(conditional string, result *collect.DatabaseConnection) (bool, error) {
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

		return false, errors.New("unable to parse ClickHouse connected analyzer")

	case "version":
		expected, err := version.NewVersion(strings.ReplaceAll(parts[2], "x", "0"))
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

		actual, err := version.NewVersion(strings.ReplaceAll(result.Version, "x", "0"))
		if err != nil {
			return false, errors.Wrap(err, "failed to parse ClickHouse db actual version")
		}

		constraints, err := version.NewConstraint(fmt.Sprintf("%s %s", operation, expected))
		if err != nil {
			return false, errors.Wrap(err, "failed to create constraint")
		}
		return constraints.Check(actual), nil
	}

	return false, nil
}

func (a *AnalyzeClickhouse) analyze(analyzer *troubleshootv1beta2.DatabaseAnalyze, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	fullPath := path.Join("", fmt.Sprintf("clickhouse/%s.json", a.collectorName()))

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
		IconKey: "kubernetes__analyze",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/-analyze.svg",
	}

	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}

			isMatch, err := compareClickhouseConditionalToActual(outcome.Fail.When, &databaseConnection)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare ClickHouse database conditional")
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

			isMatch, err := compareClickhouseConditionalToActual(outcome.Warn.When, &databaseConnection)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare ClickHouse database conditional")
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

			isMatch, err := compareClickhouseConditionalToActual(outcome.Pass.When, &databaseConnection)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare ClickHouse database conditional")
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
