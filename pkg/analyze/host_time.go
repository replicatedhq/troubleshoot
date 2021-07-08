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

func (a *AnalyzeHostTime) Analyze(getCollectedFileContents func(string) ([]byte, error)) ([]*AnalyzeResult, error) {
	hostAnalyzer := a.hostAnalyzer

	contents, err := getCollectedFileContents("system/time.json")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get collected file")
	}

	timeInfo := collect.TimeInfo{}
	if err := json.Unmarshal(contents, &timeInfo); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal time info")
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
				continue
			}

			isMatch, err := compareHostTimeStatusToActual(outcome.Fail.When, timeInfo)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %s", outcome.Fail.When)
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
				continue
			}

			isMatch, err := compareHostTimeStatusToActual(outcome.Warn.When, timeInfo)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %s", outcome.Warn.When)
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
				continue
			}

			isMatch, err := compareHostTimeStatusToActual(outcome.Pass.When, timeInfo)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %s", outcome.Pass.When)
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
