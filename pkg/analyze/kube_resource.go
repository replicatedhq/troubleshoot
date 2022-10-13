package analyzer

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	iutils "github.com/replicatedhq/troubleshoot/pkg/interfaceutils"
	"gopkg.in/yaml.v2"
)

func analyzeResource(analyzer *troubleshootv1beta2.ClusterResource, getFileContents func(string) (map[string][]byte, error)) (*AnalyzeResult, error) {

	filemap := make(map[string]string)

	filemap["Deployment"] = "deployments"
	filemap["StatefulSet"] = "statefulsets"

	targetFolder, ok := filemap[analyzer.Kind]

	if ok != true {
		return nil, errors.New("failed to find resource")
	}

	files, err := getFileContents(filepath.Join("cluster-resources", targetFolder, fmt.Sprintf("%s.json", analyzer.Namespace)))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read collected deployments from namespace")
	}

	var resource interface{}
	for _, file := range files {
		err = yaml.Unmarshal(file, &resource)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse data as yaml doc")
		}
	}

	var selected interface{}
	items, _ := iutils.GetAtPath(resource, "items")
	itemslice := items.([]interface{})
	for _, item := range itemslice {
		name, _ := iutils.GetAtPath(item, "metadata.name")
		if name == analyzer.Name {
			selected = item
		}
	}

	actual, err := iutils.GetAtPath(selected, analyzer.YamlPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get object at path: %s", analyzer.YamlPath)
	}

	var expected interface{}
	err = yaml.Unmarshal([]byte(analyzer.Value), &expected)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse expected value as yaml doc")
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
