package analyzer

import (
	"encoding/json"
	"fmt"
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

func (a *AnalyzeHostHTTP) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {

	collectorName := a.hostAnalyzer.CollectorName
	if collectorName == "" {
		collectorName = "result"
	}

	const nodeBaseDir = "host-collectors/http"
	localPath := fmt.Sprintf("%s/%s.json", nodeBaseDir, collectorName)
	fileName := fmt.Sprintf("%s.json", collectorName)

	collectedContents, err := retrieveCollectedContents(
		getCollectedFileContents,
		localPath,
		nodeBaseDir,
		fileName,
	)
	if err != nil {
		return []*AnalyzeResult{{Title: a.Title()}}, err
	}

	results, err := analyzeHostCollectorResults(collectedContents, a.hostAnalyzer.Outcomes, a.CheckCondition, a.Title())
	if err != nil {
		return nil, errors.Wrap(err, "failed to analyze http request")
	}

	return results, nil
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

func analyzeHTTPResult(analyzer *troubleshootv1beta2.HTTPAnalyze, fileName string, getCollectedFileContents getCollectedFileContents, title string) ([]*AnalyzeResult, error) {
	contents, err := getCollectedFileContents(fileName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get collected file")
	}

	httpInfo := &httpResult{}
	if err := json.Unmarshal(contents, httpInfo); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal http result")
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

func (a *AnalyzeHostHTTP) CheckCondition(when string, data []byte) (bool, error) {

	var httpInfo httpResult
	if err := json.Unmarshal(data, &httpInfo); err != nil {
		return false, fmt.Errorf("failed to unmarshal data into httpResult: %v", err)
	}
	return compareHostHTTPConditionalToActual(when, &httpInfo)
}
