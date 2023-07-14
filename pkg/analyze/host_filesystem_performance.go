package analyzer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeHostFilesystemPerformance struct {
	hostAnalyzer *troubleshootv1beta2.FilesystemPerformanceAnalyze
}

func (a *AnalyzeHostFilesystemPerformance) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Filesystem Performance")
}

func (a *AnalyzeHostFilesystemPerformance) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostFilesystemPerformance) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	hostAnalyzer := a.hostAnalyzer

	collectorName := hostAnalyzer.CollectorName
	if collectorName == "" {
		collectorName = "filesystemPerformance"
	}
	name := filepath.Join("host-collectors/filesystemPerformance", collectorName+".json")
	contents, err := getCollectedFileContents(name)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get collected file %s", name)
	}

	fsPerf := collect.FSPerfResults{}
	if err := json.Unmarshal(contents, &fsPerf); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal filesystem performance results from %s", name)
	}

	result := &AnalyzeResult{
		Title: a.Title(),
	}

	for _, outcome := range hostAnalyzer.Outcomes {

		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = renderFSPerfOutcome(outcome.Fail.Message, fsPerf)
				result.URI = outcome.Fail.URI

				return []*AnalyzeResult{result}, nil
			}

			isMatch, err := compareHostFilesystemPerformanceConditionalToActual(outcome.Fail.When, fsPerf)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %q", outcome.Fail.When)
			}

			if isMatch {
				result.IsFail = true
				result.Message = renderFSPerfOutcome(outcome.Fail.Message, fsPerf)
				result.URI = outcome.Fail.URI

				return []*AnalyzeResult{result}, nil
			}
		} else if outcome.Warn != nil {
			if outcome.Warn.When == "" {
				result.IsWarn = true
				result.Message = renderFSPerfOutcome(outcome.Warn.Message, fsPerf)
				result.URI = outcome.Warn.URI

				return []*AnalyzeResult{result}, nil
			}

			isMatch, err := compareHostFilesystemPerformanceConditionalToActual(outcome.Warn.When, fsPerf)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %q", outcome.Warn.When)
			}

			if isMatch {
				result.IsWarn = true
				result.Message = renderFSPerfOutcome(outcome.Warn.Message, fsPerf)
				result.URI = outcome.Warn.URI

				return []*AnalyzeResult{result}, nil
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = renderFSPerfOutcome(outcome.Pass.Message, fsPerf)
				result.URI = outcome.Pass.URI

				return []*AnalyzeResult{result}, nil
			}

			isMatch, err := compareHostFilesystemPerformanceConditionalToActual(outcome.Pass.When, fsPerf)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %q", outcome.Pass.When)
			}

			if isMatch {
				result.IsPass = true
				result.Message = renderFSPerfOutcome(outcome.Pass.Message, fsPerf)
				result.URI = outcome.Pass.URI

				return []*AnalyzeResult{result}, nil
			}

		}
	}

	return []*AnalyzeResult{result}, nil
}

func compareHostFilesystemPerformanceConditionalToActual(conditional string, fsPerf collect.FSPerfResults) (res bool, err error) {
	parts := strings.Split(conditional, " ")
	if len(parts) != 3 {
		return false, fmt.Errorf("conditional must have exactly 3 parts, got %d", len(parts))
	}
	keyword := strings.ToLower(parts[0])
	comparator := parts[1]

	desiredDuration, err := time.ParseDuration(parts[2])
	if err != nil {
		return false, errors.Wrapf(err, "failed to parse duration %q", parts[2])
	}

	switch keyword {
	case "min":
		return doCompareHostFilesystemPerformance(comparator, fsPerf.Min, desiredDuration)
	case "max":
		return doCompareHostFilesystemPerformance(comparator, fsPerf.Max, desiredDuration)
	case "average":
		return doCompareHostFilesystemPerformance(comparator, fsPerf.Average, desiredDuration)
	case "p1":
		return doCompareHostFilesystemPerformance(comparator, fsPerf.P1, desiredDuration)
	case "p5":
		return doCompareHostFilesystemPerformance(comparator, fsPerf.P5, desiredDuration)
	case "p10":
		return doCompareHostFilesystemPerformance(comparator, fsPerf.P10, desiredDuration)
	case "p20":
		return doCompareHostFilesystemPerformance(comparator, fsPerf.P20, desiredDuration)
	case "p30":
		return doCompareHostFilesystemPerformance(comparator, fsPerf.P30, desiredDuration)
	case "p40":
		return doCompareHostFilesystemPerformance(comparator, fsPerf.P40, desiredDuration)
	case "p50":
		return doCompareHostFilesystemPerformance(comparator, fsPerf.P50, desiredDuration)
	case "p60":
		return doCompareHostFilesystemPerformance(comparator, fsPerf.P60, desiredDuration)
	case "p70":
		return doCompareHostFilesystemPerformance(comparator, fsPerf.P70, desiredDuration)
	case "p80":
		return doCompareHostFilesystemPerformance(comparator, fsPerf.P80, desiredDuration)
	case "p90":
		return doCompareHostFilesystemPerformance(comparator, fsPerf.P90, desiredDuration)
	case "p95":
		return doCompareHostFilesystemPerformance(comparator, fsPerf.P95, desiredDuration)
	case "p99":
		return doCompareHostFilesystemPerformance(comparator, fsPerf.P99, desiredDuration)
	case "p995":
		return doCompareHostFilesystemPerformance(comparator, fsPerf.P995, desiredDuration)
	case "p999":
		return doCompareHostFilesystemPerformance(comparator, fsPerf.P999, desiredDuration)
	case "p9995":
		return doCompareHostFilesystemPerformance(comparator, fsPerf.P9995, desiredDuration)
	case "p9999":
		return doCompareHostFilesystemPerformance(comparator, fsPerf.P9999, desiredDuration)
	}

	return false, fmt.Errorf("Unknown filesystem performance keyword %q", keyword)
}

func doCompareHostFilesystemPerformance(operator string, actual time.Duration, desired time.Duration) (bool, error) {
	switch operator {
	case "<":
		return actual < desired, nil
	case "<=":
		return actual <= desired, nil
	case ">":
		return actual > desired, nil
	case ">=":
		return actual >= desired, nil
	case "=", "==", "===":
		return actual == desired, nil
	}

	return false, fmt.Errorf("Unknown filesystem performance operator %q", operator)
}

func renderFSPerfOutcome(outcome string, fsPerf collect.FSPerfResults) string {
	t, err := template.New("").Parse(outcome)
	if err != nil {
		log.Printf("Failed to parse filesystem performance outcome: %v", err)
		return outcome
	}
	var buf bytes.Buffer
	err = t.Execute(&buf, fsPerf)
	if err != nil {
		log.Printf("Failed to render filesystem performance outcome: %v", err)
		return outcome
	}
	return buf.String()
}
