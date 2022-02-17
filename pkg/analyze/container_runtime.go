package analyzer

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	corev1 "k8s.io/api/core/v1"
)

func analyzeContainerRuntime(analyzer *troubleshootv1beta2.ContainerRuntime, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	collected, err := getCollectedFileContents("cluster-resources/nodes.json")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get contents of nodes.json")
	}

	var nodes corev1.NodeList
	if err := json.Unmarshal(collected, &nodes); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal node list")
	}

	foundRuntimes := []string{}
	for _, node := range nodes.Items {
		foundRuntimes = append(foundRuntimes, node.Status.NodeInfo.ContainerRuntimeVersion)
	}

	title := analyzer.CheckName
	if title == "" {
		title = "Container Runtime"
	}
	result := &AnalyzeResult{
		Title:   title,
		IconKey: "kubernetes_container_runtime",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/container-runtime.svg?w=23&h=16",
	}

	// ordering is important for passthrough
	for _, outcome := range analyzer.Outcomes {
		if outcome.Fatal != nil {
			if outcome.Fatal.When == "" {
				result.IsFatal = true
				result.Message = outcome.Fatal.Message
				result.URI = outcome.Fatal.URI

				return result, nil
			}

			for _, foundRuntime := range foundRuntimes {
				isMatch, err := compareRuntimeConditionalToActual(outcome.Fatal.When, foundRuntime)
				if err != nil {
					return nil, errors.Wrap(err, "failed to compare runtime conditional")
				}

				if isMatch {
					result.IsFatal = true
					result.Message = outcome.Fatal.Message
					result.URI = outcome.Fatal.URI

					return result, nil
				}
			}
		} else if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}

			for _, foundRuntime := range foundRuntimes {
				isMatch, err := compareRuntimeConditionalToActual(outcome.Fail.When, foundRuntime)
				if err != nil {
					return nil, errors.Wrap(err, "failed to compare runtime conditional")
				}

				if isMatch {
					result.IsFail = true
					result.Message = outcome.Fail.Message
					result.URI = outcome.Fail.URI

					return result, nil
				}
			}
		} else if outcome.Warn != nil {
			if outcome.Warn.When == "" {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return result, nil
			}

			for _, foundRuntime := range foundRuntimes {
				isMatch, err := compareRuntimeConditionalToActual(outcome.Warn.When, foundRuntime)
				if err != nil {
					return nil, errors.Wrap(err, "failed to compare runtime conditional")
				}

				if isMatch {
					result.IsWarn = true
					result.Message = outcome.Warn.Message
					result.URI = outcome.Warn.URI

					return result, nil
				}
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return result, nil
			}

			for _, foundRuntime := range foundRuntimes {
				isMatch, err := compareRuntimeConditionalToActual(outcome.Pass.When, foundRuntime)
				if err != nil {
					return nil, errors.Wrap(err, "failed to compare runtime conditional")
				}

				if isMatch {
					result.IsPass = true
					result.Message = outcome.Pass.Message
					result.URI = outcome.Pass.URI

					return result, nil
				}
			}
		}
	}

	return result, nil
}

func compareRuntimeConditionalToActual(conditional string, actual string) (bool, error) {
	parts := strings.Split(strings.TrimSpace(conditional), " ")

	// we can make this a lot more flexible
	if len(parts) != 2 {
		return false, errors.New("unable to parse conditional")
	}

	parsedRuntime, err := url.Parse(actual)
	if err != nil {
		return false, errors.Wrap(err, "failed to parse url")
	}

	switch parts[0] {
	case "=":
		fallthrough
	case "==":
		fallthrough
	case "===":
		return parts[1] == parsedRuntime.Scheme, nil
	}
	return false, nil
}
