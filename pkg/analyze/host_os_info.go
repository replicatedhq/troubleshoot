package analyzer

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/go-version"
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
	hostAnalyzer := a.hostAnalyzer

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

	for _, outcome := range hostAnalyzer.Outcomes {
		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return []*AnalyzeResult{&result}, nil
			}
			_, err := compareHostOSConditionalToActual(outcome.Fail.When, osInfo)
			if err != nil {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI
				return []*AnalyzeResult{&result}, errors.Wrap(err, "unsupported distribution")
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return []*AnalyzeResult{&result}, nil
			}

			isMatch, err := compareHostOSConditionalToActual(outcome.Pass.When, osInfo)
			if err != nil {
				return []*AnalyzeResult{&result}, errors.Wrap(err, "failed to compare host os with actual")
			}
			if isMatch {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return []*AnalyzeResult{&result}, nil
			}
		}
	}

	return []*AnalyzeResult{&result}, nil
}

/*
- pass
    when: "ubuntu == 16.04"
- pass
    when: "ubuntu == 18.04"
*/
func compareHostOSConditionalToActual(conditional string, hostInfo collect.HostOSInfo) (bool, error) {
	parts := strings.Split(conditional, " ")
	if len(parts) != 3 {
		return false, fmt.Errorf("Expected exactly 3 parts, got %d", len(parts))
	}
	// only for ubuntu 16.04, check if the kernel version is < 4.15
	//https://kurl.sh/docs/install-with-kurl/system-requirements
	if parts[0] == "ubuntu-16.04-kernel" {
		return bailIfUnsupportedKernel(parts[1], parts[2], hostInfo.KernelVersion)
	}
	_, err := bailIfUnsupportedOS(parts[0], parts[1], parts[2], hostInfo.ReleaseVersion)
	if err != nil {
		return false, errors.Wrap(err, "unsupported distribution")
	}

	return true, nil
}

/*
- pass
    when: "ubuntu == 16.04"
- pass
    when: "ubuntu == 18.04"
- fail
    when: "ubuntu-16.04-kernel < 4.15"
*/

func bailIfUnsupportedOS(platform string, operator string, expectedVersion string, actualVersion string) (bool, error) {
	//https://github.com/replicatedhq/kURL/blob/caea37f6d3895b34cc6b1ab29772827c601f447d/scripts/common/preflights.sh#L45-L61
	supportedDistributionMap := map[string][]string{
		"ubuntu": {"16.04", "18.04", "20.04"},
		"rhel":   {"7.4", "7.5", "7.6", "7.7", "7.8", "7.9", "8.0", "8.1", "8.2", "8.3", "8.4"},
		"centos": {"7.4", "7.5", "7.6", "7.7", "7.8", "7.9", "8.0", "8.1", "8.2", "8.3", "8.4"},
		"ol":     {"7.4", "7.5", "7.6", "7.7", "7.8", "7.9", "8.0", "8.1", "8.2", "8.3", "8.4"},
		"amzn2":  {},
		"amzn":   {},
	}
	if platform == "amzn" || platform == "amzn2" {
		if _, ok := supportedDistributionMap[platform]; ok {
			return true, nil
		}
		return false, fmt.Errorf("Unexpected platform %q", platform)
	}
	d, err := version.NewVersion(expectedVersion)
	if err != nil {
		return false, errors.Wrap(err, "failed to create expected version")
	}
	a, err := version.NewVersion(actualVersion)
	if err != nil {
		return false, errors.Wrap(err, "failed to create actual version")
	}
	// first check if the actual matches with expected
	if !d.Equal(a) {
		return false, fmt.Errorf("actual dist %s does not match with expected %s", a, d)
	}
	// then check if the expected match with supported
	if supportedVersions, ok := supportedDistributionMap[platform]; ok {
		for _, s := range supportedVersions {
			s, err := version.NewVersion(s)
			if err != nil {
				return false, errors.Wrap(err, "failed to create supported version")
			}
			switch operator {
			case "=", "==", "===":
				if d.Equal(s) {
					return true, nil
				}
			}
		}
	}

	return false, fmt.Errorf("expected distribution does not match the supported %q", expectedVersion)
}

/*
- fail
    when: "ubuntu-16.04-kernel < 4.15"
*/
func bailIfUnsupportedKernel(operator string, expectedVersion string, actualVersion string) (bool, error) {
	if operator != "<" {
		return false, fmt.Errorf("Unsupported operator %q", operator)
	}
	supportedKernelVersion, err := version.NewVersion("4.15")
	if err != nil {
		return false, errors.Wrap(err, "failed to create supported kernel version")
	}

	actualKernelVersionParts := strings.Split(actualVersion, ".")
	if len(actualKernelVersionParts) < 2 {
		return false, fmt.Errorf("Incorrect kernel version %s", actualVersion)
	}
	actualKernelVersion := actualKernelVersionParts[0] + "." + actualKernelVersionParts[1]
	a, err := version.NewVersion(actualKernelVersion)
	if err != nil {
		return false, errors.Wrap(err, "failed to create actual kernel version")
	}

	if a.LessThan(supportedKernelVersion) {
		return false, fmt.Errorf("%s is less than 4.15", actualKernelVersion)
	}

	expectedKernelVersionParts := strings.Split(expectedVersion, ".")
	if len(expectedKernelVersionParts) < 2 {
		return false, fmt.Errorf("Incorrect expected kernel version %s", expectedVersion)
	}
	expectedKernelVersion := expectedKernelVersionParts[0] + "." + expectedKernelVersionParts[1]

	d, err := version.NewVersion(expectedKernelVersion)
	if err != nil {
		return false, errors.Wrap(err, "failed to create expected kernel version")
	}
	if d.LessThanOrEqual(supportedKernelVersion) {
		return false, errors.Wrap(err, "expected kernel version is less than supported kernel version")
	}

	return true, nil
}
