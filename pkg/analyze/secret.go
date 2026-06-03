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

	result := AnalyzeResult{
		Title:   a.Title(),
		IconKey: "kubernetes_analyze_secret",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/secret.svg?w=13&h=16",
	}

	var failOutcome, warnOutcome, notFoundWarnOutcome *troubleshootv1beta2.SingleOutcome
	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			failOutcome = outcome.Fail
		}
		if outcome.Warn != nil {
			warnOutcome = outcome.Warn
			if outcome.Warn.When == "notFound" {
				notFoundWarnOutcome = outcome.Warn
			}
		}
	}

	applyNotFound := func(defaultMessage string) {
		switch {
		case notFoundWarnOutcome != nil:
			result.IsWarn = true
			result.Message = notFoundWarnOutcome.Message
			result.URI = notFoundWarnOutcome.URI
		case failOutcome != nil:
			result.IsFail = true
			result.Message = failOutcome.Message
			result.URI = failOutcome.URI
		case warnOutcome != nil:
			result.IsWarn = true
			result.Message = warnOutcome.Message
			result.URI = warnOutcome.URI
		default:
			result.IsWarn = true
			result.Message = defaultMessage
		}
	}

	if !foundSecret.SecretExists {
		applyNotFound(fmt.Sprintf("Secret %s was not found in namespace %s", analyzer.SecretName, analyzer.Namespace))
		return &result, nil
	}

	if analyzer.Key != "" {
		if foundSecret.Key != analyzer.Key || !foundSecret.KeyExists {
			applyNotFound(fmt.Sprintf("Key %s was not found in secret %s/%s", analyzer.Key, analyzer.Namespace, analyzer.SecretName))
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
