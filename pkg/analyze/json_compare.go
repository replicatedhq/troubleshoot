package analyzer

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strconv"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	iutils "github.com/replicatedhq/troubleshoot/pkg/interfaceutils"
)

func analyzeJsonCompare(analyzer *troubleshootv1beta2.JsonCompare, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
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
	}

	var expected interface{}
	err = json.Unmarshal([]byte(analyzer.Value), &expected)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse expected value as json")
	}

	title := analyzer.CheckName
	if title == "" {
		title = analyzer.CollectorName
	}

	result := &AnalyzeResult{
		Title:   title,
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
		Title:   title,
		IconKey: "kubernetes_text_analyze",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
		IsFail:  true,
		Message: "Invalid analyzer",
	}, nil
}
