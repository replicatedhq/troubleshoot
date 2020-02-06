package analyzer

import (
	"encoding/json"
	"fmt"
	"regexp"
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

	nodes := []corev1.Node{}
	if err := json.Unmarshal(collected, &nodes); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal node list")
	}

	matchingNodes := []corev1.Node{}

	for _, node := range nodes {
		isMatch, err := nodeMatchesFilters(node, analyzer.Filters)
		if err != nil {
			return nil, errors.Wrap(err, "failed to check if node matches filter")
		}

		if isMatch {
			matchingNodes = append(matchingNodes, node)
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
			isWhenMatch, err := compareNodeResourceConditionalToActual(outcome.Fail.When, matchingNodes, len(nodes))
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
			isWhenMatch, err := compareNodeResourceConditionalToActual(outcome.Warn.When, matchingNodes, len(nodes))
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
			isWhenMatch, err := compareNodeResourceConditionalToActual(outcome.Pass.When, matchingNodes, len(nodes))
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

func compareNodeResourceConditionalToActual(conditional string, matchingNodes []corev1.Node, totalNodeCount int) (bool, error) {
	if conditional == "" {
		return true, nil
	}

	parts := strings.Split(strings.TrimSpace(conditional), " ")

	if len(parts) == 2 {
		parts = append([]string{"count"}, parts...)
	}

	if len(parts) != 3 {
		return false, errors.New("unable to parse nodeResources conditional")
	}

	operator := parts[1]

	var desiredValue interface{}
	desiredValue = parts[2]

	parsedDesiredValue, err := strconv.Atoi(parts[2])
	if err == nil {
		desiredValue = parsedDesiredValue
	}

	reg := regexp.MustCompile(`(?P<function>.*)\((?P<property>.*)\)`)
	match := reg.FindStringSubmatch(parts[0])

	if match == nil {
		// We support this as equivalent to the count() function
		match = reg.FindStringSubmatch(fmt.Sprintf("count() == %s", parts[0]))
	}

	if match == nil || len(match) != 3 {
		return false, errors.New("conditional does not match pattern of function(property?)")
	}

	function := match[1]
	property := match[2]

	var actualValue interface{}

	switch function {
	case "count":
		actualValue = len(matchingNodes)
		break
	case "min":
		av, err := findMin(matchingNodes, property)
		if err != nil {
			return false, errors.Wrap(err, "failed to find min")
		}
		actualValue = av
	}

	switch operator {
	case "=", "==", "===":
		return desiredValue == actualValue, nil
	case "<":
		if _, ok := actualValue.(int); ok {
			if _, ok := desiredValue.(int); ok {
				return actualValue.(int) < desiredValue.(int), nil
			}
		}
		return actualValue.(*resource.Quantity).Cmp(resource.MustParse(desiredValue.(string))) == -1, nil

	case ">":
		if _, ok := actualValue.(int); ok {
			if _, ok := desiredValue.(int); ok {
				return actualValue.(int) > desiredValue.(int), nil
			}
		}
		return actualValue.(*resource.Quantity).Cmp(resource.MustParse(desiredValue.(string))) == 1, nil

	case "<=":
		if _, ok := actualValue.(int); ok {
			if _, ok := desiredValue.(int); ok {
				return actualValue.(int) <= desiredValue.(int), nil
			}
		}
		return actualValue.(*resource.Quantity).Cmp(resource.MustParse(desiredValue.(string))) == -1 ||
			actualValue.(*resource.Quantity).Cmp(resource.MustParse(desiredValue.(string))) == 0, nil

	case ">=":
		if _, ok := actualValue.(int); ok {
			if _, ok := desiredValue.(int); ok {
				return actualValue.(int) >= desiredValue.(int), nil
			}
		}
		return actualValue.(*resource.Quantity).Cmp(resource.MustParse(desiredValue.(string))) == 1 ||
			actualValue.(*resource.Quantity).Cmp(resource.MustParse(desiredValue.(string))) == 0, nil
	}

	return false, errors.New("unexpected conditional in nodeResources")
}

func findMin(nodes []corev1.Node, property string) (*resource.Quantity, error) {
	var min *resource.Quantity

	for _, node := range nodes {
		switch property {
		case "cpuCapacity":
			if min == nil {
				min = node.Status.Capacity.Cpu()
			} else {
				if node.Status.Capacity.Cpu().Cmp(*min) == -1 {
					min = node.Status.Capacity.Cpu()
				}
			}
			break
		case "cpuAllocatable":
			if min == nil {
				min = node.Status.Allocatable.Cpu()
			} else {
				if node.Status.Allocatable.Cpu().Cmp(*min) == -1 {
					min = node.Status.Allocatable.Cpu()
				}
			}
			break
		case "memoryCapacity":
			if min == nil {
				min = node.Status.Capacity.Memory()
			} else {
				if node.Status.Capacity.Memory().Cmp(*min) == -1 {
					min = node.Status.Capacity.Memory()
				}
			}
			break
		case "memoryAllocatable":
			if min == nil {
				min = node.Status.Allocatable.Memory()
			} else {
				if node.Status.Allocatable.Memory().Cmp(*min) == -1 {
					min = node.Status.Allocatable.Memory()
				}
			}
			break
		case "podCapacity":
			if min == nil {
				min = node.Status.Capacity.Pods()
			} else {
				if node.Status.Capacity.Pods().Cmp(*min) == -1 {
					min = node.Status.Capacity.Pods()
				}
			}
			break
		case "podAllocatable":
			if min == nil {
				min = node.Status.Allocatable.Pods()
			} else {
				if node.Status.Allocatable.Pods().Cmp(*min) == -1 {
					min = node.Status.Allocatable.Pods()
				}
			}
			break
		case "ephemeralStorageCapacity":
			if min == nil {
				min = node.Status.Capacity.StorageEphemeral()
			} else {
				if node.Status.Capacity.StorageEphemeral().Cmp(*min) == -1 {
					min = node.Status.Capacity.StorageEphemeral()
				}
			}
			break
		case "ephemeralStorageAllocatable":
			if min == nil {
				min = node.Status.Allocatable.StorageEphemeral()
			} else {
				if node.Status.Allocatable.StorageEphemeral().Cmp(*min) == -1 {
					min = node.Status.Allocatable.StorageEphemeral()
				}
			}
			break

		}
	}

	return min, nil
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
