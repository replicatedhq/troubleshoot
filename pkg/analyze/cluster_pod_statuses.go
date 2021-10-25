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
	corev1 "k8s.io/api/core/v1"
)

func clusterPodStatuses(analyzer *troubleshootv1beta2.ClusterPodStatuses, getChildCollectedFileContents func(string) (map[string][]byte, error)) ([]*AnalyzeResult, error) {
	collected, err := getChildCollectedFileContents(filepath.Join("cluster-resources", "pods", "*.json"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read collected pods")
	}

	fmt.Println("collected", len(collected))

	var pods []corev1.Pod
	for fileName, fileContent := range collected {
		podsNs := strings.TrimSuffix(fileName, ".json")
		include := len(analyzer.Namespaces) == 0
		for _, ns := range analyzer.Namespaces {
			if ns == podsNs {
				include = true
				break
			}
		}
		fmt.Println("include", include)
		if include {
			var nsPods []corev1.Pod
			if err := json.Unmarshal(fileContent, &nsPods); err != nil {
				return nil, errors.Wrapf(err, "failed to unmarshal pods list for namespace %s", podsNs)
			}
			pods = append(pods, nsPods...)
		}
	}

	allResults := []*AnalyzeResult{}

	for _, pod := range pods {
		podResults := []*AnalyzeResult{}

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
				// TODO log error
				continue
			}

			parts := strings.Split(strings.TrimSpace(when), " ")
			if len(parts) < 2 {
				// TODO log error
				continue
			}

			match := false
			fmt.Println("parts", parts[0], parts[1])
			fmt.Println("string(pod.Status.Phase)", string(pod.Status.Phase))
			switch parts[0] {
			case "=", "==", "===":
				match = parts[1] == string(pod.Status.Phase)
			case "!=", "!==":
				match = parts[1] != string(pod.Status.Phase)
			}

			fmt.Println("match", match)

			if !match {
				continue
			}

			r.Title = analyzer.CheckName
			if r.Title == "" {
				r.Title = "Pod {{ .Name }} status"
			}

			if r.Message == "" {
				r.Message = "Pod {{ .Name }} status is {{ .Status.Phase }}"
			}

			fmt.Println("r.Title", r.Title)
			fmt.Println("r.Message", r.Message)

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
			r.Message = m.String()
		}

		allResults = append(allResults, podResults...)
	}

	fmt.Println("allResults", allResults)

	return allResults, nil
}
