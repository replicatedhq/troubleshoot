package analyzer

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
)

const (
	CephStatusGood string = "HEALTH_OK"
	CephStatusWarn string = "HEALTH_WARN"
	CephStatusErr  string = "HEALTH_ERR"
)

type CephHealth struct {
	Status string `json:"status"`
}

type CephStatus struct {
	Health CephHealth `json:"health"`
}

func analyzeRook(analyzer *troubleshootv1beta1.RookStatus, getRookCollection func(string) ([]byte, error)) (*AnalyzeResult, error) {
	title := analyzer.CheckName
	if title == "" {
		title = "Rook Status"
	}

	result := &AnalyzeResult{
		Title:   title,
		IconKey: "", // TODO: we'll eventually remove the IconKey
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/rook.svg?w=11&h=16",
	}

	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			isWhenMatch, err := compareRookStatusToActual(outcome.Fail.When, getRookCollection)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse when")
			}

			if isWhenMatch {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}
		} else if outcome.Warn != nil {
			isWhenMatch, err := compareRookStatusToActual(outcome.Warn.When, getRookCollection)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse when")
			}

			if isWhenMatch {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return result, nil
			}
		} else if outcome.Pass != nil {
			isWhenMatch, err := compareRookStatusToActual(outcome.Pass.When, getRookCollection)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse when")
			}

			if isWhenMatch {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return result, nil
			}
		}
	}

	return result, nil
}

func compareRookStatusToActual(conditional string, getRookCollection func(string) ([]byte, error)) (bool, error) {
	if conditional == "" {
		return true, nil
	}

	parts := strings.Fields(strings.TrimSpace(conditional))
	if len(parts) != 3 {
		return false, errors.New("unable to parse rookAnalyzer conditional")
	}

	check := parts[0]
	operator := parts[1]
	desiredValue := parts[2]

	switch check {
	case "Status":
		return checkRookStatus(operator, desiredValue, getRookCollection)
	}

	return false, errors.New("Invalid rook check")
}

func checkRookStatus(operator, desiredValue string, getRookCollection func(string) ([]byte, error)) (bool, error) {
	statusBytes, err := getRookCollection("rook/status/status.json")
	if err != nil {
		return false, errors.New("failed to get contents of rook status.json")
	}
	status := CephStatus{}
	if err := json.Unmarshal(statusBytes, &status); err != nil {
		return false, err
	}

	switch operator {
	case "=", "==", "===":
		if status.Health.Status != desiredValue {
			return false, nil
		} else {
			return true, nil
		}
	}
	return false, errors.New("unexpected conditional")
}
