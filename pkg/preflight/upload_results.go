package preflight

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"
	analyzerunner "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

func uploadResults(uri string, analyzeResults []*analyzerunner.AnalyzeResult) error {
	uploadPreflightResults := &UploadPreflightResults{
		Results: []*UploadPreflightResult{},
	}
	for _, analyzeResult := range analyzeResults {
		uploadPreflightResult := &UploadPreflightResult{
			Strict:  analyzeResult.Strict,
			IsFail:  analyzeResult.IsFail,
			IsWarn:  analyzeResult.IsWarn,
			IsPass:  analyzeResult.IsPass,
			Title:   analyzeResult.Title,
			Message: analyzeResult.Message,
			URI:     analyzeResult.URI,
			Note:    analyzeResult.Note,
		}

		uploadPreflightResults.Results = append(uploadPreflightResults.Results, uploadPreflightResult)
	}

	return upload(uri, uploadPreflightResults)
}

func uploadErrors(uri string, collectors []collect.Collector) error {
	errors := []*UploadPreflightError{}
	for _, collector := range collectors {
		for _, e := range collector.GetRBACErrors() {
			errors = append(errors, &UploadPreflightError{
				Error: e.Error(),
			})
		}
	}

	results := &UploadPreflightResults{
		Errors: errors,
	}

	return upload(uri, results)
}

func upload(uri string, payload *UploadPreflightResults) error {
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
