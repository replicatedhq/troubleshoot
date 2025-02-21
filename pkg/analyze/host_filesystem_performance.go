package analyzer

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/internal/util"
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
	result := &AnalyzeResult{Title: a.Title()}
	hostAnalyzer := a.hostAnalyzer
	collectorName := a.hostAnalyzer.CollectorName

	if collectorName == "" {
		collectorName = "filesystemPerformance"
	}
	const nodeBaseDir = "host-collectors/filesystemPerformance"
	localPath := fmt.Sprintf("%s/%s.json", nodeBaseDir, collectorName)
	fileName := fmt.Sprintf("%s.json", collectorName)

	collectedContents, err := retrieveCollectedContents(
		getCollectedFileContents,
		localPath,
		nodeBaseDir,
		fileName,
	)
	if err != nil {
		if len(hostAnalyzer.Outcomes) >= 1 {
			// if the very first outcome is FILE_NOT_COLLECTED', then use that outcome
			// otherwise, return the error
			if hostAnalyzer.Outcomes[0].Fail != nil && hostAnalyzer.Outcomes[0].Fail.When == FILE_NOT_COLLECTED {
				result.IsFail = true
				result.Message = renderFSPerfOutcome(hostAnalyzer.Outcomes[0].Fail.Message, collect.FSPerfResults{})
				result.URI = hostAnalyzer.Outcomes[0].Fail.URI
				return []*AnalyzeResult{result}, nil
			}
			if hostAnalyzer.Outcomes[0].Warn != nil && hostAnalyzer.Outcomes[0].Warn.When == FILE_NOT_COLLECTED {
				result.IsWarn = true
				result.Message = renderFSPerfOutcome(hostAnalyzer.Outcomes[0].Warn.Message, collect.FSPerfResults{})
				result.URI = hostAnalyzer.Outcomes[0].Warn.URI
				return []*AnalyzeResult{result}, nil
			}
			if hostAnalyzer.Outcomes[0].Pass != nil && hostAnalyzer.Outcomes[0].Pass.When == FILE_NOT_COLLECTED {
				result.IsPass = true
				result.Message = renderFSPerfOutcome(hostAnalyzer.Outcomes[0].Pass.Message, collect.FSPerfResults{})
				result.URI = hostAnalyzer.Outcomes[0].Pass.URI
				return []*AnalyzeResult{result}, nil
			}
			return nil, errors.Wrapf(err, "failed to get collected file %s", localPath)
		}
	}

	var results []*AnalyzeResult
	for _, content := range collectedContents {
		currentTitle := a.Title()
		if content.NodeName != "" {
			currentTitle = fmt.Sprintf("%s - Node %s", a.Title(), content.NodeName)
		}
		result, err := a.analyzeSingleNode(content, currentTitle)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to analyze filesystem performance for %s", currentTitle)
		}
		if result != nil {
			results = append(results, result...)
		}
	}

	return results, nil
}

func compareHostFilesystemPerformanceConditionalToActual(conditional string, fsPerf collect.FSPerfResults) (res bool, err error) {
	if conditional == FILE_NOT_COLLECTED {
		return false, nil
	}

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

	return false, fmt.Errorf("unknown filesystem performance keyword %q", keyword)
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

	return false, fmt.Errorf("unknown filesystem performance operator %q", operator)
}

func renderFSPerfOutcome(outcome string, fsPerf collect.FSPerfResults) string {
	rendered, err := util.RenderTemplate(outcome, fsPerf)
	if err != nil {
		log.Printf("Failed to render filesystem performance outcome: %v", err)
		return outcome
	}
	return rendered
}

func (a *AnalyzeHostFilesystemPerformance) analyzeSingleNode(content collectedContent, currentTitle string) ([]*AnalyzeResult, error) {
	hostAnalyzer := a.hostAnalyzer
	result := &AnalyzeResult{
		Title: currentTitle,
	}
	fioResult := collect.FioResult{}
	if err := json.Unmarshal(content.Data, &fioResult); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal fio results from %s", currentTitle)
	}

	var job *collect.FioJobs
	for _, j := range fioResult.Jobs {
		if j.JobName == collect.FioJobName {
			job = &j
			break
		}
	}
	if job == nil {
		return nil, errors.Errorf("no job named 'fsperf' found in fio results from %s", currentTitle)
	}

	fioWriteLatency := job.Sync

	fsPerf := fioWriteLatency.FSPerfResults()
	if err := json.Unmarshal(content.Data, &fsPerf); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal filesystem performance results from %s", currentTitle)
	}

	for _, outcome := range hostAnalyzer.Outcomes {

		switch {
		case outcome.Fail != nil:
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
		case outcome.Warn != nil:
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
		case outcome.Pass != nil:
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
		default:
			return nil, errors.New("unexpected outcome")
		}
	}

	return []*AnalyzeResult{result}, nil
}
