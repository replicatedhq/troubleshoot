package analyzer

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/pkg/util/taints"

	"github.com/replicatedhq/troubleshoot/internal/util"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
)

type AnalyzeNodeResources struct {
	analyzer *troubleshootv1beta2.NodeResources
}

type NodeResourceMsg struct {
	*troubleshootv1beta2.NodeResourceFilters
	NodeCount int
}

func (a *AnalyzeNodeResources) Title() string {
	title := a.analyzer.CheckName
	if title == "" {
		title = "Node Resources"
	}
	return title
}

func (a *AnalyzeNodeResources) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeNodeResources) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	result, err := a.analyzeNodeResources(a.analyzer, getFile)
	if err != nil {
		return nil, err
	}
	result.Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	return []*AnalyzeResult{result}, nil
}

func (a *AnalyzeNodeResources) analyzeNodeResources(analyzer *troubleshootv1beta2.NodeResources, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {

	collected, err := getCollectedFileContents(fmt.Sprintf("%s/%s.json", constants.CLUSTER_RESOURCES_DIR, constants.CLUSTER_RESOURCES_NODES))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get contents of nodes.json")
	}

	var nodes corev1.NodeList
	if err := json.Unmarshal(collected, &nodes); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal node list")
	}

	matchingNodes := []corev1.Node{}

	for _, node := range nodes.Items {
		isMatch, err := nodeMatchesFilters(node, analyzer.Filters)
		if err != nil {
			return nil, errors.Wrap(err, "failed to check if node matches filter")
		}

		if isMatch {
			matchingNodes = append(matchingNodes, node)
		}
	}

	result := &AnalyzeResult{
		Title:   a.Title(),
		IconKey: "kubernetes_node_resources",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/node-resources.svg?w=16&h=18",
	}

	nodeMsg := NodeResourceMsg{
		analyzer.Filters, len(matchingNodes),
	}

	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			isWhenMatch, err := compareNodeResourceConditionalToActual(outcome.Fail.When, matchingNodes, analyzer.Filters)

			if err != nil {
				return nil, errors.Wrap(err, "failed to parse when")
			}

			if isWhenMatch {
				result.IsFail = true
				result.Message, err = util.RenderTemplate(outcome.Fail.Message, nodeMsg)
				if err != nil {
					return nil, errors.Wrap(err, "failed to render message template")
				}
				result.URI = outcome.Fail.URI
				return result, nil
			}
		} else if outcome.Warn != nil {
			isWhenMatch, err := compareNodeResourceConditionalToActual(outcome.Warn.When, matchingNodes, analyzer.Filters)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse when")
			}

			if isWhenMatch {
				result.IsWarn = true
				result.Message, err = util.RenderTemplate(outcome.Warn.Message, nodeMsg)
				if err != nil {
					return nil, errors.Wrap(err, "failed to render message template")
				}
				result.URI = outcome.Warn.URI

				return result, nil
			}
		} else if outcome.Pass != nil {
			isWhenMatch, err := compareNodeResourceConditionalToActual(outcome.Pass.When, matchingNodes, analyzer.Filters)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse when")
			}

			if isWhenMatch {
				result.IsPass = true
				result.Message, err = util.RenderTemplate(outcome.Pass.Message, nodeMsg)
				if err != nil {
					return nil, errors.Wrap(err, "failed to render message template")
				}
				result.URI = outcome.Pass.URI

				return result, nil
			}
		}
	}

	return result, nil
}

