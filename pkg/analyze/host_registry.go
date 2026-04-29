package analyzer

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/internal/util"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"k8s.io/klog/v2"
)

// RegistryImagesSummary is passed as template data when rendering outcome messages.
// Fields are exported so Go templates can reference them.
//
//   - Verified: images confirmed to exist in the registry.
//   - Missing: images confirmed not to exist in the registry.
//   - Errors: images that could not be checked (parse failures, timeouts, auth errors, etc).
//   - UnverifiedReasons: map of image name to reason string for every unverified image
//     (union of Missing and Errors).
//
// The `when` conditions follow the existing registry images analyzer nomenclature:
// "verified", "missing", and "errors" (see https://troubleshoot.sh/docs/analyze/registry-images).
type RegistryImagesSummary struct {
	Verified          []string
	Missing           []string
	Errors            []string
	UnverifiedReasons map[string]string
}

type AnalyzeHostRegistryImages struct {
	hostAnalyzer *troubleshootv1beta2.HostRegistryImagesAnalyze
}

func (a *AnalyzeHostRegistryImages) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Registry Images")
}

func (a *AnalyzeHostRegistryImages) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostRegistryImages) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	collectorName := a.hostAnalyzer.CollectorName
	if collectorName == "" {
		collectorName = "images"
	}

	const nodeBaseDir = "host-collectors/registry-images"
	localPath := fmt.Sprintf("%s/%s.json", nodeBaseDir, collectorName)
	fileName := fmt.Sprintf("%s.json", collectorName)

	collectedContents, err := retrieveCollectedContents(
		getCollectedFileContents,
		localPath,
		nodeBaseDir,
		fileName,
	)
	if err != nil {
		return []*AnalyzeResult{{Title: a.Title()}}, err
	}

	var results []*AnalyzeResult
	for _, content := range collectedContents {
		currentTitle := a.Title()
		if content.NodeName != "" {
			currentTitle = fmt.Sprintf("%s - Node %s", a.Title(), content.NodeName)
		}

		result, err := a.evaluateOutcomesWithTemplate(content.Data, currentTitle)
		if err != nil {
			return nil, errors.Wrap(err, "failed to analyze host registry images")
		}
		if result != nil {
			klog.V(2).Infof("registry images analysis result: title=%q pass=%t warn=%t fail=%t message=%q",
				result.Title, result.IsPass, result.IsWarn, result.IsFail, result.Message)
			results = append(results, result)
		}
	}

	return results, nil
}

func (a *AnalyzeHostRegistryImages) evaluateOutcomesWithTemplate(data []byte, title string) (*AnalyzeResult, error) {
	summary, err := buildRegistryImagesSummary(data)
	if err != nil {
		return nil, err
	}

	for _, outcome := range a.hostAnalyzer.Outcomes {
		result := &AnalyzeResult{Title: title}

		switch {
		case outcome.Fail != nil:
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = renderRegistryMessage(outcome.Fail.Message, summary)
				result.URI = outcome.Fail.URI
				return result, nil
			}
			isMatch, err := compareRegistryConditionalToActual(outcome.Fail.When, len(summary.Verified), len(summary.Missing), len(summary.Errors))
			if err != nil {
				return result, errors.Wrapf(err, "failed to compare %s", outcome.Fail.When)
			}
			if isMatch {
				result.IsFail = true
				result.Message = renderRegistryMessage(outcome.Fail.Message, summary)
				result.URI = outcome.Fail.URI
				return result, nil
			}

		case outcome.Warn != nil:
			if outcome.Warn.When == "" {
				result.IsWarn = true
				result.Message = renderRegistryMessage(outcome.Warn.Message, summary)
				result.URI = outcome.Warn.URI
				return result, nil
			}
			isMatch, err := compareRegistryConditionalToActual(outcome.Warn.When, len(summary.Verified), len(summary.Missing), len(summary.Errors))
			if err != nil {
				return result, errors.Wrapf(err, "failed to compare %s", outcome.Warn.When)
			}
			if isMatch {
				result.IsWarn = true
				result.Message = renderRegistryMessage(outcome.Warn.Message, summary)
				result.URI = outcome.Warn.URI
				return result, nil
			}

		case outcome.Pass != nil:
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = renderRegistryMessage(outcome.Pass.Message, summary)
				result.URI = outcome.Pass.URI
				return result, nil
			}
			isMatch, err := compareRegistryConditionalToActual(outcome.Pass.When, len(summary.Verified), len(summary.Missing), len(summary.Errors))
			if err != nil {
				return result, errors.Wrapf(err, "failed to compare %s", outcome.Pass.When)
			}
			if isMatch {
				result.IsPass = true
				result.Message = renderRegistryMessage(outcome.Pass.Message, summary)
				result.URI = outcome.Pass.URI
				return result, nil
			}
		}
	}

	return nil, nil
}

func buildRegistryImagesSummary(data []byte) (*RegistryImagesSummary, error) {
	var registryInfo collect.RegistryInfo
	if err := json.Unmarshal(data, &registryInfo); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal registry info")
	}

	summary := &RegistryImagesSummary{
		UnverifiedReasons: map[string]string{},
	}
	for image, info := range registryInfo.Images {
		if info.Error != "" {
			summary.Errors = append(summary.Errors, image)
			summary.UnverifiedReasons[image] = info.Error
		} else if !info.Exists {
			summary.Missing = append(summary.Missing, image)
			summary.UnverifiedReasons[image] = "image not found in registry"
		} else {
			summary.Verified = append(summary.Verified, image)
		}
	}
	slices.Sort(summary.Verified)
	slices.Sort(summary.Missing)
	slices.Sort(summary.Errors)
	return summary, nil
}

func renderRegistryMessage(message string, summary *RegistryImagesSummary) string {
	rendered, err := util.RenderTemplate(message, summary)
	if err != nil {
		klog.V(2).Infof("Failed to render registry message template: %v", err)
		return message
	}
	return rendered
}

func (a *AnalyzeHostRegistryImages) CheckCondition(when string, data []byte) (bool, error) {
	summary, err := buildRegistryImagesSummary(data)
	if err != nil {
		return false, err
	}

	return compareRegistryConditionalToActual(when, len(summary.Verified), len(summary.Missing), len(summary.Errors))
}
