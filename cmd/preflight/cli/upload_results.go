package cli

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"
	analyzerunner "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/preflight"
)

func uploadResults(uri string, analyzeResults []*analyzerunner.AnalyzeResult) error {
	uploadPreflightResults := &preflight.UploadPreflightResults{
		Results: []*preflight.UploadPreflightResult{},
	}
	for _, analyzeResult := range analyzeResults {
		uploadPreflightResult := &preflight.UploadPreflightResult{
			IsFail:  analyzeResult.IsFail,
			IsWarn:  analyzeResult.IsWarn,
			IsPass:  analyzeResult.IsPass,
			Title:   analyzeResult.Title,
			Message: analyzeResult.Message,
			URI:     analyzeResult.URI,
		}

		uploadPreflightResults.Results = append(uploadPreflightResults.Results, uploadPreflightResult)
	}

	return upload(uri, uploadPreflightResults)
}

func uploadErrors(uri string, collectors collect.Collectors) error {
	errors := []*preflight.UploadPreflightError{}
	for _, collector := range collectors {
		for _, e := range collector.RBACErrors {
			errors = append(errors, &preflight.UploadPreflightError{
				Error: e.Error(),
			})
		}
	}

	results := &preflight.UploadPreflightResults{
		Errors: errors,
	}

	return upload(uri, results)
}

func upload(uri string, payload *preflight.UploadPreflightResults) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal payload")
	}

	req, err := http.NewRequest("POST", uri, bytes.NewBuffer(b))
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}

	req.Header.Set("Content-Type", "application/json")

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to execute request")
	}

	if resp.StatusCode > 290 {
		return errors.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
