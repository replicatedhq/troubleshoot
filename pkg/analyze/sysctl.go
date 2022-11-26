package analyzer

import (
	"bufio"
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

// The when condition for outcomes in this analyzer is interpreted as "for some node".
// For example, "when: net.ipv4.ip_forward = 0" is true if at least one node has IP forwarding
// disabled.
func analyzeSysctl(analyzer *troubleshootv1beta2.SysctlAnalyze, findFiles func(string) (map[string][]byte, error)) (*AnalyzeResult, error) {
	files, err := findFiles("sysctl/*")
	if err != nil {
		return nil, errors.Wrap(err, "failed to find collected sysctl parameters")
	}
	if len(files) == 0 {
		return nil, errors.Wrap(err, "no sysctl parameters collected")
	}

	nodeParams := map[string]map[string]string{}

	for filename, parameters := range files {
		nodeName := filepath.Base(filename)
		nodeParams[nodeName] = parseSysctlParameters(parameters)
	}

	for _, outcome := range analyzer.Outcomes {
		result, err := evalSysctlOutcome(nodeParams, outcome)
		if err != nil {
			return nil, err
		}
		if result != nil {
			result.Title = analyzer.CheckName
			if result.Title == "" {
				result.Title = "Sysctl"
			}
			return result, nil
		}
	}

	return nil, nil
}

// Example: /proc/sys/net/ipv4/ip_forward = 1
var sysctlParamRX = regexp.MustCompile(`(^[^\s]+)\s=\s(.+)`)

func parseSysctlParameters(parameters []byte) map[string]string {
	buffer := bytes.NewBuffer(parameters)
	scanner := bufio.NewScanner(buffer)

	parsed := map[string]string{}

	for scanner.Scan() {
		matches := sysctlParamRX.FindStringSubmatch(scanner.Text())
		if len(matches) != 3 {
			continue
		}
		key := matches[1]
		value := matches[2]

		// "/proc/sys/net/ipv4/ip_forward" => "net.ipv4.ip_forward"
		key = strings.TrimPrefix(key, "/proc/sys/")
		parts := strings.Split(key, "/")
		key = strings.Join(parts, ".")

		parsed[key] = value
	}

	return parsed
}

func evalSysctlOutcome(nodeParams map[string]map[string]string, outcome *troubleshootv1beta2.Outcome) (*AnalyzeResult, error) {
	result := &AnalyzeResult{}

	var singleOutcome *troubleshootv1beta2.SingleOutcome

	if outcome.Pass != nil {
		singleOutcome = outcome.Pass
		result.IsPass = true
	}
	if outcome.Warn != nil {
		singleOutcome = outcome.Warn
		result.IsWarn = true
	}
	if outcome.Fail != nil {
		singleOutcome = outcome.Fail
		result.IsFail = true
	}

	if singleOutcome.When == "" {
		result.Message = singleOutcome.Message
		return result, nil
	}

	nodes, err := evalSysctlWhen(nodeParams, singleOutcome.When)
	if err != nil {
		return nil, err
	}

	// The when for this outcome is not true for any
	if len(nodes) == 0 {
		return nil, nil
	}

	// The when condition is true for at least one node
	if len(nodes) == 1 {
		result.Message = fmt.Sprintf("Node %s: %s", nodes[0], singleOutcome.Message)
	} else {
		result.Message = fmt.Sprintf("Nodes %s: %s", strings.Join(nodes, ", "), singleOutcome.Message)
	}

	result.URI = singleOutcome.URI

	return result, nil
}

// Example: net.ipv4.ip_forward = 0
var sysctlWhenRX = regexp.MustCompile(`([^\s]+)\s+([><=]=*)\s+(.+)`)

// Returns the list of node names the condition is true for. The condition is not considered true
// if the parameter is missing for the node.
func evalSysctlWhen(nodeParams map[string]map[string]string, when string) ([]string, error) {
	matches := sysctlWhenRX.FindStringSubmatch(when)
	if len(matches) != 4 {
		return nil, fmt.Errorf("Failed to parse when %q", when)
	}

	switch matches[2] {
	case "=", "==", "===":
		var nodes []string

		for nodeName, params := range nodeParams {
			nodeValue, ok := params[matches[1]]
			if !ok {
				continue
			}
			if nodeValue == strings.TrimSpace(matches[3]) {
				nodes = append(nodes, nodeName)
			}
		}

		sort.Strings(nodes)

		return nodes, nil

	case "<":
		var nodes []string

		for nodeName, params := range nodeParams {
			nodeValue, err := strconv.ParseInt(params[matches[1]], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("Failed to parse when %q", when)
			}

			expectedValue, err := strconv.ParseInt(strings.TrimSpace(matches[3]), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("Failed to parse when %q", when)
			}
			if nodeValue < expectedValue {
				nodes = append(nodes, nodeName)
			}
		}

		sort.Strings(nodes)

		return nodes, nil

	case "<=":
		var nodes []string

		for nodeName, params := range nodeParams {
			nodeValue, err := strconv.ParseInt(params[matches[1]], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("Failed to parse when %q", when)
			}

			expectedValue, err := strconv.ParseInt(strings.TrimSpace(matches[3]), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("Failed to parse when %q", when)
			}
			if nodeValue <= expectedValue {
				nodes = append(nodes, nodeName)
			}
		}

		sort.Strings(nodes)

		return nodes, nil

	case ">":
		var nodes []string

		for nodeName, params := range nodeParams {
			nodeValue, err := strconv.ParseInt(params[matches[1]], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("Failed to parse when %q", when)
			}

			expectedValue, err := strconv.ParseInt(strings.TrimSpace(matches[3]), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("Failed to parse when %q", when)
			}
			if nodeValue > expectedValue {
				nodes = append(nodes, nodeName)
			}
		}

		sort.Strings(nodes)

		return nodes, nil

	case ">=":
		var nodes []string

		for nodeName, params := range nodeParams {
			nodeValue, err := strconv.ParseInt(params[matches[1]], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("Failed to parse when %q", when)
			}

			expectedValue, err := strconv.ParseInt(strings.TrimSpace(matches[3]), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("Failed to parse when %q", when)
			}
			if nodeValue >= expectedValue {
				nodes = append(nodes, nodeName)
			}
		}

		sort.Strings(nodes)

		return nodes, nil

	default:
		return nil, fmt.Errorf("Unknown operator %q", matches[2])
	}
}
