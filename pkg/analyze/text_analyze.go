package analyzer

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type AnalyzeTextAnalyze struct {
	analyzer *troubleshootv1beta2.TextAnalyze
}

func (a *AnalyzeTextAnalyze) Title() string {
	checkName := a.analyzer.CheckName
	if checkName == "" {
		checkName = a.analyzer.CollectorName
	}

	return checkName
}

func (a *AnalyzeTextAnalyze) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeTextAnalyze) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	return analyzeTextAnalyze(a.analyzer, findFiles, a.Title())
}

func analyzeTextAnalyze(
	analyzer *troubleshootv1beta2.TextAnalyze, getCollectedFileContents getChildCollectedFileContents, title string,
) ([]*AnalyzeResult, error) {
	fullPath := filepath.Join(analyzer.CollectorName, analyzer.FileName)
	excludeFiles := []string{}
	for _, excludeFile := range analyzer.ExcludeFiles {
		excludeFiles = append(excludeFiles, filepath.Join(analyzer.CollectorName, excludeFile))
	}

	collected, err := getCollectedFileContents(fullPath, excludeFiles)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read collected file name: %s", fullPath)
	}

	if len(collected) == 0 {
		if analyzer.IgnoreIfNoFiles {
			return nil, nil
		}

		return []*AnalyzeResult{
			{
				Title:   title,
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
				IsWarn:  true,
				Message: "No matching files",
			},
		}, nil
	}

	results := []*AnalyzeResult{}

	if analyzer.RegexPattern != "" {
		for _, fileContents := range collected {
			result, err := analyzeRegexPattern(analyzer.RegexPattern, fileContents, analyzer.Outcomes, title)
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
			result, err := analyzeRegexGroups(analyzer.RegexGroups, fileContents, analyzer.Outcomes, title)
			if err != nil {
				return nil, err
			}
			if result != nil {
				results = append(results, result)
			}
		}
	}

	for i := range results {
		results[i].Strict = analyzer.Strict.BoolOrDefaultFalse()
	}

	if len(results) > 0 {
		return results, nil
	}

	return []*AnalyzeResult{
		{
			Title:   title,
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

	var failOutcome *troubleshootv1beta2.SingleOutcome
	var passOutcome *troubleshootv1beta2.SingleOutcome
	for _, outcome := range outcomes {
		if outcome.Fail != nil {
			failOutcome = outcome.Fail
		} else if outcome.Pass != nil {
			passOutcome = outcome.Pass
		}
	}
	result := AnalyzeResult{
		Title:   checkName,
		IconKey: "kubernetes_text_analyze",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
	}

	reMatch := re.MatchString(string(collected))
	failWhen := false
	if failOutcome != nil && failOutcome.When != "" {
		failWhen, err = strconv.ParseBool(failOutcome.When)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to process when statement: %s", failOutcome.When)
		}
	}
	passWhen := true
	if passOutcome != nil && passOutcome.When != "" {
		passWhen, err = strconv.ParseBool(passOutcome.When)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to process when statement: %s", passOutcome.When)
		}
	}

	if passWhen == failWhen {
		return nil, errors.Wrap(err, "outcome when conditions for fail and pass are equal")
	}

	if reMatch == passWhen {
		result.IsPass = true
		if passOutcome != nil {
			result.Message = passOutcome.Message
			result.URI = passOutcome.URI
		}
		return &result, nil
	}

	result.IsFail = true
	if failOutcome != nil {
		result.Message = failOutcome.Message
		result.URI = failOutcome.URI
	}
	return &result, nil
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
			isMatch, err := compareRegex(outcome.Fail.When, foundMatches)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare regex fail conditional")
			}

			if isMatch {
				result.IsFail = true
				tplMessage, err := templateRegExGroup(outcome.Fail.Message, foundMatches)
				if err != nil {
					return result, errors.Wrap(err, "failed to template message in outcome.Fail block")
				}
				result.Message = tplMessage
				result.URI = outcome.Fail.URI

				return result, nil
			}
		} else if outcome.Warn != nil {
			isMatch, err := compareRegex(outcome.Warn.When, foundMatches)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare regex warn conditional")
			}

			if isMatch {
				result.IsWarn = true
				tplMessage, err := templateRegExGroup(outcome.Warn.Message, foundMatches)
				if err != nil {
					return result, errors.Wrap(err, "failed to template message in outcome.Warn block")
				}
				result.Message = tplMessage
				result.URI = outcome.Warn.URI

				return result, nil
			}
		} else if outcome.Pass != nil {
			isMatch, err := compareRegex(outcome.Pass.When, foundMatches)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare regex pass conditional")
			}

			if isMatch {
				result.IsPass = true
				tplMessage, err := templateRegExGroup(outcome.Pass.Message, foundMatches)
				if err != nil {
					return result, errors.Wrap(err, "failed to template message in outcome.Pass block")
				}
				result.Message = tplMessage
				result.URI = outcome.Pass.URI

				return result, nil
			}
		}
	}

	return result, nil
}

// templateRegExGroup takes a tpl and replaces the variables using matches.
func templateRegExGroup(tpl string, matches map[string]string) (string, error) {
	t, err := template.New("").Parse(tpl)
	if err != nil {
		return "", err
	}
	var msg bytes.Buffer
	err = t.Execute(&msg, matches)
	if err != nil {
		return "", err
	}
	return msg.String(), nil
}

func compareRegex(conditional string, foundMatches map[string]string) (bool, error) {
	if conditional == "" {
		return true, nil
	}
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
			return false, fmt.Errorf("unexpected operator %q in regex comparator, cannot compare %q and %q", operator, foundValue, lookForValue)
		}

		return foundValue == lookForValue, nil
	}

	return false, nil
}