func compareNodeResourceConditionalToActual(conditional string, matchingNodes []corev1.Node, filters *troubleshootv1beta2.NodeResourceFilters) (res bool, err error) {
	res = false
	err = nil

	defer func() {
		if r := recover(); r != nil {
			err = errors.Errorf("failed to evaluate %q: %v", conditional, r)
		}
	}()

	if conditional == "" {
		res = true
		return
	}

	parts := strings.Fields(strings.TrimSpace(conditional))

	if len(parts) == 2 {
		parts = append([]string{"count"}, parts...)
	}

	if len(parts) != 3 {
		err = errors.New("unable to parse nodeResources conditional")
		return
	}

	operator := parts[1]

	var desiredValue interface{}
	desiredValue = parts[2]

	parsedDesiredValue, err := strconv.Atoi(parts[2])
	if err == nil {
		desiredValue = parsedDesiredValue
	} else {
		err = nil // try parsing as a resource
	}

	reg := regexp.MustCompile(`(?P<function>.*)\((?P<property>.*)\)`)
	match := reg.FindStringSubmatch(parts[0])

	if match == nil {
		// We support this as equivalent to the count() function
		match = reg.FindStringSubmatch(fmt.Sprintf("count() == %s", parts[0]))
	}

	if match == nil || len(match) != 3 {
		err = errors.New("conditional does not match pattern of function(property?)")
		return
	}

	function := match[1]
	property := match[2]
	resourceName := ""

	if filters != nil {
		resourceName = filters.ResourceName
	}

	var actualValue interface{}

	switch function {
	case "count":
		actualValue = len(matchingNodes)
	case "min":
		actualValue = findMin(matchingNodes, property, resourceName)
	case "max":
		actualValue = findMax(matchingNodes, property, resourceName)
	case "sum":
		actualValue = findSum(matchingNodes, property, resourceName)
	case "nodeCondition":
		operatorChecker := regexp.MustCompile(`={1,3}`)
		if !operatorChecker.MatchString(operator) {
			err = errors.New("nodeCondition function can only be compared using equals expression.")
			return
		}
		if match[2] == "" {
			err = errors.New("value function parameter missing.")
			return
		}

		for _, node := range matchingNodes {
			for _, condition := range node.Status.Conditions {
				if string(condition.Type) == match[2] && string(condition.Status) == desiredValue {
					res = true
					return
				}
			}
		}
		res = false
		return
	}

	switch operator {
	case "=", "==", "===":
		if _, ok := actualValue.(int); ok {
			if _, ok := desiredValue.(int); ok {
				res = actualValue.(int) == desiredValue.(int)
				return
			}
		}

		if _, ok := desiredValue.(string); ok {
			res = actualValue.(*resource.Quantity).Cmp(resource.MustParse(desiredValue.(string))) == 0
			return
		}

		res = actualValue.(*resource.Quantity).Cmp(resource.MustParse(strconv.Itoa(desiredValue.(int)))) == 0
		return

	case "<":
		if _, ok := actualValue.(int); ok {
			if _, ok := desiredValue.(int); ok {
				res = actualValue.(int) < desiredValue.(int)
				return
			}
		}
		if _, ok := desiredValue.(string); ok {
			res = actualValue.(*resource.Quantity).Cmp(resource.MustParse(desiredValue.(string))) == -1
			return
		}

		res = actualValue.(*resource.Quantity).Cmp(resource.MustParse(strconv.Itoa(desiredValue.(int)))) == -1
		return

	case ">":
		if _, ok := actualValue.(int); ok {
			if _, ok := desiredValue.(int); ok {
				res = actualValue.(int) > desiredValue.(int)
				return
			}
		}
		if _, ok := desiredValue.(string); ok {
			res = actualValue.(*resource.Quantity).Cmp(resource.MustParse(desiredValue.(string))) == 1
			return
		}

		res = actualValue.(*resource.Quantity).Cmp(resource.MustParse(strconv.Itoa(desiredValue.(int)))) == 1
		return

	case "<=":
		if _, ok := actualValue.(int); ok {
			if _, ok := desiredValue.(int); ok {
				res = actualValue.(int) <= desiredValue.(int)
				return
			}
		}
		if _, ok := desiredValue.(string); ok {
			res = actualValue.(*resource.Quantity).Cmp(resource.MustParse(desiredValue.(string))) == 0 ||
				actualValue.(*resource.Quantity).Cmp(resource.MustParse(desiredValue.(string))) == -1
			return
		}

		res = actualValue.(*resource.Quantity).Cmp(resource.MustParse(strconv.Itoa(desiredValue.(int)))) == 0 ||
			actualValue.(*resource.Quantity).Cmp(resource.MustParse(strconv.Itoa(desiredValue.(int)))) == -1
		return

	case ">=":
		if _, ok := actualValue.(int); ok {
			if _, ok := desiredValue.(int); ok {
				res = actualValue.(int) >= desiredValue.(int)
				return
			}
		}
		if _, ok := desiredValue.(string); ok {
			res = actualValue.(*resource.Quantity).Cmp(resource.MustParse(desiredValue.(string))) == 0 ||
				actualValue.(*resource.Quantity).Cmp(resource.MustParse(desiredValue.(string))) == 1
			return
		}

		res = actualValue.(*resource.Quantity).Cmp(resource.MustParse(strconv.Itoa(desiredValue.(int)))) == 0 ||
			actualValue.(*resource.Quantity).Cmp(resource.MustParse(strconv.Itoa(desiredValue.(int)))) == 1
		return
	}

	err = errors.New("unexpected conditional in nodeResources")
	return
}

