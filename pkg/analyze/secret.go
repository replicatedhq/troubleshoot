package analyzer

import (
	"encoding/json"
	"fmt"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeSecret struct {
	analyzer *troubleshootv1beta2.AnalyzeSecret
}

func (a *AnalyzeSecret) Title() string {
	title := a.analyzer.CheckName
	if title == "" {
		title = fmt.Sprintf("Secret %s", a.analyzer.SecretName)
	}

	return title
}

func (a *AnalyzeSecret) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeSecret) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	result, err := a.analyzeSecret(a.analyzer, getFile)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	result.Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	return []*AnalyzeResult{result}, nil
}

func (a *AnalyzeSecret) analyzeSecret(analyzer *troubleshootv1beta2.AnalyzeSecret, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
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

	// The secret analyzer only supports fail (not found) and pass (found) outcomes
	// per https://troubleshoot.sh/docs/analyze/secrets. If the spec contains
	// neither, return nil and let the framework surface the missing-outcome error.
	var failOutcome, passOutcome *troubleshootv1beta2.SingleOutcome
	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			failOutcome = outcome.Fail
		} else if outcome.Pass != nil {
			passOutcome = outcome.Pass
		}
	}
	if failOutcome == nil && passOutcome == nil {
		return nil, nil
	}

	result := AnalyzeResult{
		Title:   a.Title(),
		IconKey: "kubernetes_analyze_secret",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/secret.svg?w=13&h=16",
		IsFail:  true,
	}
	if failOutcome != nil {
		result.Message = failOutcome.Message
		result.URI = failOutcome.URI
	}

	secretFound := foundSecret.SecretExists
	if secretFound && analyzer.Key != "" {
		secretFound = foundSecret.Key == analyzer.Key && foundSecret.KeyExists
	}
	if secretFound {
		result.IsFail = false
		result.IsPass = true
		if passOutcome != nil {
			result.Message = passOutcome.Message
			result.URI = passOutcome.URI
		}
	}

	if result.Message == "" {
		switch {
		case result.IsPass:
			result.Message = fmt.Sprintf("Secret %s was found in namespace %s", analyzer.SecretName, analyzer.Namespace)
		case analyzer.Key != "" && foundSecret.SecretExists:
			result.Message = fmt.Sprintf("Key %s was not found in secret %s/%s", analyzer.Key, analyzer.Namespace, analyzer.SecretName)
		default:
			result.Message = fmt.Sprintf("Secret %s was not found in namespace %s", analyzer.SecretName, analyzer.Namespace)
		}
	}

	return &result, nil
}
