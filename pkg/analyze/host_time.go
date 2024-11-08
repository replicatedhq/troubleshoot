package analyzer

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

const (
	SynchronizedActive     = "synchronized+active"
	SynchronizedInactive   = "synchronized+inactive"
	UnsynchronizedActive   = "unsynchronized+active"
	UnsynchronizedInactive = "unsynchronized+inactive"
)

type AnalyzeHostTime struct {
	hostAnalyzer *troubleshootv1beta2.TimeAnalyze
}

func (a *AnalyzeHostTime) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Time")
}

func (a *AnalyzeHostTime) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostTime) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	result := AnalyzeResult{Title: a.Title()}

	collectedContents, err := retrieveCollectedContents(
		getCollectedFileContents,
		collect.HostTimePath,
		collect.NodeInfoBaseDir,
		collect.HostTimeFileName,
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

func compareHostTimeStatusToActual(status string, timeInfo collect.TimeInfo) (res bool, err error) {
	parts := strings.Split(status, " ")

	if len(parts) != 3 {
		return false, fmt.Errorf("Expected exactly 3 parts, got %d", len(parts))
	}
	if parts[0] == "timezone" {
		if parts[1] != "=" && parts[1] != "==" && parts[1] != "===" && parts[1] != "!=" {
			return false, errors.New(`Only supported operators are "==" and "!="`)
		}
		if parts[1] == "!=" {
			return parts[2] != timeInfo.Timezone, nil
		}
		return parts[2] == timeInfo.Timezone, nil
	}

	if parts[0] == "ntp" {
		if parts[1] != "=" && parts[1] != "==" && parts[1] != "===" {
			return false, errors.New(`Only supported operator is "=="`)
		}

		switch parts[2] {
		case SynchronizedActive:
			return timeInfo.NTPSynchronized && timeInfo.NTPActive, nil
		case SynchronizedInactive:
			return timeInfo.NTPSynchronized && !timeInfo.NTPActive, nil
		case UnsynchronizedActive:
			return !timeInfo.NTPSynchronized && timeInfo.NTPActive, nil
		case UnsynchronizedInactive:
			return !timeInfo.NTPSynchronized && !timeInfo.NTPActive, nil
		default:
			return false, fmt.Errorf("Unknown status %q. Allowed values are %q, %q, %q, or %q", parts[2], SynchronizedActive, SynchronizedInactive, UnsynchronizedActive, UnsynchronizedInactive)
		}
	}

	return false, fmt.Errorf("Unknown keyword: %s", parts[0])
}

func (a *AnalyzeHostTime) CheckCondition(when string, data []byte) (bool, error) {

	var timeInfo collect.TimeInfo
	if err := json.Unmarshal(data, &timeInfo); err != nil {
		return false, fmt.Errorf("failed to unmarshal data into TimeInfo: %v", err)
	}

	return compareHostTimeStatusToActual(when, timeInfo)
}
