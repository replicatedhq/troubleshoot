package analyzer

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/types"
)

type CephHealth string

const (
	CephHealthOK   CephHealth = "HEALTH_OK"
	CephHealthWarn CephHealth = "HEALTH_WARN"
	CephHealthErr  CephHealth = "HEALTH_ERR"
)

func (a CephHealth) Compare(b CephHealth) int {
	if a == b {
		return 0
	}
	switch a {
	case CephHealthOK:
		return 1
	case CephHealthWarn:
		switch b {
		case CephHealthOK:
			return -1
		case CephHealthErr:
			return 1
		}
		return 1
	case CephHealthErr:
		switch b {
		case CephHealthOK, CephHealthWarn:
			return -1
		}
		return 1
	default:
		return -1
	}
}

var CephStatusDefaultOutcomes = []*troubleshootv1beta2.Outcome{
	{
		Pass: &troubleshootv1beta2.SingleOutcome{
			Message: "Ceph is healthy",
		},
	},
	{
		Warn: &troubleshootv1beta2.SingleOutcome{
			Message: "Ceph status is HEALTH_WARN",
			URI:     "https://rook.io/docs/rook/v1.4/ceph-common-issues.html",
		},
	},
	{
		Fail: &troubleshootv1beta2.SingleOutcome{
			Message: "Ceph status is HEALTH_ERR",
			URI:     "https://rook.io/docs/rook/v1.4/ceph-common-issues.html",
		},
	},
}

type CephStatus struct {
	Health HealthStatus `json:"health"`
	OsdMap struct {
		OsdMap OsdMap `json:"osdmap"`
	} `json:"osdmap"`
	PgMap PgMap `json:"pgmap"`
}

type HealthStatus struct {
	Status string                  `json:"status"`
	Checks map[string]CheckMessage `json:"checks"`
}

type CheckMessage struct {
	Severity string  `json:"severity"`
	Summary  Summary `json:"summary"`
}

type Summary struct {
	Message string `json:"message"`
}

type OsdMap struct {
	NumOsd   int  `json:"num_osds"`
	NumUpOsd int  `json:"num_up_osds"`
	Full     bool `json:"full"`
	NearFull bool `json:"nearfull"`
}

type PgMap struct {
	UsedBytes  uint64 `json:"bytes_used"`
	TotalBytes uint64 `json:"bytes_total"`
}

type AnalyzeCephStatus struct {
	analyzer *troubleshootv1beta2.CephStatusAnalyze
}

func (a *AnalyzeCephStatus) Title() string {
	title := a.analyzer.CheckName
	if title == "" {
		title = "Ceph Status"
	}

	return title
}

func (a *AnalyzeCephStatus) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeCephStatus) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	result, err := a.cephStatus(a.analyzer, getFile)
	if err != nil {
		return nil, err
	}
	if result != nil {
		result.Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	}
	return []*AnalyzeResult{result}, nil
}

func (a *AnalyzeCephStatus) cephStatus(analyzer *troubleshootv1beta2.CephStatusAnalyze, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	fileName := path.Join(collect.GetCephCollectorFilepath(analyzer.CollectorName, analyzer.Namespace), "status.json")
	collected, err := getCollectedFileContents(fileName)

	if err != nil {
		if _, ok := err.(*types.NotFoundError); ok {
			return nil, nil
		}
		return nil, errors.Wrap(err, "failed to read collected ceph status")
	}

	analyzeResult := &AnalyzeResult{
		Title:   a.Title(),
		IconKey: "rook", // maybe this should be ceph?
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/rook.svg?w=11&h=16",
	}

	status := CephStatus{}
	if err := json.Unmarshal(collected, &status); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal status.json")
	}

	if len(analyzer.Outcomes) == 0 {
		analyzer.Outcomes = CephStatusDefaultOutcomes
	}

	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				outcome.Fail.When = string(CephHealthErr)
			}

			match, err := compareCephStatus(status.Health.Status, outcome.Fail.When)
			if err != nil {
				return nil, errors.Wrap(err, "failed to compare ceph status")
			} else if match {
				analyzeResult.IsFail = true
				analyzeResult.Message = detailedCephMessage(outcome.Fail.Message, status)
				analyzeResult.URI = outcome.Fail.URI
				return analyzeResult, nil
			}
		} else if outcome.Warn != nil {
			if outcome.Warn.When == "" {
				outcome.Warn.When = string(CephHealthWarn)
			}

			match, err := compareCephStatus(status.Health.Status, outcome.Warn.When)
			if err != nil {
				return nil, errors.Wrap(err, "failed to compare ceph status")
			} else if match {
				analyzeResult.IsWarn = true
				analyzeResult.Message = detailedCephMessage(outcome.Warn.Message, status)
				analyzeResult.URI = outcome.Warn.URI
				return analyzeResult, nil
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" {
				outcome.Pass.When = string(CephHealthOK)
			}

			match, err := compareCephStatus(status.Health.Status, outcome.Pass.When)
			if err != nil {
				return nil, errors.Wrap(err, "failed to compare ceph status")
			} else if match {
				analyzeResult.IsPass = true
				analyzeResult.Message = outcome.Pass.Message
				analyzeResult.URI = outcome.Pass.URI

				return analyzeResult, nil
			}
		}
	}

	return analyzeResult, nil
}

func compareCephStatus(actual, when string) (bool, error) {
	parts := strings.Split(strings.TrimSpace(when), " ")

	if len(parts) == 1 {
		value := strings.TrimSpace(parts[0])
		return value == actual, nil
	}

	if len(parts) != 2 {
		return false, errors.New("unable to parse when range")
	}

	operator := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	compareResult := CephHealth(actual).Compare(CephHealth(value))

	switch operator {
	case "=", "==", "===":
		return compareResult == 0, nil
	case "<":
		return compareResult == -1, nil
	case ">":
		return compareResult == 1, nil
	case "<=":
		return compareResult <= 0, nil
	case ">=":
		return compareResult >= 0, nil
	default:
		return false, errors.New("unknown operator")
	}
}

func detailedCephMessage(outcomeMessage string, status CephStatus) string {
	var msg = []string{}

	if outcomeMessage != "" {
		msg = append(msg, outcomeMessage)
	}

	if status.OsdMap.OsdMap.NumOsd > 0 {
		msg = append(msg, fmt.Sprintf("%v/%v OSDs up", status.OsdMap.OsdMap.NumUpOsd, status.OsdMap.OsdMap.NumOsd))
	}

	if status.OsdMap.OsdMap.Full {
		msg = append(msg, "OSD disk is full")
	} else if status.OsdMap.OsdMap.NearFull {
		msg = append(msg, "OSD disk is nearly full")
	}

	if status.PgMap.TotalBytes > 0 {
		pgUsage := 100 * float64(status.PgMap.UsedBytes) / float64(status.PgMap.TotalBytes)
		msg = append(msg, fmt.Sprintf("PG storage usage is %.1f%%", pgUsage))
	}

	if status.Health.Checks != nil {
		for k, v := range status.Health.Checks {
			msg = append(msg, fmt.Sprintf("%s: %s", k, v.Summary.Message))
		}
	}

	return strings.Join(msg, "\n")
}
