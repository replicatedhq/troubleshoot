package analyzer

import (
	"encoding/json"
	"fmt"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

func analyzeSecret(analyzer *troubleshootv1beta2.AnalyzeSecret, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	filename := collect.GetSecretFileName(
		&troubleshootv1beta2.Secret{
			Namespace: analyzer.Namespace,
			Name:      analyzer.SecretName,
			Key:       analyzer.Key,
		},
		analyzer.SecretName,
	)

	secretData, err := getCollectedFileContents(filename)
	if err != nil {
		return nil, err
	}

	var foundSecret collect.SecretOutput
	if err := json.Unmarshal(secretData, &foundSecret); err != nil {
		return nil, err
	}

	title := analyzer.CheckName
	if title == "" {
		title = fmt.Sprintf("Secret %s", analyzer.SecretName)
	}

	result := AnalyzeResult{
		Title:   title,
		IconKey: "kubernetes_analyze_secret",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/secret.svg?w=13&h=16",
	}

	var failOutcome *troubleshootv1beta2.Outcome
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
