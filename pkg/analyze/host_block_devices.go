package analyzer

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeHostBlockDevices struct {
	hostAnalyzer *troubleshootv1beta2.BlockDevicesAnalyze
}

func (a *AnalyzeHostBlockDevices) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Block Devices")
}

func (a *AnalyzeHostBlockDevices) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostBlockDevices) Analyze(getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	hostAnalyzer := a.hostAnalyzer

	contents, err := getCollectedFileContents("system/block_devices.json")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get collected file")
	}

	var devices []collect.BlockDeviceInfo
	if err := json.Unmarshal(contents, &devices); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal block devices info")
	}

	result := AnalyzeResult{}

	result.Title = a.Title()

	for _, outcome := range hostAnalyzer.Outcomes {
		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return &result, nil
			}

			isMatch, err := compareHostBlockDevicesConditionalToActual(outcome.Fail.When, devices)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %s", outcome.Fail.When)
			}

			if isMatch {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return &result, nil
			}
		} else if outcome.Warn != nil {
			if outcome.Warn.When == "" {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return &result, nil
			}

			isMatch, err := compareHostBlockDevicesConditionalToActual(outcome.Warn.When, devices)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %s", outcome.Warn.When)
			}

			if isMatch {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return &result, nil
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return &result, nil
			}

			isMatch, err := compareHostBlockDevicesConditionalToActual(outcome.Pass.When, devices)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %s", outcome.Pass.When)
			}

			if isMatch {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return &result, nil
			}
		}
	}

	return &result, nil
}

// <regexp> <op> <count>
// example: sdb > 0
func compareHostBlockDevicesConditionalToActual(conditional string, devices []collect.BlockDeviceInfo) (res bool, err error) {
	parts := strings.Split(conditional, " ")
	if len(parts) != 3 {
		return false, fmt.Errorf("Expected exactly 3 parts, got %d", len(parts))
	}

	rx, err := regexp.Compile(parts[0])
	if err != nil {
		return false, errors.Wrapf(err, "failed to compile regex %q", parts[0])
	}
	count := countEligibleBlockDevices(rx, devices)

	desiredInt, err := strconv.Atoi(parts[2])
	if err != nil {
		return false, errors.Wrapf(err, "failed to parse desired quantity %q", parts[2])
	}

	switch parts[1] {
	case ">":
		return count > desiredInt, nil
	case ">=":
		return count >= desiredInt, nil
	case "<":
		return count < desiredInt, nil
	case "<=":
		return count <= desiredInt, nil
	case "=", "==", "===":
		return count == desiredInt, nil
	}

	return false, fmt.Errorf("Unexpected operator %q", parts[1])
}

func countEligibleBlockDevices(rx *regexp.Regexp, devices []collect.BlockDeviceInfo) int {
	count := 0

	for _, device := range devices {
		if isEligibleBlockDevice(rx, device, devices) {
			count++
		}
	}

	return count
}

func isEligibleBlockDevice(rx *regexp.Regexp, device collect.BlockDeviceInfo, devices []collect.BlockDeviceInfo) bool {
	if !rx.MatchString(device.Name) {
		return false
	}

	if device.Type != "disk" {
		return false
	}

	if device.Mountpoint != "" {
		return false
	}

	if device.FilesystemType != "" {
		return false
	}

	if device.ReadOnly {
		return false
	}

	if device.Removable {
		return false
	}

	for _, d := range devices {
		if d.ParentKernelName == device.KernelName {
			return false
		}
	}

	return true
}
