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
		Title:   title,
		IconKey: "kubernetes_node_resources",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/node-resources.svg?w=16&h=18",
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

	parts := strings.Fields(strings.TrimSpace(conditional))

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
	case "min":
		actualValue = findMin(matchingNodes, property)
	case "max":
		actualValue = findMax(matchingNodes, property)
	case "sum":
		actualValue = findSum(matchingNodes, property)
	}

	switch operator {
	case "=", "==", "===":
		if _, ok := actualValue.(int); ok {
			if _, ok := desiredValue.(int); ok {
				return actualValue.(int) == desiredValue.(int), nil
			}
		}

		if _, ok := desiredValue.(string); ok {
			return actualValue.(*resource.Quantity).Cmp(resource.MustParse(desiredValue.(string))) == 0, nil
		}

		return actualValue.(*resource.Quantity).Cmp(resource.MustParse(strconv.Itoa(desiredValue.(int)))) == 0, nil

	case "<":
		if _, ok := actualValue.(int); ok {
			if _, ok := desiredValue.(int); ok {
				return actualValue.(int) < desiredValue.(int), nil
			}
		}
		if _, ok := desiredValue.(string); ok {
			return actualValue.(*resource.Quantity).Cmp(resource.MustParse(desiredValue.(string))) == -1, nil
		}

		return actualValue.(*resource.Quantity).Cmp(resource.MustParse(strconv.Itoa(desiredValue.(int)))) == -1, nil

	case ">":
		if _, ok := actualValue.(int); ok {
			if _, ok := desiredValue.(int); ok {
				return actualValue.(int) > desiredValue.(int), nil
			}
		}
		if _, ok := desiredValue.(string); ok {
			return actualValue.(*resource.Quantity).Cmp(resource.MustParse(desiredValue.(string))) == 1, nil
		}

		return actualValue.(*resource.Quantity).Cmp(resource.MustParse(strconv.Itoa(desiredValue.(int)))) == 1, nil

	case "<=":
		if _, ok := actualValue.(int); ok {
			if _, ok := desiredValue.(int); ok {
				return actualValue.(int) <= desiredValue.(int), nil
			}
		}
		if _, ok := desiredValue.(string); ok {
			return actualValue.(*resource.Quantity).Cmp(resource.MustParse(desiredValue.(string))) == 0 ||
				actualValue.(*resource.Quantity).Cmp(resource.MustParse(desiredValue.(string))) == -1, nil
		}

		return actualValue.(*resource.Quantity).Cmp(resource.MustParse(strconv.Itoa(desiredValue.(int)))) == 0 ||
			actualValue.(*resource.Quantity).Cmp(resource.MustParse(strconv.Itoa(desiredValue.(int)))) == -1, nil

	case ">=":
		if _, ok := actualValue.(int); ok {
			if _, ok := desiredValue.(int); ok {
				return actualValue.(int) >= desiredValue.(int), nil
			}
		}
		if _, ok := desiredValue.(string); ok {
			return actualValue.(*resource.Quantity).Cmp(resource.MustParse(desiredValue.(string))) == 0 ||
				actualValue.(*resource.Quantity).Cmp(resource.MustParse(desiredValue.(string))) == 1, nil
		}

		return actualValue.(*resource.Quantity).Cmp(resource.MustParse(strconv.Itoa(desiredValue.(int)))) == 0 ||
			actualValue.(*resource.Quantity).Cmp(resource.MustParse(strconv.Itoa(desiredValue.(int)))) == 1, nil
	}

	return false, errors.New("unexpected conditional in nodeResources")
}

func getQuantity(node corev1.Node, property string) *resource.Quantity {
	switch property {
	case "cpuCapacity":
		return node.Status.Capacity.Cpu()
	case "cpuAllocatable":
		return node.Status.Allocatable.Cpu()
	case "memoryCapacity":
		return node.Status.Capacity.Memory()
	case "memoryAllocatable":
		return node.Status.Allocatable.Memory()
	case "podCapacity":
		return node.Status.Capacity.Pods()
	case "podAllocatable":
		return node.Status.Allocatable.Pods()
	case "ephemeralStorageCapacity":
		return node.Status.Capacity.StorageEphemeral()
	case "ephemeralStorageAllocatable":
		return node.Status.Allocatable.StorageEphemeral()
	}
	return nil
}

func findSum(nodes []corev1.Node, property string) *resource.Quantity {
	sum := resource.Quantity{}

	for _, node := range nodes {
		if quant := getQuantity(node, property); quant != nil {
			sum.Add(*quant)
		}
	}

	return &sum
}

func findMin(nodes []corev1.Node, property string) *resource.Quantity {
	var min *resource.Quantity

	for _, node := range nodes {
		if quant := getQuantity(node, property); quant != nil {
			if min == nil {
				min = quant
			} else if quant.Cmp(*min) == -1 {
				min = quant
			}
		}
	}

	return min
}

func findMax(nodes []corev1.Node, property string) *resource.Quantity {
	var max *resource.Quantity

	for _, node := range nodes {
		if quant := getQuantity(node, property); quant != nil {
			if max == nil {
				max = quant
			} else if quant.Cmp(*max) == 1 {
				max = quant
			}
		}
	}

	return max
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
