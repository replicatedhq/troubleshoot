package analyzer

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type httpResult struct {
	Error    *collect.HTTPError
	Response *collect.HTTPResponse
}

type AnalyzeHostHTTP struct {
	hostAnalyzer *troubleshootv1beta2.HTTPAnalyze
}

func (a *AnalyzeHostHTTP) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "HTTP Request")
}

func (a *AnalyzeHostHTTP) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostHTTP) Analyze(getCollectedFileContents func(string) ([]byte, error)) ([]*AnalyzeResult, error) {
	hostAnalyzer := a.hostAnalyzer

	name := filepath.Join("http", "result.json")
	if hostAnalyzer.CollectorName != "" {
		name = filepath.Join("http", hostAnalyzer.CollectorName+".json")
	}
	contents, err := getCollectedFileContents(name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get collected file")
	}

	httpInfo := &httpResult{}
	if err := json.Unmarshal(contents, httpInfo); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal http result")
	}

	result := &AnalyzeResult{
		Title: a.Title(),
	}

	for _, outcome := range hostAnalyzer.Outcomes {
		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return []*AnalyzeResult{result}, nil
			}

			isMatch, err := compareHostHTTPConditionalToActual(outcome.Fail.When, httpInfo)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %s", outcome.Fail.When)
			}

			if isMatch {
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

			isMatch, err := compareHostHTTPConditionalToActual(outcome.Warn.When, httpInfo)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %s", outcome.Warn.When)
			}

			if isMatch {
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

			isMatch, err := compareHostHTTPConditionalToActual(outcome.Pass.When, httpInfo)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %s", outcome.Pass.When)
			}

			if isMatch {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return []*AnalyzeResult{result}, nil
			}

		}
	}

	return []*AnalyzeResult{result}, nil
}

func compareHostHTTPConditionalToActual(conditional string, result *httpResult) (res bool, err error) {
	if conditional == "error" {
		return result.Error != nil, nil
	}

	parts := strings.Split(conditional, " ")
	if len(parts) != 3 {
		return false, fmt.Errorf("Failed to parse conditional: got %d parts", len(parts))
	}

	if parts[0] != "statusCode" {
		return false, errors.New(`Conditional must begin with keyword "statusCode"`)
	}

	if parts[1] != "=" && parts[1] != "==" && parts[1] != "===" {
		return false, errors.New(`Only supported operator is "=="`)
	}

	i, err := strconv.Atoi(parts[2])
	if err != nil {
		return false, err
	}

	if result.Response == nil {
		return false, err
	}
	return result.Response.Status == i, nil
}
