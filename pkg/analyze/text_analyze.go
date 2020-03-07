package analyzer

import (
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
)

func analyzeTextAnalyze(analyzer *troubleshootv1beta1.TextAnalyze, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	fullPath := path.Join(analyzer.CollectorName, analyzer.FileName)
	collected, err := getCollectedFileContents(fullPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read collected file name: %s", fullPath)
	}

	checkName := analyzer.CheckName
	if checkName == "" {
		checkName = analyzer.CollectorName
	}

	if analyzer.RegexPattern != "" {
		return analyzeRegexPattern(analyzer.RegexPattern, collected, analyzer.Outcomes, checkName)
	}

	if analyzer.RegexGroups != "" {
		return analyzeRegexGroups(analyzer.RegexGroups, collected, analyzer.Outcomes, checkName)
	}

	return &AnalyzeResult{
		Title:   analyzer.CheckName,
		IconKey: "kubernetes_text_analyze",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
		IsFail:  true,
		Message: "Invalid analyzer",
	}, nil
}

func analyzeRegexPattern(pattern string, collected []byte, outcomes []*troubleshootv1beta1.Outcome, checkName string) (*AnalyzeResult, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to compile regex: %s", pattern)
	}

	var failOutcome *troubleshootv1beta1.Outcome
	var passOutcome *troubleshootv1beta1.Outcome
	for _, outcome := range outcomes {
		if outcome.Fail != nil {
			failOutcome = outcome
		} else if outcome.Pass != nil {
			passOutcome = outcome
		}
	}

	if re.MatchString(string(collected)) {
		return &AnalyzeResult{
			Title:   checkName,
			IsPass:  true,
			Message: passOutcome.Pass.Message,
			URI:     passOutcome.Pass.URI,
		}, nil
	}

	return &AnalyzeResult{
		Title:   checkName,
		IconKey: "kubernetes_text_analyze",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
		IsFail:  true,
		Message: failOutcome.Fail.Message,
		URI:     failOutcome.Fail.URI,
	}, nil
}

func analyzeRegexGroups(pattern string, collected []byte, outcomes []*troubleshootv1beta1.Outcome, checkName string) (*AnalyzeResult, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to compile regex: %s", pattern)
	}

	match := re.FindStringSubmatch(string(collected))

	result := &AnalyzeResult{
		Title:   checkName,
		IconKey: "kubernetes_text_analyze",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg?w=13&h=16",
	}

	foundMatches := map[string]string{}
	for i, name := range re.SubexpNames() {
		if i != 0 && name != "" {
			foundMatches[name] = match[i]
		}
	}

	// allow fallthrough
	for _, outcome := range outcomes {
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
			return foundValueInt == lookForValueInt, nil

		case "<":
			return foundValueInt < lookForValueInt, nil

		case ">":
			return foundValueInt > lookForValueInt, nil

		case "<=":
			return foundValueInt <= lookForValueInt, nil

		case ">=":
			return foundValueInt >= lookForValueInt, nil
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
