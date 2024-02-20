package analyzer

import (
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

type AnalyzeEvent struct {
	analyzer *troubleshootv1beta2.EventAnalyze
}

type eventFilter struct {
	kind   string
	reason string
	regex  string
}

func (a *AnalyzeEvent) Title() string {
	if a.analyzer.CheckName != "" {
		return a.analyzer.CheckName
	}
	if a.analyzer.CollectorName != "" {
		return a.analyzer.CollectorName
	}
	return "Event"
}

func (a *AnalyzeEvent) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeEvent) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	// read collected events based on namespace
	namespace := getNamespace(a.analyzer.Namespace)
	fullPath := path.Join(constants.CLUSTER_RESOURCES_DIR, constants.CLUSTER_RESOURCES_EVENTS, namespace)
	fullPath = fmt.Sprintf("%s.json", fullPath)
	fileContent, err := getFile(fullPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read collected events for namespace: %s", namespace)
	}

	events, err := convertToEventList(fileContent)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read collected events")
	}

	// filter if there's single event matched with the given criteria
	// match: Reason (optional) && Kind (optional) && regex (optional)
	// e.g. Reason: Unhealthy. Kind: Pod. Message: Readiness probe failed:...
	filter := eventFilter{
		kind:   a.analyzer.Kind,
		reason: a.analyzer.Reason,
		regex:  a.analyzer.RegexPattern,
	}

	if filter.reason == "" && filter.regex == "" {
		return nil, errors.New("Reason or regex pattern is required")
	}

	event := getEvent(events, filter)

	return analyzeEventResult(event, a.analyzer.Outcomes, a.Title())

}

func getNamespace(namespace string) string {
	if namespace == "" {
		return corev1.NamespaceDefault
	}
	return namespace
}

func convertToEventList(data []byte) (*corev1.EventList, error) {
	var eventList corev1.EventList
	err := json.Unmarshal(data, &eventList)
	if err != nil {
		return nil, fmt.Errorf("failed to convert []byte to corev1.EventList: %w", err)
	}
	return &eventList, nil
}

func getEvent(events *corev1.EventList, filter eventFilter) *corev1.Event {
	for _, event := range events.Items {
		if !matchReason(event.Reason, filter.reason) {
			continue
		}
		if !matchKind(event.InvolvedObject.Kind, filter.kind) {
			continue
		}
		toSearch := fmt.Sprintf("%s %s", event.Reason, event.Message)
		if matchRegex(toSearch, filter.regex) {
			klog.V(2).Infof("event matched: %v for reason: %s kind: %s regex: %s ", event, filter.reason, filter.kind, filter.regex)
			return &event
		}
	}
	return nil
}

func matchReason(actual, expected string) bool {
	// reason is optional
	if expected == "" {
		return true
	}
	return strings.EqualFold(actual, expected)
}

func matchKind(actual, expected string) bool {
	// kind is optional
	if expected == "" {
		return true
	}
	return strings.EqualFold(actual, expected)
}

func matchRegex(actual, expectedRegex string) bool {
	// regex is optional
	if expectedRegex == "" {
		return true
	}
	re, err := regexp.Compile(expectedRegex)
	if err != nil {
		klog.V(2).Infof("failed to read message regex: %v", err)
		return false
	}
	return re.MatchString(actual)
}

func analyzeEventResult(event *corev1.Event, outcomes []*troubleshootv1beta2.Outcome, checkName string) ([]*AnalyzeResult, error) {

	results := []*AnalyzeResult{}

	// for now, only support single outcome
	// we will return when there's a matched event
	willReturn := event != nil

	result := &AnalyzeResult{
		Title:   checkName,
		IconKey: "kubernetes_event",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/kubernetes.svg?w=16&h=16",
	}

	for _, o := range outcomes {
		if o.Fail != nil {
			toReturn, err := strconv.ParseBool(o.Fail.When)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse when condition: %s", o.Fail.When)
			}
			if toReturn == willReturn {
				result.IsFail = true
				result.Message = decorateMessage(o.Fail.Message, event)
				result.URI = o.Fail.URI
				break
			}
		}

		if o.Warn != nil {
			toReturn, err := strconv.ParseBool(o.Warn.When)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse when condition: %s", o.Warn.When)
			}
			if toReturn == willReturn {
				result.IsWarn = true
				result.Message = decorateMessage(o.Warn.Message, event)
				result.URI = o.Warn.URI
				break
			}
		}

		if o.Pass != nil {
			toReturn, err := strconv.ParseBool(o.Pass.When)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse when condition: %s", o.Pass.When)
			}
			if toReturn == willReturn {
				result.IsPass = true
				result.Message = decorateMessage(o.Pass.Message, event)
				result.URI = o.Pass.URI
				break
			}
		}

	}
	results = append(results, result)
	return results, nil
}

func decorateMessage(message string, event *corev1.Event) string {
	if event == nil {
		return message
	}
	out := fmt.Sprintf("%s. Name: %s Message: %s", message, event.InvolvedObject.Name, event.Message)
	return out
}
