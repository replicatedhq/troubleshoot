package analyzer

import (
	"encoding/json"
	"fmt"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

func analyzeSecret(analyzer *troubleshootv1beta1.AnalyzeSecret, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	secretData, err := getCollectedFileContents(fmt.Sprintf("secrets/%s/%s.json", analyzer.Namespace, analyzer.SecretName))
	if err != nil {
		return nil, err
	}

	var foundSecret collect.FoundSecret
	if err := json.Unmarshal(secretData, &foundSecret); err != nil {
		return nil, err
	}

	title := analyzer.CheckName
	if title == "" {
		title = fmt.Sprintf("Secret %s", analyzer.SecretName)
	}

	result := AnalyzeResult{
		Title: title,
	}

	var failOutcome *troubleshootv1beta1.Outcome
	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			failOutcome = outcome
		}
	}

	if !foundSecret.SecretExists {
		result.IsFail = true
		result.Message = failOutcome.Fail.Message
		result.URI = failOutcome.Fail.URI

		return &result, nil
	}

	if analyzer.Key != "" {
		if foundSecret.Key != analyzer.Key || !foundSecret.KeyExists {
			result.IsFail = true
			result.Message = failOutcome.Fail.Message
			result.URI = failOutcome.Fail.URI

			return &result, nil
		}
	}

	result.IsPass = true
	for _, outcome := range analyzer.Outcomes {
		if outcome.Pass != nil {
			result.Message = outcome.Pass.Message
			result.URI = outcome.Pass.URI
		}
	}

	return &result, nil
}
