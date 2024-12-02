package analyzer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

type AnalyzeClusterContainerStatuses struct {
	analyzer *troubleshootv1beta2.ClusterContainerStatuses
}

type podsWithContainers map[string]struct {
	name              string
	namespace         string
	containerStatuses []corev1.ContainerStatus
}

type matchedContainerInfo struct {
	Namespace     string
	PodName       string
	ContainerName string
	Ready         bool
	RestartCount  int32
	Message       string
}

func (a *AnalyzeClusterContainerStatuses) Title() string {
	if a.analyzer.CheckName != "" {
		return a.analyzer.CheckName
	}
	return "Cluster Container Status"
}

func (a *AnalyzeClusterContainerStatuses) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeClusterContainerStatuses) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	// get all pod list files from clusterResources collector directory
	excludeFiles := []string{}
	podListFiles, err := findFiles(filepath.Join(constants.CLUSTER_RESOURCES_DIR, constants.CLUSTER_RESOURCES_PODS, "*.json"), excludeFiles)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read collected pods")
	}

	// get pods matched analyzer filters
	pods, err := a.getPodsMatchingFilters(podListFiles)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pods matching filters")
	}

	results, err := a.analyzeContainerStatuses(pods)
	if err != nil {
		return nil, errors.Wrap(err, "failed to analyze container statuses")
	}

	return results, nil
}

func (a *AnalyzeClusterContainerStatuses) getPodsMatchingFilters(podListFiles map[string][]byte) (podsWithContainers, error) {
	var podsMatchedNamespace []corev1.Pod
	matchedPods := podsWithContainers{}

	// filter pods matched namespace selector
	for fileName, fileContent := range podListFiles {
		// pod list fileName is the namespace name, e.g. default.json
		currentNamespace := strings.TrimSuffix(filepath.Base(fileName), ".json")
		selectedNamespaces := a.analyzer.Namespaces
		if len(selectedNamespaces) > 0 {
			if !slices.Contains(selectedNamespaces, currentNamespace) {
				continue
			}
		}

		// filter pods by namespace
		var podList corev1.PodList
		if err := json.Unmarshal(fileContent, &podList); err != nil {
			var pods []corev1.Pod
			// fallback to old format
			if err := json.Unmarshal(fileContent, &pods); err != nil {
				return nil, errors.Wrapf(err, "failed to unmarshal pods list for namespace %s", currentNamespace)
			}
			podsMatchedNamespace = append(podsMatchedNamespace, pods...)
		} else {
			podsMatchedNamespace = append(podsMatchedNamespace, podList.Items...)
		}
	}

	// filter pods by container criteria
	for _, pod := range podsMatchedNamespace {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.RestartCount < a.analyzer.RestartCount {
				continue
			}
			// check if the pod has already been matched
			key := string(pod.UID)
			if _, ok := matchedPods[key]; !ok {
				matchedPods[key] = struct {
					name              string
					namespace         string
					containerStatuses []corev1.ContainerStatus
				}{
					name:              pod.Name,
					namespace:         pod.Namespace,
					containerStatuses: []corev1.ContainerStatus{containerStatus},
				}
				continue
			}
			entry := matchedPods[key]
			entry.containerStatuses = append(entry.containerStatuses, containerStatus)
		}
	}

	return matchedPods, nil
}

func (a *AnalyzeClusterContainerStatuses) analyzeContainerStatuses(podContainers podsWithContainers) ([]*AnalyzeResult, error) {
	results := []*AnalyzeResult{}

	// for each outcome, iterate over the pods and match the outcome against the container statues
	for _, outcome := range a.analyzer.Outcomes {
		r := AnalyzeResult{
			Title:   a.Title(),
			IconKey: "kubernetes_container_statuses",
			IconURI: "https://troubleshoot.sh/images/analyzer-icons/kubernetes.svg?w=16&h=16",
		}
		when := ""

		switch {
		case outcome.Fail != nil:
			r.IsFail = true
			r.Message = outcome.Fail.Message
			r.URI = outcome.Fail.URI
			when = outcome.Fail.When
		case outcome.Warn != nil:
			r.IsWarn = true
			r.Message = outcome.Warn.Message
			r.URI = outcome.Warn.URI
			when = outcome.Warn.When
		case outcome.Pass != nil:
			r.IsPass = true
			r.Message = outcome.Pass.Message
			r.URI = outcome.Pass.URI
			when = outcome.Pass.When
		default:
			klog.Warning("unexpected outcome in clusterContainerStatuses analyzer")
			continue
		}

		// empty when indicates final case, let's return the result
		if when == "" {
			return []*AnalyzeResult{&r}, nil
		}

		// continue matching with when condition
		reason, isEqualityOp, err := parseWhen(when)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse when")
		}

		for _, pod := range podContainers {
			matched, matchedContainerInfo := matchContainerReason(reason, pod.name, pod.namespace, pod.containerStatuses)
			if matched != isEqualityOp {
				continue
			}
			r.Message = renderContainerMessage(r.Message, &matchedContainerInfo)
			results = append(results, &r)
		}
	}
	return results, nil
}

// matchContainerReason iterates over the containerStatuses and returns true on the first reason that matches
func matchContainerReason(reason string, podName string, namespace string, containerStatuses []corev1.ContainerStatus) (bool, matchedContainerInfo) {
	var matched bool
	info := matchedContainerInfo{}
	info.Namespace = namespace
	info.PodName = podName

	for _, containerStatus := range containerStatuses {
		state := containerStatus.State
		info.ContainerName = containerStatus.Name
		info.Ready = containerStatus.Ready
		info.RestartCount = containerStatus.RestartCount

		switch {
		case containerStatus.LastTerminationState.Terminated != nil && strings.EqualFold(containerStatus.LastTerminationState.Terminated.Reason, reason):
			matched = true
			info.Message = containerStatus.LastTerminationState.Terminated.Message
		case state.Terminated != nil && strings.EqualFold(state.Terminated.Reason, reason):
			info.Message = state.Terminated.Message
			matched = true
		case state.Waiting != nil && strings.EqualFold(state.Waiting.Reason, reason):
			matched = true
			info.Message = state.Waiting.Message
		}
	}
	return matched, info
}

// parseWhen parses the when string into operator and reason
// return error if reason is not in the expected format
func parseWhen(when string) (string, bool, error) {
	parts := strings.Split(strings.TrimSpace(when), " ")
	if len(parts) != 2 {
		return "", false, errors.Errorf("expected 2 parts in when %q", when)
	}
	operator := parts[0]
	reason := parts[1]
	var isEqualityOp bool

	switch operator {
	case "=", "==", "===":
		isEqualityOp = true
	case "!=", "!==":
		isEqualityOp = false
	default:
		return "", false, errors.Errorf("unexpected operator %q in containerStatuses reason", operator)
	}

	return reason, isEqualityOp, nil
}

func renderContainerMessage(message string, info *matchedContainerInfo) string {
	if info == nil {
		return message
	}
	out := fmt.Sprintf("Container matched. Container: %s, Namespace: %s, Pod: %s", info.ContainerName, info.Namespace, info.PodName)

	tmpl := template.New("container")
	msgTmpl, err := tmpl.Parse(message)
	if err != nil {
		klog.V(2).Infof("failed to parse message template: %v", err)
		return out
	}

	var m bytes.Buffer
	err = msgTmpl.Execute(&m, info)
	if err != nil {
		klog.V(2).Infof("failed to render message template: %v", err)
		return out
	}

	return strings.TrimSpace(m.String())
}
