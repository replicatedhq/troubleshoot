package analyzer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

type AnalyzeClusterPodStatuses struct {
	analyzer *troubleshootv1beta2.ClusterPodStatuses
}

func (a *AnalyzeClusterPodStatuses) Title() string {
	if a.analyzer.CheckName != "" {
		return a.analyzer.CheckName
	}

	return "Cluster Pod Status"
}

func (a *AnalyzeClusterPodStatuses) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeClusterPodStatuses) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	results, err := clusterPodStatuses(a.analyzer, findFiles)
	if err != nil {
		return nil, err
	}
	for i := range results {
		results[i].Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	}
	return results, nil
}

func clusterPodStatuses(analyzer *troubleshootv1beta2.ClusterPodStatuses, getChildCollectedFileContents getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	excludeFiles := []string{}
	collected, err := getChildCollectedFileContents(filepath.Join(constants.CLUSTER_RESOURCES_DIR, constants.CLUSTER_RESOURCES_PODS, "*.json"), excludeFiles)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read collected pods")
	}

	var pods []corev1.Pod
	for fileName, fileContent := range collected {
		podsNs := strings.TrimSuffix(filepath.Base(fileName), ".json")
		include := len(analyzer.Namespaces) == 0
		for _, ns := range analyzer.Namespaces {
			if ns == podsNs {
				include = true
				break
			}
		}
		if include {
			var nsPods corev1.PodList
			if err := json.Unmarshal(fileContent, &nsPods); err != nil {
				var nsPodsArr []corev1.Pod
				if err := json.Unmarshal(fileContent, &nsPodsArr); err != nil {
					return nil, errors.Wrapf(err, "failed to unmarshal pods list for namespace %s", podsNs)
				}
				pods = append(pods, nsPodsArr...)
			} else {
				pods = append(pods, nsPods.Items...)
			}
		}
	}

	allResults := []*AnalyzeResult{}

	for _, pod := range pods {
		if pod.Status.Reason == "" {
			pod.Status.Reason, pod.Status.Message = k8sutil.GetPodStatusReason(&pod)
		}

		if pod.Status.Message == "" {
			messages := []string{}
			collectedEvents, err := getChildCollectedFileContents(filepath.Join(constants.CLUSTER_RESOURCES_DIR, "events", fmt.Sprintf("%s.json", pod.Namespace)), excludeFiles)
			if err != nil {
				return nil, errors.Wrap(err, "failed to read collected events")
			}

			for _, fileContent := range collectedEvents {
				var nsEvents []corev1.Event
				if err := json.Unmarshal(fileContent, &nsEvents); err != nil {
					// try new format
					var nsEventsList corev1.EventList
					if err := json.Unmarshal(fileContent, &nsEventsList); err != nil {
						return nil, errors.Wrap(err, "failed to unmarshal events")
					}
					nsEvents = nsEventsList.Items
				}

				for _, event := range nsEvents {
					if event.InvolvedObject.Kind == "Pod" && event.InvolvedObject.Name == pod.Name && event.InvolvedObject.Namespace == pod.Namespace {
						if event.Type == "Warning" && event.Message != "" {
							messages = append(messages, event.Message)
						}
					}
				}
			}
			pod.Status.Message = strings.Join(messages, ". ")
		}

		for _, outcome := range analyzer.Outcomes {
			r := AnalyzeResult{}
			when := ""

			if outcome.Fail != nil {
				r.IsFail = true
				r.Message = outcome.Fail.Message
				r.URI = outcome.Fail.URI
				when = outcome.Fail.When
			} else if outcome.Warn != nil {
				r.IsWarn = true
				r.Message = outcome.Warn.Message
				r.URI = outcome.Warn.URI
				when = outcome.Warn.When
			} else if outcome.Pass != nil {
				r.IsPass = true
				r.Message = outcome.Pass.Message
				r.URI = outcome.Pass.URI
				when = outcome.Pass.When
			} else {
				klog.Error("error: found an empty outcome in a clusterPodStatuses analyzer\n")
				continue
			}

			operator := ""
			reason := ""
			match := false
			if when != "" {
				parts := strings.Split(strings.TrimSpace(when), " ")
				if len(parts) < 2 {
					klog.Errorf("invalid 'when' format: %s\n", when)
					continue
				}
				operator = parts[0]
				reason = parts[1]

				switch operator {
				case "=", "==", "===":
					if reason == "Healthy" {
						match = !k8sutil.IsPodUnhealthy(&pod)
					} else {
						match = reason == string(pod.Status.Phase) || reason == string(pod.Status.Reason)
					}
				case "!=", "!==":
					if reason == "Healthy" {
						match = k8sutil.IsPodUnhealthy(&pod)
					} else {
						match = reason != string(pod.Status.Phase) && reason != string(pod.Status.Reason)
					}
				}

				if !match {
					continue
				}
			}

			r.InvolvedObject = &corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Namespace:  pod.Namespace,
				Name:       pod.Name,
			}

			r.Title = analyzer.CheckName
			if r.Title == "" {
				r.Title = "Pod {{ .Namespace }}/{{ .Name }} status"
			}

			if r.Message == "" {
				r.Message = "Pod {{ .Namespace }}/{{ .Name }} status is {{ .Status.Reason }}. {{ .Status.Message }}"
			}

			tmpl := template.New("pod")

			// template the title
			titleTmpl, err := tmpl.Parse(r.Title)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create new title template")
			}
			var t bytes.Buffer
			err = titleTmpl.Execute(&t, pod)
			if err != nil {
				return nil, errors.Wrap(err, "failed to execute template")
			}
			r.Title = t.String()

			// template the message
			msgTmpl, err := tmpl.Parse(r.Message)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create new title template")
			}
			var m bytes.Buffer
			err = msgTmpl.Execute(&m, pod)
			if err != nil {
				return nil, errors.Wrap(err, "failed to execute template")
			}
			r.Message = strings.TrimSpace(m.String())

			// add to results, break and check the next pod
			allResults = append(allResults, &r)
			break
		}
	}

	return allResults, nil
}
