package analyzer

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
)

func analyzeJobStatus(analyzer *troubleshootv1beta2.JobStatus, getFileContents func(string, []string) (map[string][]byte, error)) ([]*AnalyzeResult, error) {
	if analyzer.Name == "" {
		return analyzeAllJobStatuses(analyzer, getFileContents)
	} else {
		return analyzeOneJobStatus(analyzer, getFileContents)
	}
}

func analyzeOneJobStatus(analyzer *troubleshootv1beta2.JobStatus, getFileContents func(string, []string) (map[string][]byte, error)) ([]*AnalyzeResult, error) {
	excludeFiles := []string{}
	files, err := getFileContents(filepath.Join("cluster-resources", "jobs", fmt.Sprintf("%s.json", analyzer.Namespace)), excludeFiles)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read collected jobs from namespace")
	}

	var result *AnalyzeResult
	for _, collected := range files { // only 1 file here
		var jobs batchv1.JobList
		if err := json.Unmarshal(collected, &jobs); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal job list")
		}

		var job *batchv1.Job
		for _, j := range jobs.Items {
			if j.Name == analyzer.Name {
				job = j.DeepCopy()
				break
			}
		}

		if job == nil {
			// there's not an error, but maybe the requested deployment is not even deployed
			result = &AnalyzeResult{
				Title:   fmt.Sprintf("%s Job Status", analyzer.Name),
				IconKey: "kubernetes_deployment_status",                                                  // TODO: need new icon
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17", // TODO: need new icon
				IsFail:  true,
				Message: fmt.Sprintf("The job %q was not found", analyzer.Name),
			}
		} else if len(analyzer.Outcomes) > 0 {
			result, err = jobStatus(analyzer.Outcomes, job)
			if err != nil {
				return nil, errors.Wrap(err, "failed to process status")
			}
		} else {
			result = getDefaultJobResult(job)
		}
	}

	if result == nil {
		return nil, nil
	}

	return []*AnalyzeResult{result}, nil
}

func analyzeAllJobStatuses(analyzer *troubleshootv1beta2.JobStatus, getFileContents func(string, []string) (map[string][]byte, error)) ([]*AnalyzeResult, error) {
	fileNames := make([]string, 0)
	if analyzer.Namespace != "" {
		fileNames = append(fileNames, filepath.Join("cluster-resources", "jobs", fmt.Sprintf("%s.json", analyzer.Namespace)))
	}
	for _, ns := range analyzer.Namespaces {
		fileNames = append(fileNames, filepath.Join("cluster-resources", "jobs", fmt.Sprintf("%s.json", ns)))
	}

	// no namespace specified, so we need to analyze all jobs
	if len(analyzer.Namespaces) == 0 {
		fileNames = append(fileNames, filepath.Join("cluster-resources", "jobs", "*.json"))
	}

	excludeFiles := []string{}
	results := []*AnalyzeResult{}
	for _, fileName := range fileNames {
		files, err := getFileContents(fileName, excludeFiles)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read collected jobs from file")
		}

		for _, collected := range files {
			var jobs batchv1.JobList
			if err := json.Unmarshal(collected, &jobs); err != nil {
				return nil, errors.Wrap(err, "failed to unmarshal job list")
			}

			for _, job := range jobs.Items {
				result := getDefaultJobResult(&job)
				if result != nil {
					results = append(results, result)
				}
			}
		}
	}

	return results, nil
}

func jobStatus(outcomes []*troubleshootv1beta2.Outcome, job *batchv1.Job) (*AnalyzeResult, error) {
	result := &AnalyzeResult{
		Title:   fmt.Sprintf("%s Status", job.Name),
		IconKey: "kubernetes_deployment_status",                                                  // TODO: needs new icon
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17", // TODO: needs new icon
	}

	// ordering from the spec is important, the first one that matches returns
	for _, outcome := range outcomes {
		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}

			match, err := compareJobStatusToWhen(outcome.Fail.When, job)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse fail range")
			}

			if match {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}
		} else if outcome.Warn != nil {
			if outcome.Warn.When == "" {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return result, nil
			}

			match, err := compareJobStatusToWhen(outcome.Warn.When, job)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse warn range")
			}

			if match {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return result, nil
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return result, nil
			}

			match, err := compareJobStatusToWhen(outcome.Pass.When, job)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse pass range")
			}

			if match {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return result, nil
			}
		}
	}

	return result, nil
}

func getDefaultJobResult(job *batchv1.Job) *AnalyzeResult {
	if job.Spec.Completions == nil && job.Status.Succeeded > 1 {
		return nil
	}

	if job.Spec.Completions != nil && *job.Spec.Completions == job.Status.Succeeded {
		return nil
	}

	if job.Status.Failed == 0 {
		return nil
	}

	return &AnalyzeResult{
		Title:   fmt.Sprintf("%s/%s Job Status", job.Namespace, job.Name),
		IconKey: "kubernetes_deployment_status",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17",
		IsFail:  true,
		Message: fmt.Sprintf("The job %s/%s is not complete", job.Namespace, job.Name),
	}
}

func compareJobStatusToWhen(when string, job *batchv1.Job) (bool, error) {
	parts := strings.Split(strings.TrimSpace(when), " ")

	// we can make this a lot more flexible
	if len(parts) != 3 {
		return false, errors.Errorf("unable to parse when range: %s", when)
	}

	value, err := strconv.Atoi(parts[2])
	if err != nil {
		return false, errors.Wrapf(err, "failed to parse when value: %s", parts[2])
	}

	var actual int32
	switch parts[0] {
	case "succeeded":
		actual = job.Status.Succeeded
	case "failed":
		actual = job.Status.Failed
	default:
		return false, errors.Errorf("unknown when value: %s", parts[0])
	}

	switch parts[1] {
	case "=":
		fallthrough
	case "==":
		fallthrough
	case "===":
		return actual == int32(value), nil

	case "<":
		return actual < int32(value), nil

	case ">":
		return actual > int32(value), nil

	case "<=":
		return actual <= int32(value), nil

	case ">=":
		return actual >= int32(value), nil
	}

	return false, errors.Errorf("unknown comparator: %q", parts[1])
}
