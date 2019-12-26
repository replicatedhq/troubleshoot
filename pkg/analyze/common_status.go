package analyzer

import (
	"strconv"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
)

func commonStatus(outcomes []*troubleshootv1beta1.Outcome, title string, readyReplicas int) (*AnalyzeResult, error) {
	result := &AnalyzeResult{
		Title: title,
	}

	// ordering from the spec is important, the first one that matches returns
	for _, outcome := range outcomes {
		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}

			match, err := compareActualToWhen(outcome.Fail.When, readyReplicas)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse fail range")
			}

			if match {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}
		} else if outcome.Warn != nil {
			if outcome.Warn.When == "" {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return result, nil
			}

			match, err := compareActualToWhen(outcome.Warn.When, readyReplicas)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse warn range")
			}

			if match {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return result, nil
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return result, nil
			}

			match, err := compareActualToWhen(outcome.Pass.When, readyReplicas)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse pass range")
			}

			if match {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return result, nil
			}
		}
	}

	return result, nil
}

func compareActualToWhen(when string, actual int) (bool, error) {
	parts := strings.Split(strings.TrimSpace(when), " ")

	// we can make this a lot more flexible
	if len(parts) != 2 {
		return false, errors.New("unable to parse when range")
	}

	value, err := strconv.Atoi(parts[1])
	if err != nil {
		return false, errors.Wrap(err, "failed to parse when value")
	}

	switch parts[0] {
	case "=":
		fallthrough
	case "==":
		fallthrough
	case "===":
		return actual == value, nil

	case "<":
		return actual < value, nil

	case ">":
		return actual > value, nil

	case "<=":
		return actual <= value, nil

	case ">=":
		return actual >= value, nil
	}

	return false, errors.Errorf("unknown comparator: %q", parts[0])
}
