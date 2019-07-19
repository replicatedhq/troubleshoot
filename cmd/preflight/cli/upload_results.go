package cli

import (
	"bytes"
	"encoding/json"
	"net/http"

	analyzerunner "github.com/replicatedhq/troubleshoot/pkg/analyze"
)

type UploadPreflightResult struct {
	IsFail bool `json:"isFail,omitempty"`
	IsWarn bool `json:"isWarn,omitempty"`
	IsPass bool `json:"isPass,omitempty"`

	Title   string `json:"title"`
	Message string `json:"message"`
	URI     string `json:"uri,omitempty"`
}

type UploadPreflightResults struct {
	Results []*UploadPreflightResult `json:"results"`
}

func tryUploadResults(uri string, preflightName string, analyzeResults []*analyzerunner.AnalyzeResult) error {
	uploadPreflightResults := UploadPreflightResults{
		Results: []*UploadPreflightResult{},
	}
	for _, analyzeResult := range analyzeResults {
		uploadPreflightResult := &UploadPreflightResult{
			IsFail:  analyzeResult.IsFail,
			IsWarn:  analyzeResult.IsWarn,
			IsPass:  analyzeResult.IsPass,
			Title:   analyzeResult.Title,
			Message: analyzeResult.Message,
			URI:     analyzeResult.URI,
		}

		uploadPreflightResults.Results = append(uploadPreflightResults.Results, uploadPreflightResult)
	}

	b, err := json.Marshal(uploadPreflightResults)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", uri, bytes.NewBuffer(b))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := http.DefaultClient
	_, err = client.Do(req)
	if err != nil {
		return err
	}

	return nil
}
