package analyzer

import (
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

func analyzeRegistry(analyzer *troubleshootv1beta2.RegistryImagesAnalyze, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	collectorName := analyzer.CollectorName
	if collectorName == "" {
		collectorName = "images"
	}

	fullPath := path.Join("registry", fmt.Sprintf("%s.json", collectorName))

	collected, err := getCollectedFileContents(fullPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read collected file name: %s", fullPath)
	}

	registryInfo := collect.RegistryInfo{}
	if err := json.Unmarshal(collected, &registryInfo); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal database connection result")
	}

	numMissingImages := 0
	numVerifiedImages := 0
	numErrors := 0
	for _, image := range registryInfo.Images {
		if image.Error != "" {
			numErrors++
		} else if !image.Exists {
			numMissingImages++
		} else {
			numVerifiedImages++
		}
	}

	title := analyzer.CheckName
	if title == "" {
		title = collectorName
	}

	result := &AnalyzeResult{
		Title:   title,
		IconKey: "kubernetes_registry_analyze",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/registry-analyze.svg",
	}

	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}

			isMatch, err := compareRegistryConditionalToActual(outcome.Fail.When, numVerifiedImages, numMissingImages, numErrors)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare registry conditional")
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

			isMatch, err := compareRegistryConditionalToActual(outcome.Warn.When, numVerifiedImages, numMissingImages, numErrors)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare registry conditional")
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

			isMatch, err := compareRegistryConditionalToActual(outcome.Pass.When, numVerifiedImages, numMissingImages, numErrors)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare registry conditional")
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

func compareRegistryConditionalToActual(conditional string, numVerifiedImages int, numMissingImages int, numErrors int) (bool, error) {
	parts := strings.Split(strings.TrimSpace(conditional), " ")

	if len(parts) != 3 {
		return false, errors.Errorf("unable to parse conditional: %s", conditional)
	}

	switch parts[0] {
	case "verified":
		result, err := doCompareRegistryImageCount(parts[1], parts[2], numVerifiedImages)
		if err != nil {
			return false, errors.Wrap(err, "failed to compare number of verified images")
		}

		return result, nil

	case "missing":
		result, err := doCompareRegistryImageCount(parts[1], parts[2], numMissingImages)
		if err != nil {
			return false, errors.Wrap(err, "failed to compare number of missing images")
		}

		return result, nil

	case "errors":
		result, err := doCompareRegistryImageCount(parts[1], parts[2], numErrors)
		if err != nil {
			return false, errors.Wrap(err, "failed to compare number of errors")
		}

		return result, nil
	}

	return false, errors.Errorf("unknown term %q in conditional", parts[0])
}

func doCompareRegistryImageCount(operator string, desired string, actual int) (bool, error) {
	desiredInt, err := strconv.Atoi(desired)
	if err != nil {
		return false, errors.Wrap(err, "failed to parse")
	}

	switch operator {
	case "<":
		return actual < desiredInt, nil
	case "<=":
		return actual <= desiredInt, nil
	case ">":
		return actual > desiredInt, nil
	case ">=":
		return actual >= desiredInt, nil
	case "=", "==", "===":
		return actual == desiredInt, nil
	}

	return false, errors.Errorf("unknown operator: %s", operator)
}
