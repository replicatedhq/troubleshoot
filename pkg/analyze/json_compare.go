package analyzer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
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

	// due to jsp.Execute may return a slice of results unsorted, we need to sort the slice before comparing
	equal := deepEqualWithSlicesSorted(actual, expected)

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

// deepEqualWithSlicesSorted compares two interfaces and returns true if they contain the same values
// If the interfaces are slices, they are sorted before comparison to ensure order does not matter
// If the interfaces are not slices, reflect.DeepEqual is used
func deepEqualWithSlicesSorted(actual, expected interface{}) bool {
	ra, re := reflect.ValueOf(actual), reflect.ValueOf(expected)

	// If types are different, they're not equal
	if ra.Kind() != re.Kind() {
		return false
	}

	// If types are slices, compare sorted slices
	if ra.Kind() == reflect.Slice {
		return compareSortedSlices(ra.Interface().([]interface{}), re.Interface().([]interface{}))
	}

	// Otherwise, compare values (reflect.DeepEqual)
	return reflect.DeepEqual(actual, expected)
}

// compareSortedSlices compares two sorted slices of interfaces and returns true if they contain the same values
func compareSortedSlices(actual, expected []interface{}) bool {
	if len(actual) != len(expected) {
		return false
	}

	// Sort slices
	sortSliceOfInterfaces(actual)
	sortSliceOfInterfaces(expected)

	// Compare slices (reflect.DeepEqual)
	return reflect.DeepEqual(actual, expected)
}

func sortSliceOfInterfaces(slice []interface{}) {
	sort.Slice(slice, func(i, j int) bool {
		return order(slice[i], slice[j])
	})
}

// order function determines the order of two interface{} values
func order(a, b interface{}) bool {
	switch va := a.(type) {
	case int:
		if vb, ok := b.(int); ok {
			return va < vb
		}
	case float64:
		if vb, ok := b.(float64); ok {
			return va < vb
		}
	case string:
		if vb, ok := b.(string); ok {
			return va < vb
		}
	case bool:
		if vb, ok := b.(bool); ok {
			return !va && vb // false < true
		}
	}
	// use string representation for comparison
	return fmt.Sprintf("%v", a) < fmt.Sprintf("%v", b)
}
