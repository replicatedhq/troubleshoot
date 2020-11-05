package analyzer

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type Conditional struct {
	method           string
	lookForMatchName string
	operator         string
	lookForValue     string
}

func analyzeTextAnalyze(analyzer *troubleshootv1beta2.TextAnalyze, getCollectedFileContents func(string) (map[string][]byte, error)) ([]*AnalyzeResult, error) {
	fullPath := filepath.Join(analyzer.CollectorName, analyzer.FileName)
	collected, err := getCollectedFileContents(fullPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read collected file name: %s", fullPath)
	}

	checkName := analyzer.CheckName
	if checkName == "" {
		checkName = analyzer.CollectorName
	}

	if len(collected) == 0 {
		return []*AnalyzeResult{
			{
				Title:   checkName,
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
				IsFail:  false,
				Message: "No matching files",
			},
		}, nil
	}

	results := []*AnalyzeResult{}

	if analyzer.RegexPattern != "" {
		for _, fileContents := range collected {
			result, err := analyzeRegexPattern(analyzer.RegexPattern, fileContents, analyzer.Outcomes, checkName)
			if err != nil {
				return nil, err
			}
			if result != nil {
				results = append(results, result)
			}
		}
	}

	if analyzer.RegexGroups != "" {
		for _, fileContents := range collected {
			result, err := analyzeRegexGroups(analyzer.RegexGroups, fileContents, analyzer.Outcomes, checkName)
			if err != nil {
				return nil, err
			}
			if result != nil {
				results = append(results, result)
			}
		}
	}

	if len(results) > 0 {
		return results, nil
	}

	return []*AnalyzeResult{
		{
			Title:   checkName,
			IconKey: "kubernetes_text_analyze",
			IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			IsFail:  true,
			Message: "Invalid analyzer",
		},
	}, nil
}

func analyzeRegexPattern(pattern string, collected []byte, outcomes []*troubleshootv1beta2.Outcome, checkName string) (*AnalyzeResult, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to compile regex: %s", pattern)
	}

	var failOutcome *troubleshootv1beta2.Outcome
	var passOutcome *troubleshootv1beta2.Outcome
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

func analyzeRegexGroups(pattern string, collected []byte, outcomes []*troubleshootv1beta2.Outcome, checkName string) (*AnalyzeResult, error) {
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
		if i != 0 && name != "" && len(match) > i {
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
	parsedConditional, err := parseConditional(conditional)
	if err != nil {
		return false, err
	}

	foundValue, ok := foundMatches[parsedConditional.lookForMatchName]
	if !ok {
		// not an error, just wasn't matched
		return false, nil
	}

	if parsedConditional.method == "semverCompare" {
		return compareSemVer(parsedConditional.operator, foundValue, parsedConditional.lookForValue)
	} else if parsedConditional.method == "semverRange" {
		return compareSemVer("", foundValue, parsedConditional.lookForValue)
	} else if lookForValueInt, err := strconv.Atoi(parsedConditional.lookForValue); err == nil {
		// if the value side of the conditional is an int, we assume it's an int
		foundValueInt, err := strconv.Atoi(foundValue)
		if err != nil {
			// not an error but maybe it should be...
			return false, nil
		}
		switch parsedConditional.operator {
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
	}
	if parsedConditional.operator != "=" && parsedConditional.operator != "==" && parsedConditional.operator != "===" {
		return false, fmt.Errorf("unexpected operator %q in regex comparator, cannot compare %q and %q", parsedConditional.operator, foundValue, parsedConditional.lookForValue)
	}

	return foundValue == parsedConditional.lookForValue, nil
}

func compareSemVer(operator string, foundValue string, lookForValue string) (bool, error) {
	if operator != "" {
		expected, err := semver.ParseTolerant(strings.Replace(lookForValue, "x", "0", -1))
		if err != nil {
			return false, errors.Wrap(err, "failed to parse expected semantic version")
		}
		lookForValue = fmt.Sprintf("%s %s", operator, expected.String())
	}
	actual, err := semver.ParseTolerant(strings.Replace(foundValue, "x", "0", -1))
	if err != nil {
		return false, errors.Wrap(err, "failed to parse found semantic version")
	}
	expectedRange, err := semver.ParseRange(lookForValue)
	if err != nil {
		return false, errors.Wrap(err, "failed to parse semver range")
	}
	return expectedRange(actual), nil
}

func parseConditional(conditional string) (*Conditional, error) {
	parsedConditional := new(Conditional)
	if strings.Contains(conditional, "semverCompare") {
		rgx := regexp.MustCompile(`semverCompare\((?P<cond>[a-z"|<>=& ,0-9]+)\)`)
		rs := rgx.FindStringSubmatch(conditional)
		if rs == nil {
			return nil, errors.Errorf("Unable to parse semverCompare expresion \"%s\". Correct format is \"semverCompare(variable operator expectedVersion)\"", conditional)
		}
		parts := strings.Split(strings.TrimSpace(rs[1]), " ")
		parsedConditional.method = "semverCompare"
		parsedConditional.lookForMatchName = parts[0]
		parsedConditional.operator = parts[1]
		parsedConditional.lookForValue = parts[2]

	} else if strings.Contains(conditional, "semverRange") {
		rgx := regexp.MustCompile(`semverRange\((?P<cond>[a-z"|<>=&,0-9]+) ?, ?\"(?P<cond2>[a-z|<>=& .!,0-9]+)\"\)`)
		rs := rgx.FindStringSubmatch(conditional)
		if rs == nil {
			return nil, errors.Errorf("Unable to parse semverRange expresion \"%s\". Correct format is \"semverRange(variable, \"expectedRange\")\"", conditional)
		}

		parsedConditional.method = "semverRange"
		parsedConditional.lookForMatchName = rs[1]
		parsedConditional.operator = ""
		parsedConditional.lookForValue = rs[2]

	} else {
		parts := strings.Split(strings.TrimSpace(conditional), " ")
		if len(parts) != 3 {
			return nil, errors.New("unable to parse regex conditional")
		}
		parsedConditional.method = "default"
		parsedConditional.lookForMatchName = parts[0]
		parsedConditional.operator = parts[1]
		parsedConditional.lookForValue = parts[2]
	}
	return parsedConditional, nil
}