func getQuantity(node corev1.Node, property string, resourceName string) *resource.Quantity {
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
	case "resourceCapacity":
		capacity, ok := node.Status.Capacity[corev1.ResourceName(resourceName)]
		if !ok {
			return nil
		}
		return &capacity
	case "resourceAllocatable":
		allocatable, ok := node.Status.Allocatable[corev1.ResourceName(resourceName)]
		if !ok {
			return nil
		}
		return &allocatable
	}
	return nil
}

func findSum(nodes []corev1.Node, property string, resourceName string) *resource.Quantity {
	sum := resource.Quantity{}

	for _, node := range nodes {
		if quant := getQuantity(node, property, resourceName); quant != nil {
			sum.Add(*quant)
		}
	}

	return &sum
}

func findMin(nodes []corev1.Node, property string, resourceName string) *resource.Quantity {
	var min *resource.Quantity

	for _, node := range nodes {
		if quant := getQuantity(node, property, resourceName); quant != nil {
			if min == nil {
				min = quant
			} else if quant.Cmp(*min) == -1 {
				min = quant
			}
		}
	}

	return min
}

func findMax(nodes []corev1.Node, property string, resourceName string) *resource.Quantity {
	var max *resource.Quantity

	for _, node := range nodes {
		if quant := getQuantity(node, property, resourceName); quant != nil {
			if max == nil {
				max = quant
			} else if quant.Cmp(*max) == 1 {
				max = quant
			}
		}
	}

	return max
}

func nodeMatchesFilters(node corev1.Node, filters *troubleshootv1beta2.NodeResourceFilters) (bool, error) {
	if filters == nil {
		return true, nil
	}

	if filters.ResourceName != "" {
		capacity, capacityExists := node.Status.Capacity[corev1.ResourceName(filters.ResourceName)]
		allocatable, allocatableExists := node.Status.Allocatable[corev1.ResourceName(filters.ResourceName)]

		if !capacityExists && !allocatableExists {
			return false, nil
		}

		if filters.ResourceCapacity != "" {
			parsed, err := resource.ParseQuantity(filters.ResourceCapacity)
			if err != nil {
				return false, errors.Wrap(err, "failed to parse resource capacity")
			}

			// Compare the capacity value with the parsed value
			if capacity.Cmp(parsed) == -1 {
				return false, nil
			}
		}

		if filters.ResourceAllocatable != "" {
			parsed, err := resource.ParseQuantity(filters.ResourceAllocatable)
			if err != nil {
				return false, errors.Wrap(err, "failed to parse resource allocatable")
			}

			// Compare the allocatable value with the parsed value
			if allocatable.Cmp(parsed) == -1 {
				return false, nil
			}
		}
	}

	// all filters must pass for this to pass
	if filters.Selector != nil {
		selector, err := metav1.LabelSelectorAsSelector(
			&metav1.LabelSelector{
				MatchLabels:      filters.Selector.MatchLabel,
				MatchExpressions: filters.Selector.MatchExpressions,
			},
		)
		if err != nil {
			return false, errors.Wrap(err, "failed to create label selector")
		}

		found := selector.Matches(labels.Set(node.Labels))
		if !found {
			return false, nil
		}
	}

	if filters.Taint != nil {
		return taints.TaintExists(node.Spec.Taints, filters.Taint), nil
	}

	if filters.CPUArchitecture != "" {
		parsed := filters.CPUArchitecture

		if !strings.EqualFold(node.Status.NodeInfo.Architecture, parsed) {
			return false, nil
		}
	}

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
