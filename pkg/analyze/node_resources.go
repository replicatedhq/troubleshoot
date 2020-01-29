package analyzer

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func analyzeNodeResources(analyzer *troubleshootv1beta1.NodeResources, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	collected, err := getCollectedFileContents("cluster-resources/nodes.json")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get contents of nodes.json")
	}

	var nodes []corev1.Node
	if err := json.Unmarshal(collected, &nodes); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal node list")
	}

	matchingNodeCount := 0

	for _, node := range nodes {
		isMatch, err := nodeMatchesFilters(node, analyzer.Filters)
		if err != nil {
			return nil, errors.Wrap(err, "failed to check if node matches filter")
		}

		if isMatch {
			matchingNodeCount++
		}
	}

	title := analyzer.CheckName
	if title == "" {
		title = "Node Resources"
	}

	result := &AnalyzeResult{
		Title: title,
	}

	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			isWhenMatch, err := compareNodeResourceConditionalToActual(outcome.Fail.When, matchingNodeCount)
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
			isWhenMatch, err := compareNodeResourceConditionalToActual(outcome.Warn.When, matchingNodeCount)
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
			isWhenMatch, err := compareNodeResourceConditionalToActual(outcome.Pass.When, matchingNodeCount)
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

func compareNodeResourceConditionalToActual(conditional string, actual int) (bool, error) {
	if conditional == "" {
		return true, nil
	}

	parts := strings.Split(strings.TrimSpace(conditional), " ")

	if len(parts) != 2 {
		return false, errors.New("unable to parse nodeResources conditional")
	}

	operator := parts[0]
	desiredValue, err := strconv.Atoi(parts[1])
	if err != nil {
		return false, errors.Wrap(err, "failed to parse nodeResource value")
	}

	switch operator {
	case "=", "==", "===":
		return desiredValue == actual, nil
	case "<":
		return actual < desiredValue, nil
	case "<=":
		return actual <= desiredValue, nil
	case ">":
		return actual > desiredValue, nil
	case ">=":
		return actual >= desiredValue, nil
	}

	return false, errors.New("unexpected conditional in nodeResources")
}

func nodeMatchesFilters(node corev1.Node, filters *troubleshootv1beta1.NodeResourceFilters) (bool, error) {
	if filters == nil {
		return true, nil
	}

	// all filters must pass for this to pass

	if filters.CPUCapacity != "" {
		parsed, err := resource.ParseQuantity(filters.CPUCapacity)
		if err != nil {
			return false, errors.Wrap(err, "failed to parse cpu capacity")
		}

		if node.Status.Capacity.Cpu().Cmp(parsed) == -1 {
			return false, nil
		}
	}
	if filters.CPUAllocatable != "" {
		parsed, err := resource.ParseQuantity(filters.CPUAllocatable)
		if err != nil {
			return false, errors.Wrap(err, "failed to parse cpu allocatable")
		}

		if node.Status.Allocatable.Cpu().Cmp(parsed) == -1 {
			return false, nil
		}
	}

	if filters.MemoryCapacity != "" {
		parsed, err := resource.ParseQuantity(filters.MemoryCapacity)
		if err != nil {
			return false, errors.Wrap(err, "failed to parse memory capacity")
		}

		if node.Status.Capacity.Memory().Cmp(parsed) == -1 {
			return false, nil
		}
	}
	if filters.MemoryAllocatable != "" {
		parsed, err := resource.ParseQuantity(filters.MemoryAllocatable)
		if err != nil {
			return false, errors.Wrap(err, "failed to parse memory allocatable")
		}

		if node.Status.Allocatable.Memory().Cmp(parsed) == -1 {
			return false, nil
		}
	}

	if filters.PodCapacity != "" {
		parsed, err := resource.ParseQuantity(filters.PodCapacity)
		if err != nil {
			return false, errors.Wrap(err, "failed to parse pod capacity")
		}

		if node.Status.Capacity.Pods().Cmp(parsed) == -1 {
			return false, nil
		}
	}
	if filters.PodAllocatable != "" {
		parsed, err := resource.ParseQuantity(filters.PodAllocatable)
		if err != nil {
			return false, errors.Wrap(err, "failed to parse pod allocatable")
		}

		if node.Status.Allocatable.Pods().Cmp(parsed) == -1 {
			return false, nil
		}
	}

	if filters.EphemeralStorageCapacity != "" {
		parsed, err := resource.ParseQuantity(filters.EphemeralStorageCapacity)
		if err != nil {
			return false, errors.Wrap(err, "failed to parse ephemeralstorage capacity")
		}

		if node.Status.Capacity.StorageEphemeral().Cmp(parsed) == -1 {
			return false, nil
		}
	}
	if filters.EphemeralStorageAllocatable != "" {
		parsed, err := resource.ParseQuantity(filters.EphemeralStorageAllocatable)
		if err != nil {
			return false, errors.Wrap(err, "failed to parse ephemeralstorage allocatable")
		}

		if node.Status.Allocatable.StorageEphemeral().Cmp(parsed) == -1 {
			return false, nil
		}
	}

	return true, nil
}
