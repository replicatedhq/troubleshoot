package analyzer

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"k8s.io/apimachinery/pkg/api/resource"
)

type AnalyzeHostDiskUsage struct {
	hostAnalyzer *troubleshootv1beta2.DiskUsageAnalyze
}

func (a *AnalyzeHostDiskUsage) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, fmt.Sprintf("Disk Usage %s", a.hostAnalyzer.CollectorName))
}

func (a *AnalyzeHostDiskUsage) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostDiskUsage) Analyze(getCollectedFileContents func(string) ([]byte, error)) ([]*AnalyzeResult, error) {
	hostAnalyzer := a.hostAnalyzer

	key := collect.HostDiskUsageKey(hostAnalyzer.CollectorName)
	contents, err := getCollectedFileContents(key)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get collected file %s", key)
	}

	diskUsageInfo := collect.DiskUsageInfo{}
	if err := json.Unmarshal(contents, &diskUsageInfo); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal disk usage info from %s", key)
	}

	var coll resultCollector

	for _, outcome := range hostAnalyzer.Outcomes {
		result := &AnalyzeResult{Title: a.Title()}

		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				coll.push(result)
			}

			isMatch, err := compareHostDiskUsageConditionalToActual(outcome.Fail.When, diskUsageInfo.TotalBytes, diskUsageInfo.UsedBytes)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %q", outcome.Fail.When)
			}

			if isMatch {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				coll.push(result)
			}
		} else if outcome.Warn != nil {
			if outcome.Warn.When == "" {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				coll.push(result)
			}

			isMatch, err := compareHostDiskUsageConditionalToActual(outcome.Warn.When, diskUsageInfo.TotalBytes, diskUsageInfo.UsedBytes)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %q", outcome.Warn.When)
			}

			if isMatch {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				coll.push(result)
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				coll.push(result)
			}

			isMatch, err := compareHostDiskUsageConditionalToActual(outcome.Pass.When, diskUsageInfo.TotalBytes, diskUsageInfo.UsedBytes)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %q", outcome.Pass.When)
			}

			if isMatch {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				coll.push(result)
			}
		}
	}

	return coll.get(a.Title()), nil
}

func compareHostDiskUsageConditionalToActual(conditional string, totalBytes uint64, usedBytes uint64) (res bool, err error) {
	parts := strings.Split(conditional, " ")
	if len(parts) != 3 {
		return false, fmt.Errorf("conditional must have exactly 3 parts, got %d", len(parts))
	}
	stat := strings.ToLower(parts[0])
	comparator := parts[1]
	desired := parts[2]

	switch stat {
	case "total":
		return doCompareHostDiskUsage(comparator, desired, totalBytes)
	case "used":
		return doCompareHostDiskUsage(comparator, desired, usedBytes)
	case "available":
		return doCompareHostDiskUsage(comparator, desired, totalBytes-usedBytes)
	case "used/total":
		used := float64(usedBytes) / float64(totalBytes)
		return doCompareHostDiskUsagePercent(comparator, desired, used)
	case "available/total":
		available := float64(totalBytes-usedBytes) / float64(totalBytes)
		return doCompareHostDiskUsagePercent(comparator, desired, available)
	}
	return false, fmt.Errorf("unknown disk usage statistic %q", stat)
}

func doCompareHostDiskUsage(operator string, desired string, actual uint64) (bool, error) {
	quantity, err := resource.ParseQuantity(desired)
	if err != nil {
		return false, fmt.Errorf("could not parse quantity %q", desired)
	}
	desiredInt, ok := quantity.AsInt64()
	if !ok {
		return false, fmt.Errorf("could not parse quantity %q", desired)
	}

	switch operator {
	case "<":
		return actual < uint64(desiredInt), nil
	case "<=":
		return actual <= uint64(desiredInt), nil
	case ">":
		return actual > uint64(desiredInt), nil
	case ">=":
		return actual >= uint64(desiredInt), nil
	case "=", "==", "===":
		return actual == uint64(desiredInt), nil
	}

	return false, errors.New("unknown operator")
}

func doCompareHostDiskUsagePercent(operator string, desired string, actual float64) (bool, error) {
	isPercent := false
	if strings.HasSuffix(desired, "%") {
		desired = strings.TrimSuffix(desired, "%")
		isPercent = true
	}
	desiredPercent, err := strconv.ParseFloat(desired, 64)
	if err != nil {
		return false, errors.Wrap(err, "parsed desired quantity")
	}
	if isPercent {
		desiredPercent = desiredPercent / 100.0
	}

	switch operator {
	case "<":
		return actual < desiredPercent, nil
	case "<=":
		return actual <= desiredPercent, nil
	case ">":
		return actual > desiredPercent, nil
	case ">=":
		return actual >= desiredPercent, nil
	case "=", "==", "===":
		return actual == desiredPercent, nil
	}

	return false, errors.New("unknown operator")
}
