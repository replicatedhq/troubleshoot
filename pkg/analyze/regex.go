package analyzer

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
)

func analyzeRegex(analyzer *troubleshootv1beta1.RegEx, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	contents, err := getCollectedFileContents(path.Join(analyzer.CollectorName, analyzer.CollectorName+".txt"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get file contents for regex")
	}

	expression := regexp.MustCompile(analyzer.Expression)
	match := expression.FindStringSubmatch(string(contents))

	result := &AnalyzeResult{
		Title: analyzer.CheckName,
	}
	// to avoid empty strings in the UI..,.
	if result.Title == "" {
		result.Title = analyzer.CollectorName
	}

	foundMatches := map[string]string{}
	for i, name := range expression.SubexpNames() {
		if i != 0 && name != "" {
			foundMatches[name] = match[i]
		}
	}

	// allow fallthrough
	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}

			isMatch, err := compareRegex(outcome.Fail.When, foundMatches)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare regex fail conditional")
			}

			if isMatch {
				result.IsFail = true
				result.Message = outcome.Fail.Message
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

			isMatch, err := compareRegex(outcome.Warn.When, foundMatches)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare regex warn conditional")
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

			isMatch, err := compareRegex(outcome.Pass.When, foundMatches)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare regex pass conditional")
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

func compareRegex(conditional string, foundMatches map[string]string) (bool, error) {
	parts := strings.Split(strings.TrimSpace(conditional), " ")

	if len(parts) != 3 {
		return false, errors.New("unable to parse regex conditional")
	}

	lookForMatchName := parts[0]
	operator := parts[1]
	lookForValue := parts[2]

	foundValue, ok := foundMatches[lookForMatchName]
	if !ok {
		// not an error, just wasn't matched
		return false, nil
	}

	// if the value side of the conditional is an int, we assume it's an int
	lookForValueInt, err := strconv.Atoi(lookForValue)
	if err == nil {
		fmt.Printf("look for = %s, found = %s\n", lookForValue, foundValue)
		foundValueInt, err := strconv.Atoi(foundValue)
		if err != nil {
			// not an error but maybe it should be...
			return false, nil
		}

		switch operator {
		case "=":
			fallthrough
		case "==":
			fallthrough
		case "===":
			return lookForValueInt == foundValueInt, nil

		case "<":
			return lookForValueInt < foundValueInt, nil

		case ">":
			return lookForValueInt > foundValueInt, nil

		case "<=":
			return lookForValueInt <= foundValueInt, nil

		case ">=":
			return lookForValueInt >= foundValueInt, nil
		}
	} else {
		// all we can support is "=" and "==" and "===" for now
		if operator != "=" && operator != "==" && operator != "===" {
			return false, errors.New("unexpected operator in regex comparator")
		}

		return foundValue == lookForValue, nil
	}

	return false, nil
}
