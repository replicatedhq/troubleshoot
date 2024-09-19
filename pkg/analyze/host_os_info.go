package analyzer

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/blang/semver/v4"
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

func (a *AnalyzeHostOS) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	var results []*AnalyzeResult
	result := AnalyzeResult{}
	result.Title = a.Title()

	// check if the host os info file exists (local mode)
	contents, err := getCollectedFileContents(collect.HostOSInfoPath)
	if err != nil {
		//check if the host os info nodes file exists (remote mode)
		contents, err := getCollectedFileContents(collect.HostOSNodes)
		if err != nil {
			return []*AnalyzeResult{&result}, errors.Wrap(err, "failed to get collected file")
		} else {
			var nodes collect.HostOSInfoNodes
			if err := json.Unmarshal(contents, &nodes); err != nil {
				return []*AnalyzeResult{&result}, errors.Wrap(err, "failed to unmarshal host os info nodes")
			}

			// iterate over each node and analyze the host os info
			for _, node := range nodes.Nodes {
				contents, err := getCollectedFileContents(collect.HostOSInfoDir + "/" + node + "/" + collect.HostOSInfoJSON)
				if err != nil {
					return []*AnalyzeResult{&result}, errors.Wrap(err, "failed to get collected file")
				}

				var osInfo collect.HostOSInfo
				if err := json.Unmarshal(contents, &osInfo); err != nil {
					return []*AnalyzeResult{&result}, errors.Wrap(err, "failed to unmarshal host os info")
				}
				analyzeOSVersionResult, err := analyzeOSVersionResult(osInfo, a.hostAnalyzer.Outcomes, a.Title()+" of Node "+node)
				if err != nil {
					return []*AnalyzeResult{&result}, errors.Wrap(err, "failed to analyze os version result")
				}
				// append the results of each node to the results
				results = append(results, analyzeOSVersionResult...)
			}

			return results, nil
		}
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

	for _, outcome := range outcomes {

		result := AnalyzeResult{
			Title: title,
		}
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
		if len(parts) < 3 {
			return []*AnalyzeResult{&result}, errors.New("when condition must have at least 3 parts")
		}
		expectedVer := fixVersion(parts[2])
		toleratedVer, err := semver.ParseTolerant(expectedVer)
		if err != nil {
			return []*AnalyzeResult{&result}, errors.Wrapf(err, "failed to parse version: %s", expectedVer)
		}
		when = fmt.Sprintf("%s %v", parts[1], toleratedVer)
		whenRange, err := semver.ParseRange(when)
		if err != nil {
			return []*AnalyzeResult{&result}, errors.Wrapf(err, "failed to parse version range: %s", when)
		}

		// Match the kernel version regardless of the platform
		// e.g "kernelVersion == 4.15"
		if parts[0] == "kernelVersion" {
			fixedKernelVer := fixVersion(osInfo.KernelVersion)
			toleratedKernelVer, err := semver.ParseTolerant(fixedKernelVer)
			if err != nil {
				return []*AnalyzeResult{}, errors.Wrapf(err, "failed to parse tolerant: %v", fixedKernelVer)
			}
			if whenRange(toleratedKernelVer) {
				return []*AnalyzeResult{&result}, nil
			}
		}

		// Match the platform version and and kernel version passed in as
		// "<platform>-<kernelVersion>-kernel" e.g "centos-8.2-kernel == 8.2"
		platform := parts[0]
		kernelInfo := fmt.Sprintf("%s-%s-kernel", osInfo.Platform, osInfo.PlatformVersion)
		if len(strings.Split(platform, "-")) == 3 && strings.Split(platform, "-")[2] == "kernel" {
			if platform == kernelInfo {
				fixedKernelVer := fixVersion(osInfo.KernelVersion)
				toleratedKernelVer, err := semver.ParseTolerant(fixedKernelVer)
				if err != nil {
					return []*AnalyzeResult{&result}, errors.Wrapf(err, "failed to parse tolerant: %v", fixedKernelVer)
				}
				if whenRange(toleratedKernelVer) {
					return []*AnalyzeResult{&result}, nil
				}
			}
			// Match the platform version
			// e.g "centos == 8.2"
		} else if platform == osInfo.Platform {
			fixedDistVer := fixVersion(osInfo.PlatformVersion)
			toleratedDistVer, err := semver.ParseTolerant(fixedDistVer)
			if err != nil {
				return []*AnalyzeResult{&result}, errors.Wrapf(err, "failed to parse tolerant: %v", fixedDistVer)
			}
			if whenRange(toleratedDistVer) {
				return []*AnalyzeResult{&result}, nil
			}
		}
	}

	return []*AnalyzeResult{}, nil
}

var rx = regexp.MustCompile(`^[0-9]+\.?[0-9]*\.?[0-9]*`)

func fixVersion(versionStr string) string {

	splitStr := strings.Split(versionStr, ".")
	for i := 0; i < len(splitStr); i++ {
		if splitStr[i] != "0" {
			splitStr[i] = strings.TrimPrefix(splitStr[i], "0")
		}
	}
	fixTrailZero := strings.Join(splitStr, ".")
	version := rx.FindString(fixTrailZero)
	version = strings.TrimRight(version, ".")
	return version
}
