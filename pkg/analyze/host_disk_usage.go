package analyzer

import (
	"encoding/json"
	"fmt"
	"reflect"
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

func (a *AnalyzeHostDiskUsage) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {

	collectorName := a.hostAnalyzer.CollectorName
	if collectorName == "" {
		collectorName = "diskUsage"
	}

	const nodeBaseDir = "host-collectors/diskUsage"
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

	results, err := analyzeHostCollectorResults(collectedContents, a.hostAnalyzer.Outcomes, a.CheckCondition, a.Title())
	if err != nil {
		return nil, errors.Wrap(err, "failed to analyze disk usage")
	}

	return results, nil
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

func (a *AnalyzeHostDiskUsage) CheckCondition(when string, data collectorData) (bool, error) {
	rawData, ok := data.([]byte)
	if !ok {
		return false, fmt.Errorf("expected data to be []uint8 (raw bytes), got: %v", reflect.TypeOf(data))
	}

	var diskUsageInfo collect.DiskUsageInfo
	if err := json.Unmarshal(rawData, &diskUsageInfo); err != nil {
		return false, fmt.Errorf("failed to unmarshal data into DiskUsageInfo: %v", err)
	}

	return compareHostDiskUsageConditionalToActual(when, diskUsageInfo.TotalBytes, diskUsageInfo.UsedBytes)
}
