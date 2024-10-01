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
	"github.com/replicatedhq/troubleshoot/pkg/constants"
)

type AnalyzeHostOS struct {
	hostAnalyzer *troubleshootv1beta2.HostOSAnalyze
}

type NodeOSInfo struct {
	NodeName string
	collect.HostOSInfo
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
	var nodesOSInfo []NodeOSInfo
	result := AnalyzeResult{}
	result.Title = a.Title()

	// check if the host os info file exists (local mode)
	contents, err := getCollectedFileContents(collect.HostOSInfoPath)
	if err != nil {
		// check if the node list file exists (remote mode)
		contents, err := getCollectedFileContents(constants.NODE_LIST_FILE)
		if err != nil {
			return []*AnalyzeResult{&result}, errors.Wrap(err, "failed to get collected file")
		}

		var nodes collect.HostOSInfoNodes
		if err := json.Unmarshal(contents, &nodes); err != nil {
			return []*AnalyzeResult{&result}, errors.Wrap(err, "failed to unmarshal host os info nodes")
		}

		// iterate over each node and analyze the host os info
		for _, node := range nodes.Nodes {
			contents, err := getCollectedFileContents(collect.NodeInfoBaseDir + "/" + node + "/" + collect.HostInfoFileName)
			if err != nil {
				return []*AnalyzeResult{&result}, errors.Wrap(err, "failed to get collected file")
			}

			var osInfo collect.HostOSInfo
			if err := json.Unmarshal(contents, &osInfo); err != nil {
				return []*AnalyzeResult{&result}, errors.Wrap(err, "failed to unmarshal host os info")
			}

			nodesOSInfo = append(nodesOSInfo, NodeOSInfo{NodeName: node, HostOSInfo: osInfo})
		}

		results, err := analyzeOSVersionResult(nodesOSInfo, a.hostAnalyzer.Outcomes, a.Title())
		if err != nil {
			return []*AnalyzeResult{&result}, errors.Wrap(err, "failed to analyze os version result")
		}
		return results, nil
	}

	var osInfo collect.HostOSInfo
	if err := json.Unmarshal(contents, &osInfo); err != nil {
		return []*AnalyzeResult{&result}, errors.Wrap(err, "failed to unmarshal host os info")
	}
	nodesOSInfo = append(nodesOSInfo, NodeOSInfo{NodeName: "", HostOSInfo: osInfo})
	return analyzeOSVersionResult(nodesOSInfo, a.hostAnalyzer.Outcomes, a.Title())
}

func analyzeOSVersionResult(nodesOSInfo []NodeOSInfo, outcomes []*troubleshootv1beta2.Outcome, title string) ([]*AnalyzeResult, error) {
	var results []*AnalyzeResult
	for _, osInfo := range nodesOSInfo {
		if title == "" {
			title = "Host OS Info"
		}

		analyzeResult, err := analyzeByOutcomes(outcomes, osInfo, title)
		if err != nil {
			return nil, errors.Wrap(err, "failed to analyze condition")
		}
		results = append(results, analyzeResult...)
	}

	return results, nil
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

func analyzeByOutcomes(outcomes []*troubleshootv1beta2.Outcome, osInfo NodeOSInfo, title string) ([]*AnalyzeResult, error) {
	var results []*AnalyzeResult
	for _, outcome := range outcomes {
		if osInfo.NodeName != "" {
			title = fmt.Sprintf("%s - Node %s", title, osInfo.NodeName)
		}

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
			results = append(results, &result)
			return results, nil
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
				results = append(results, &result)
				return results, nil
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
					results = append(results, &result)
					return results, nil
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
				results = append(results, &result)
				return results, nil
			}
		}
	}
	return results, nil
}
