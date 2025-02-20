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
	result := AnalyzeResult{Title: a.Title()}

	// Use the generic function to collect both local and remote data
	collectedContents, err := retrieveCollectedContents(
		getCollectedFileContents,
		collect.HostOSInfoPath,   // Local path
		collect.NodeInfoBaseDir,  // Remote base directory
		collect.HostInfoFileName, // Remote file name
	)
	if err != nil {
		return []*AnalyzeResult{&result}, err
	}

	results, err := analyzeHostCollectorResults(collectedContents, a.hostAnalyzer.Outcomes, a.CheckCondition, a.Title())
	if err != nil {
		return nil, errors.Wrap(err, "failed to analyze OS version")
	}

	return results, nil
}

// checkCondition checks the condition of the when clause
func (a *AnalyzeHostOS) CheckCondition(when string, data []byte) (bool, error) {

	var osInfo collect.HostOSInfo
	if err := json.Unmarshal(data, &osInfo); err != nil {
		return false, fmt.Errorf("failed to unmarshal data into HostOSInfo: %v", err)
	}

	parts := strings.Split(when, " ")
	if len(parts) < 3 {
		return false, errors.New("when condition must have at least 3 parts")
	}

	// handle things like "ubuntu == 20.04", but also "ubuntu == 20.04 || < 20.04" or "ubuntu == 20.04 || < 20.04 || >= 20.04" etc
	// the number of parts should be a multiple of 3
	if len(parts)%3 != 0 {
		return false, errors.New("when condition must have a multiple of 3 parts, such as 'ubuntu == 20.04' or 'rhel >= 8 && < 9'")
	}

	stringToParse := ""
	expectedVer := fixVersion(parts[2])
	toleratedVer, err := semver.ParseTolerant(expectedVer)
	if err != nil {
		return false, errors.Wrapf(err, "failed to parse version: %s", expectedVer)
	}
	stringToParse = fmt.Sprintf("%s %s", parts[1], toleratedVer.String())

	trimmedParts := strings.Split(when, " ")
	for len(trimmedParts) > 3 {
		trimmedParts = trimmedParts[3:]

		expectedVer = fixVersion(trimmedParts[2])
		toleratedVer, err = semver.ParseTolerant(expectedVer)
		if err != nil {
			return false, errors.Wrapf(err, "failed to parse version: %s", expectedVer)
		}

		// first part is either "||" or "&&"
		// if it's "&&", it is assumed by the semver package and should not be included
		// second part is the conditional
		// third part is the version
		if trimmedParts[0] == "||" {
			stringToParse = fmt.Sprintf("%s %s %s %s", stringToParse, trimmedParts[0], trimmedParts[1], toleratedVer.String())
		} else if trimmedParts[0] == "&&" {
			stringToParse = fmt.Sprintf("%s %s %s", stringToParse, trimmedParts[1], toleratedVer.String())
		} else {
			return false, errors.Errorf("invalid conditional, expected either && or ||, got %q", trimmedParts[0])
		}
	}

	whenRange, err := semver.ParseRange(stringToParse)
	if err != nil {
		return false, errors.Wrapf(err, "failed to parse version range: %s", when)
	}

	// Match the kernel version regardless of the platform
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

	// Match platform version and kernel version, such as "centos-8.2-kernel == 8.2"
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
	} else if platform == osInfo.Platform || platform == osInfo.PlatformFamily {
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
