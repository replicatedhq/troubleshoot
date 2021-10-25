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
	collected, err := getChildCollectedFileContents(filepath.Join("cluster-resources", "pods"))
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

	results := []*AnalyzeResult{}

	for _, pod := range pods {
		whens := []string{}
		results := []*AnalyzeResult{}

		for _, outcome := range analyzer.Outcomes {
			r := AnalyzeResult{}
			if outcome.Fail != nil {
				r.IsFail = true
				r.Message = outcome.Fail.Message
				r.URI = outcome.Fail.URI
				results = append(results, &r)
				whens = append(whens, outcome.Fail.When)
			} else if outcome.Warn != nil {
				r.IsWarn = true
				r.Message = outcome.Warn.Message
				r.URI = outcome.Warn.URI
				results = append(results, &r)
				whens = append(whens, outcome.Warn.When)
			} else if outcome.Pass != nil {
				r.IsPass = true
				r.Message = outcome.Pass.Message
				r.URI = outcome.Pass.URI
				results = append(results, &r)
				whens = append(whens, outcome.Pass.When)
			}
		}

		fmt.Println("whens", whens)

		if len(results) == 0 {
			return nil, errors.New("empty outcomes")
		}

		for i, when := range whens {
			result := results[i]

			parts := strings.Split(strings.TrimSpace(when), " ")
			match := false

			switch parts[1] {
			case "=", "==", "===":
				match = parts[2] == string(pod.Status.Phase)
			case "!=", "!==":
				match = parts[2] != string(pod.Status.Phase)
			}

			fmt.Println("match", match)

			if !match {
				continue
			}

			result.Title = analyzer.CheckName
			if result.Title == "" {
				result.Title = "Pod {{ .Name }} status"
			}

			if result.Message == "" {
				result.Message = "Pod {{ .Name }} status is {{ .Status.Phase }}"
			}

			fmt.Println("result.Title", result.Title)
			fmt.Println("result.Message", result.Message)

			tmpl := template.New("pod")

			// template the title
			titleTmpl, err := tmpl.Parse(result.Title)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create new title template")
			}
			var t bytes.Buffer
			err = titleTmpl.Execute(&t, pod)
			if err != nil {
				return nil, errors.Wrap(err, "failed to execute template")
			}
			result.Title = t.String()

			// template the message
			msgTmpl, err := tmpl.Parse(result.Message)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create new title template")
			}
			var m bytes.Buffer
			err = msgTmpl.Execute(&m, pod)
			if err != nil {
				return nil, errors.Wrap(err, "failed to execute template")
			}
			result.Message = m.String()
		}
	}

	fmt.Println("results", results)

	return results, nil
}
