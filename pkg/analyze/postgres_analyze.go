package analyzer

import (
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

func analyzePostgresAnalyze(analyzer *troubleshootv1beta1.PostgresAnalyze, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	collectorName := analyzer.CollectorName
	if collectorName == "" {
		collectorName = "postgres"
	}

	fullPath := path.Join("postgres", fmt.Sprintf("%s.json", collectorName))

	collected, err := getCollectedFileContents(fullPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read collected file name: %s", fullPath)
	}

	databaseConnection := collect.DatabaseConnection{}
	if err := json.Unmarshal(collected, &databaseConnection); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal databased connection result")
	}

	checkName := analyzer.CheckName
	if checkName == "" {
		checkName = collectorName
	}

	result := &AnalyzeResult{
		Title:   checkName,
		IconKey: "kubernetes_postgres_analyze",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/postgres-analyze.svg",
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
				return result, errors.Wrap(err, "failed to compare postgres database conditional")
			}

			if isMatch {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}
		} else if outcome.Warn != nil {
			if outcome.Pass.When == "" {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return result, nil
			}

			isMatch, err := compareDatabaseConditionalToActual(outcome.Warn.When, &databaseConnection)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare postgres database conditional")
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
				return result, errors.Wrap(err, "failed to compare postgres database conditional")
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

func compareDatabaseConditionalToActual(conditional string, result *collect.DatabaseConnection) (bool, error) {
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
		expectedRange, err := semver.ParseRange(fmt.Sprintf("%s %s", parts[1], parts[2]))
		if err != nil {
			return false, errors.Wrap(err, "failed to parse semver range")
		}

		actual, err := semver.Parse(result.Version)
		if err != nil {
			return false, errors.Wrap(err, "failed to parse actual psotgres version")
		}

		return expectedRange(actual), nil
	}

	return false, nil
}
