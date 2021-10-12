package analyzer

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeHostOS struct {
	hostAnalyzer *troubleshootv1beta2.HostOSAnalyze
}

func (a *AnalyzeHostOS) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Host OS Info")
}

func (a *AnalyzeHostOS) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostOS) Analyze(getCollectedFileContents func(string) ([]byte, error)) ([]*AnalyzeResult, error) {

	result := AnalyzeResult{}
	result.Title = a.Title()

	contents, err := getCollectedFileContents("system/hostos_info.json")
	if err != nil {
		return []*AnalyzeResult{&result}, errors.Wrap(err, "failed to get collected file")
	}

	var osInfo collect.HostOSInfo
	if err := json.Unmarshal(contents, &osInfo); err != nil {
		return []*AnalyzeResult{&result}, errors.Wrap(err, "failed to unmarshal host os info")
	}

	return analyzeOSVersionResult(osInfo, a.hostAnalyzer.Outcomes, a.Title())
}

func analyzeOSVersionResult(osInfo collect.HostOSInfo, outcomes []*troubleshootv1beta2.Outcome, title string) ([]*AnalyzeResult, error) {

	if title == "" {
		title = "Host OS Info"
	}
	result := AnalyzeResult{
		Title: title,
	}

	for _, outcome := range outcomes {
		when := ""
		message := ""
		uri := ""

		if outcome.Fail != nil {
			result.IsFail = true
			when = outcome.Fail.When
			message = outcome.Fail.Message
			uri = outcome.Fail.URI
		} else if outcome.Warn != nil {
			result.IsWarn = true
			when = outcome.Warn.When
			message = outcome.Warn.Message
			uri = outcome.Warn.URI
		} else if outcome.Pass != nil {
			result.IsPass = true
			when = outcome.Pass.When
			message = outcome.Pass.Message
			uri = outcome.Pass.URI
		} else {
			return nil, errors.New("empty outcome")
		}

		result.Message = message
		result.URI = uri
		// When is usually empty as the final case and should be treated as true
		if when == "" {
			return []*AnalyzeResult{&result}, nil
		}

		parts := strings.Split(when, " ")
		platform := parts[0]
		expectedVer := fixDistVersion(parts[2])
		toleratedVer, err := semver.ParseTolerant(expectedVer)
		if err != nil {
			return []*AnalyzeResult{&result}, errors.Wrapf(err, "failed to parse tolerant: %s", expectedVer)
		}

		when = fmt.Sprintf("%s %v", parts[1], toleratedVer)
		whenRange, err := semver.ParseRange(when)
		if err != nil {
			return []*AnalyzeResult{&result}, errors.Wrapf(err, "failed to parse range: %s", when)
		}

		kernelInfo := fmt.Sprintf("%s-%s-kernel", osInfo.Platform, osInfo.PlatformVersion)
		if len(strings.Split(platform, "-")) == 3 && strings.Split(platform, "-")[2] == "kernel" {
			if platform == kernelInfo {
				fixedKernelVer := fixKernelVersion(osInfo.KernelVersion)
				toleratedKernelVer, err := semver.ParseTolerant(fixedKernelVer)
				if err != nil {
					return []*AnalyzeResult{&result}, errors.Wrapf(err, "failed to parse tolerant: %v", fixedKernelVer)
				}
				if whenRange(toleratedKernelVer) {
					return []*AnalyzeResult{&result}, nil
				}
			}
		} else if platform == osInfo.Platform {
			fixedDistVer := fixDistVersion(osInfo.PlatformVersion)
			toleratedDistVer, err := semver.ParseTolerant(fixedDistVer)
			if err != nil {
				return []*AnalyzeResult{&result}, errors.Wrapf(err, "failed to parse tolerant: %v", fixedDistVer)
			}
			if whenRange(toleratedDistVer) {
				return []*AnalyzeResult{&result}, nil
			}
		}
	}

	return []*AnalyzeResult{&result}, nil
}

func fixDistVersion(versionStr string) string {
	splitStr := strings.Split(versionStr, ".")
	for i := 0; i < len(splitStr); i++ {
		splitStr[i] = strings.TrimPrefix(splitStr[i], "0")
	}

	return strings.Join(splitStr, ".")
}

func fixKernelVersion(versionStr string) string {
	splitStr := strings.Split(versionStr, ".")
	for i := 0; i < len(splitStr); i++ {
		splitStr[i] = strings.TrimPrefix(splitStr[i], "0")
	}
	if len(splitStr) >= 2 {
		splitStr = splitStr[:2]
		return strings.Join(splitStr, ".")
	}

	return ""
}
