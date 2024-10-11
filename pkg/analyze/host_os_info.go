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

type NodesInfo struct {
	Nodes []string `json:"nodes"`
}

type RemoteCollectContent struct {
	NodeName string
	Content  interface{}
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
	var remoteCollectContents []RemoteCollectContent
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

		var nodeNames NodesInfo
		err = json.Unmarshal(contents, &nodeNames)
		if err != nil {
			return []*AnalyzeResult{&result}, errors.Wrap(err, "failed to unmarshal host os info nodes")
		}
		for _, node := range nodeNames.Nodes {
			contents, err := getCollectedFileContents(collect.NodeInfoBaseDir + "/" + node + "/" + collect.HostInfoFileName)
			if err != nil {
				return []*AnalyzeResult{&result}, errors.Wrap(err, "failed to get collected file")
			}

			var osInfo collect.HostOSInfo
			if err := json.Unmarshal(contents, &osInfo); err != nil {
				return []*AnalyzeResult{&result}, errors.Wrap(err, "failed to unmarshal host os info")
			}

			remoteCollectContent := RemoteCollectContent{
				NodeName: node,
				Content:  osInfo,
			}

			remoteCollectContents = append(remoteCollectContents, remoteCollectContent)
		}

		results, err := analyzeOSVersionResult(remoteCollectContents, a.hostAnalyzer.Outcomes, a.Title())
		if err != nil {
			return []*AnalyzeResult{&result}, errors.Wrap(err, "failed to analyze os version result")
		}
		return results, nil
	}

	var osInfo collect.HostOSInfo
	if err := json.Unmarshal(contents, &osInfo); err != nil {
		return []*AnalyzeResult{&result}, errors.Wrap(err, "failed to unmarshal host os info")
	}
	remoteCollectContents = append(remoteCollectContents, RemoteCollectContent{NodeName: "", Content: osInfo})
	return analyzeOSVersionResult(remoteCollectContents, a.hostAnalyzer.Outcomes, a.Title())
}

func analyzeOSVersionResult(collectedContent []RemoteCollectContent, outcomes []*troubleshootv1beta2.Outcome, title string) ([]*AnalyzeResult, error) {
	var results []*AnalyzeResult
	for _, osInfo := range collectedContent {

		currentTitle := title
		if osInfo.NodeName != "" {
			currentTitle = fmt.Sprintf("%s - Node %s", title, osInfo.NodeName)
		}

		osInfo, ok := osInfo.Content.(collect.HostOSInfo)
		if !ok {
			return nil, errors.New("a valid host os info was not found")
		}

		checkCondition := func(when string) (bool, error) {
			parts := strings.Split(when, " ")
			if len(parts) < 3 {
				return false, errors.New("when condition must have at least 3 parts")
			}
			expectedVer := fixVersion(parts[2])
			toleratedVer, err := semver.ParseTolerant(expectedVer)
			if err != nil {
				return false, errors.Wrapf(err, "failed to parse version: %s", expectedVer)
			}
			when = fmt.Sprintf("%s %v", parts[1], toleratedVer)
			whenRange, err := semver.ParseRange(when)
			if err != nil {
				return false, errors.Wrapf(err, "failed to parse version range: %s", when)
			}

			// Match the kernel version regardless of the platform
			// e.g "kernelVersion == 4.15"
			if parts[0] == "kernelVersion" {
				fixedKernelVer := fixVersion(osInfo.KernelVersion)
				toleratedKernelVer, err := semver.ParseTolerant(fixedKernelVer)
				if err != nil {
					return false, errors.Wrapf(err, "failed to parse tolerant: %v", fixedKernelVer)
				}
				if whenRange(toleratedKernelVer) {
					return true, nil
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
						return false, errors.Wrapf(err, "failed to parse tolerant: %v", fixedKernelVer)
					}
					if whenRange(toleratedKernelVer) {
						return true, nil
					}
				}
				// Match the platform version
				// e.g "centos == 8.2"
			} else if platform == osInfo.Platform {
				fixedDistVer := fixVersion(osInfo.PlatformVersion)
				toleratedDistVer, err := semver.ParseTolerant(fixedDistVer)
				if err != nil {
					return false, errors.Wrapf(err, "failed to parse tolerant: %v", fixedDistVer)
				}
				if whenRange(toleratedDistVer) {
					return true, nil
				}
			}
			return false, nil
		}

		analyzeResult, err := evaluateOutcomes(outcomes, checkCondition, currentTitle)
		if err != nil {
			return nil, errors.Wrap(err, "failed to evaluate outcomes")
		}

		/*analyzeResult, err := analyzeByOutcomes(outcomes, osInfo, title)
		if err != nil {
			return nil, errors.Wrap(err, "failed to analyze condition")
		}*/
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

func evaluateOutcomes(outcomes []*troubleshootv1beta2.Outcome, checkCondition func(string) (bool, error), title string) ([]*AnalyzeResult, error) {
	var results []*AnalyzeResult

	for _, outcome := range outcomes {
		result := AnalyzeResult{
			Title: title,
		}
		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI
				results = append(results, &result)
				return results, nil
			}

			isMatch, err := checkCondition(outcome.Fail.When)
			if err != nil {
				return []*AnalyzeResult{&result}, errors.Wrapf(err, "failed to compare %s", outcome.Fail.When)
			}

			if isMatch {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI
				results = append(results, &result)
				return results, nil
			}
		} else if outcome.Warn != nil {
			if outcome.Warn.When == "" {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI
				results = append(results, &result)
				return results, nil
			}

			isMatch, err := checkCondition(outcome.Warn.When)
			if err != nil {
				return []*AnalyzeResult{&result}, errors.Wrapf(err, "failed to compare %s", outcome.Warn.When)
			}

			if isMatch {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI
				results = append(results, &result)
				return results, nil
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI
				results = append(results, &result)
				return results, nil
			}

			isMatch, err := checkCondition(outcome.Pass.When)
			if err != nil {
				return []*AnalyzeResult{&result}, errors.Wrapf(err, "failed to compare %s", outcome.Pass.When)
			}

			if isMatch {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI
				results = append(results, &result)
				return results, nil
			}
		}
	}

	return nil, nil
}
