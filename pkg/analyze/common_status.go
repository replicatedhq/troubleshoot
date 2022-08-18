package analyzer

import (
	"strconv"
	"strings"
	"fmt"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

func commonStatus(outcomes []*troubleshootv1beta2.Outcome, name string, iconKey string, iconURI string, readyReplicas int, exists bool, resourceType string) (*AnalyzeResult, error) {
	result := &AnalyzeResult{
		Title:   fmt.Sprintf("%s Status", name),
		IconKey: iconKey,
		IconURI: iconURI,
	}

	// ordering from the spec is important, the first one that matches returns
	for _, outcome := range outcomes {

		if outcome.Fail != nil {

			// if we're not checking that something is absent but it is, we should throw a default but meaningful error.
			if exists == false && outcome.Fail.When != "absent" {
				result.IsFail = true
				result.Message = fmt.Sprintf("The %s %q was not found", resourceType, name)
				result.URI = outcome.Fail.URI
				return result, nil
			}

			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}

			if  outcome.Fail.When == "absent"  {
				if exists == false {
					result.IsFail = true
					result.Message = outcome.Fail.Message
					result.URI = outcome.Fail.URI
					return result, nil
				} else {
					continue
				}
			}

			match, err := compareActualToWhen(outcome.Fail.When, readyReplicas, exists)
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

			if exists == false && outcome.Warn.When != "absent" {
				result.IsFail = true
				result.Message = fmt.Sprintf("The %s %q was not found", resourceType, name)
				result.URI = outcome.Fail.URI
				return result, nil
			}

			if outcome.Warn.When == "" {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return result, nil
			}

			if outcome.Warn.When == "absent" {
				if exists == false {
					result.IsWarn = true
					result.Message = outcome.Warn.Message
					result.URI = outcome.Warn.URI
					return result, nil
				} else {
					continue
				}
			}

			match, err := compareActualToWhen(outcome.Warn.When, readyReplicas, exists)
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

			if exists == false && outcome.Pass.When != "absent" {
				result.IsFail = true
				result.Message = fmt.Sprintf("The %s %q was not found", resourceType, name)
				result.URI = outcome.Fail.URI
				return result, nil
			}

			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return result, nil
			}

			if outcome.Pass.When == "absent" {
				if exists == false {
					result.IsPass = true
					result.Message = outcome.Pass.Message
					result.URI = outcome.Pass.URI
					return result, nil
				} else {
					continue
				}
			}

			match, err := compareActualToWhen(outcome.Pass.When, readyReplicas, exists)
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

func compareActualToWhen(when string, actual int, exists bool) (bool, error) {
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
