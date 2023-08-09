package analyzer

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strconv"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	iutils "github.com/replicatedhq/troubleshoot/pkg/interfaceutils"
	"k8s.io/client-go/util/jsonpath"
)

type AnalyzeJsonCompare struct {
	analyzer *troubleshootv1beta2.JsonCompare
}

func (a *AnalyzeJsonCompare) Title() string {
	title := a.analyzer.CheckName
	if title == "" {
		title = a.analyzer.CollectorName
	}

	return title
}

func (a *AnalyzeJsonCompare) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeJsonCompare) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	result, err := a.analyzeJsonCompare(a.analyzer, getFile)
	if err != nil {
		return nil, err
	}
	result.Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	return []*AnalyzeResult{result}, nil
}

func (a *AnalyzeJsonCompare) analyzeJsonCompare(analyzer *troubleshootv1beta2.JsonCompare, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	fullPath := filepath.Join(analyzer.CollectorName, analyzer.FileName)
	collected, err := getCollectedFileContents(fullPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read collected file name: %s", fullPath)
	}

	var actual interface{}
	err = json.Unmarshal(collected, &actual)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse collected data as json")
	}

	if analyzer.Path != "" {
		actual, err = iutils.GetAtPath(actual, analyzer.Path)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get object at path: %s", analyzer.Path)
		}
	} else if analyzer.JsonPath != "" {
		jsp := jsonpath.New(analyzer.CheckName)
		jsp.AllowMissingKeys(true).EnableJSONOutput(true)
		err = jsp.Parse(analyzer.JsonPath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse jsonpath: %s", analyzer.JsonPath)
		}

		var data bytes.Buffer
		err = jsp.Execute(&data, actual)
		if err != nil {
			return nil, errors.Wrap(err, "failed to execute jsonpath")
		}

		err = json.NewDecoder(&data).Decode(&actual)
		if err != nil {
			return nil, errors.Wrap(err, "failed to decode jsonpath result")
		}

		// If we get back a single result in a slice unwrap it.
		// Technically this doesn't strictly follow jsonpath, but it makes
		// things easier downstream. Basically we don't want to require
		// users to wrap a single result with [].
		if a, ok := actual.([]interface{}); ok && len(a) == 1 {
			actual = a[0]
		}
	}

	var expected interface{}
	err = json.Unmarshal([]byte(analyzer.Value), &expected)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse expected value as json")
	}

	result := &AnalyzeResult{
		Title:   a.Title(),
		IconKey: "kubernetes_text_analyze",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
	}

	equal := reflect.DeepEqual(actual, expected)

	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			when := false
			if outcome.Fail.When != "" {
				when, err = strconv.ParseBool(outcome.Fail.When)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to process when statement: %s", outcome.Fail.When)
				}
			}

			if when == equal {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}
		} else if outcome.Warn != nil {
			when := false
			if outcome.Warn.When != "" {
				when, err = strconv.ParseBool(outcome.Warn.When)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to process when statement: %s", outcome.Warn.When)
				}
			}

			if when == equal {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return result, nil
			}
		} else if outcome.Pass != nil {
			when := true // default to passing when values are equal
			if outcome.Pass.When != "" {
				when, err = strconv.ParseBool(outcome.Pass.When)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to process when statement: %s", outcome.Pass.When)
				}
			}

			if when == equal {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return result, nil
			}
		}
	}

	return &AnalyzeResult{
		Title:   a.Title(),
		IconKey: "kubernetes_text_analyze",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
		IsFail:  true,
		Message: "Invalid analyzer",
	}, nil
}
