package analyzer

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"k8s.io/apimachinery/pkg/api/resource"
)

func analyzeHostMemory(hostAnalyzer *troubleshootv1beta2.MemoryAnalyze, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	contents, err := getCollectedFileContents("system/memory.json")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get collected file")
	}

	memoryInfo := collect.MemoryInfo{}
	if err := json.Unmarshal(contents, &memoryInfo); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal memory info")
	}

	result := AnalyzeResult{}

	title := hostAnalyzer.CheckName
	if title == "" {
		title = "Amount of Memory"
	}
	result.Title = title

	for _, outcome := range hostAnalyzer.Outcomes {
		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return &result, nil
			}

			isMatch, err := compareHostMemoryConditionalToActual(outcome.Fail.When, memoryInfo.Total)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %s", outcome.Fail.When)
			}

			if isMatch {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return &result, nil
			}
		} else if outcome.Warn != nil {
			if outcome.Warn.When == "" {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return &result, nil
			}

			isMatch, err := compareHostMemoryConditionalToActual(outcome.Warn.When, memoryInfo.Total)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %s", outcome.Warn.When)
			}

			if isMatch {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return &result, nil
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return &result, nil
			}

			isMatch, err := compareHostMemoryConditionalToActual(outcome.Pass.When, memoryInfo.Total)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %s", outcome.Pass.When)
			}

			if isMatch {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return &result, nil
			}
		}
	}

	return &result, nil
}

func compareHostMemoryConditionalToActual(conditional string, total uint64) (res bool, err error) {
	parts := strings.Split(conditional, " ")
	if len(parts) != 2 {
		return false, fmt.Errorf("Expected 2 parts in conditional, got %d", len(parts))
	}

	operator := parts[0]
	desired := parts[1]
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
		return total < uint64(desiredInt), nil
	case "<=":
		return total <= uint64(desiredInt), nil
	case ">":
		return total > uint64(desiredInt), nil
	case ">=":
		return total >= uint64(desiredInt), nil
	case "=", "==", "===":
		return total == uint64(desiredInt), nil
	}

	return false, errors.New("unknown operator")
}
