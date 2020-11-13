package analyzer

import (
	"encoding/json"
	"path"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
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

func cephStatus(analyzer *troubleshootv1beta2.CephStatusAnalyze, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	fileName := path.Join(collect.GetCephCollectorFilepath(analyzer.CollectorName, analyzer.Namespace), "status.json")
	collected, err := getCollectedFileContents(fileName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read collected ceph status")
	}

	title := analyzer.CheckName
	if title == "" {
		title = "Ceph Status"
	}

	analyzeResult := &AnalyzeResult{
		Title:   title,
		IconKey: "rook", // maybe this should be ceph?
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/rook.svg?w=11&h=16",
	}

	status := struct {
		Health struct {
			Status string `json:"status"`
		} `json:"health"`
	}{}
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
				analyzeResult.Message = outcome.Fail.Message
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
				analyzeResult.Message = outcome.Warn.Message
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
