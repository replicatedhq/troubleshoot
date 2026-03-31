package analyzer

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeHostBlockDevices struct {
	hostAnalyzer *troubleshootv1beta2.BlockDevicesAnalyze
}

// blockDevicesMatchConfig carries analyzer settings for counting devices toward a blockDevices when-clause
// ("<name-regex> <op> <count>" against host block_devices.json). A device is eligible when its name matches
// the regex; its type is "disk", or "part" if includeUnmountedPartitions, or listed in additionalDeviceTypes;
// size >= minimumAcceptableSize when that is non-zero; it has no mountpoint or filesystem; it is not
// read-only or removable; and no other row has ParentKernelName equal to its KernelName.
type blockDevicesMatchConfig struct {
	minimumAcceptableSize      uint64
	includeUnmountedPartitions bool
	additionalDeviceTypes      []string
}

func matchConfigFromAnalyzer(a *troubleshootv1beta2.BlockDevicesAnalyze) blockDevicesMatchConfig {
	return blockDevicesMatchConfig{
		minimumAcceptableSize:      a.MinimumAcceptableSize,
		includeUnmountedPartitions: a.IncludeUnmountedPartitions,
		additionalDeviceTypes:      a.AdditionalDeviceTypes,
	}
}

func (a *AnalyzeHostBlockDevices) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Block Devices")
}

func (a *AnalyzeHostBlockDevices) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostBlockDevices) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	result := AnalyzeResult{Title: a.Title()}

	collectedContents, err := retrieveCollectedContents(
		getCollectedFileContents,
		collect.HostBlockDevicesPath,
		collect.NodeInfoBaseDir,
		collect.HostBlockDevicesFileName,
	)
	if err != nil {
		return []*AnalyzeResult{&result}, err
	}

	results, err := analyzeHostCollectorResults(collectedContents, a.hostAnalyzer.Outcomes, a.CheckCondition, a.Title())
	if err != nil {
		return nil, errors.Wrap(err, "failed to analyze block devices")
	}

	return results, nil
}

// <regexp> <op> <count>
// example: sdb > 0
func compareHostBlockDevicesConditionalToActual(conditional string, cfg blockDevicesMatchConfig, devices []collect.BlockDeviceInfo) (res bool, err error) {
	parts := strings.Split(conditional, " ")
	if len(parts) != 3 {
		return false, fmt.Errorf("Expected exactly 3 parts, got %d", len(parts))
	}

	rx, err := regexp.Compile(parts[0])
	if err != nil {
		return false, errors.Wrapf(err, "failed to compile regex %q", parts[0])
	}
	count := countEligibleBlockDevices(rx, cfg, devices)

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

func countEligibleBlockDevices(rx *regexp.Regexp, cfg blockDevicesMatchConfig, devices []collect.BlockDeviceInfo) int {
	count := 0

	for _, device := range devices {
		if isEligibleBlockDevice(rx, cfg, device, devices) {
			count++
		}
	}

	return count
}

func isEligibleDeviceType(deviceType string, cfg blockDevicesMatchConfig) bool {
	if deviceType == "disk" {
		return true
	}
	if cfg.includeUnmountedPartitions && deviceType == "part" {
		return true
	}
	return slices.Contains(cfg.additionalDeviceTypes, deviceType)
}

func isEligibleBlockDevice(rx *regexp.Regexp, cfg blockDevicesMatchConfig, device collect.BlockDeviceInfo, devices []collect.BlockDeviceInfo) bool {
	if !rx.MatchString(device.Name) {
		return false
	}

	if !isEligibleDeviceType(device.Type, cfg) {
		return false
	}

	if cfg.minimumAcceptableSize != 0 {
		if device.Size < cfg.minimumAcceptableSize {
			return false
		}
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

func (a *AnalyzeHostBlockDevices) CheckCondition(when string, data []byte) (bool, error) {
	var devices []collect.BlockDeviceInfo
	if err := json.Unmarshal(data, &devices); err != nil {
		return false, errors.Wrap(err, "failed to unmarshal block devices info")
	}

	return compareHostBlockDevicesConditionalToActual(when, matchConfigFromAnalyzer(a.hostAnalyzer), devices)
}
