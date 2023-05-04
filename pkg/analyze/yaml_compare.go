package analyzer

import (
	"path/filepath"
	"reflect"
	"strconv"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	iutils "github.com/replicatedhq/troubleshoot/pkg/interfaceutils"
	"gopkg.in/yaml.v2"
)

type AnalyzeYamlCompare struct {
	analyzer *troubleshootv1beta2.YamlCompare
}

func (a *AnalyzeYamlCompare) Title() string {
	title := a.analyzer.CheckName
	if title == "" {
		title = a.analyzer.CollectorName
	}

	return title
}

func (a *AnalyzeYamlCompare) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeYamlCompare) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	result, err := a.analyzeYamlCompare(a.analyzer, getFile)
	if err != nil {
		return nil, err
	}
	result.Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	return []*AnalyzeResult{result}, nil
}

func (a *AnalyzeYamlCompare) analyzeYamlCompare(analyzer *troubleshootv1beta2.YamlCompare, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	fullPath := filepath.Join(analyzer.CollectorName, analyzer.FileName)
	collected, err := getCollectedFileContents(fullPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read collected file name: %s", fullPath)
	}

	var actual interface{}
	err = yaml.Unmarshal(collected, &actual)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse collected data as yaml doc")
	}

	if analyzer.Path != "" {
		actual, err = iutils.GetAtPath(actual, analyzer.Path)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get object at path: %s", analyzer.Path)
		}
	}

	var expected interface{}
	err = yaml.Unmarshal([]byte(analyzer.Value), &expected)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse expected value as yaml doc")
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
