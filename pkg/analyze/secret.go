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
	// Capture fail and pass independently: a single outcome object may set both,
	// so an else-if here would silently drop the second one.
	var failOutcome, passOutcome *troubleshootv1beta2.SingleOutcome
	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			failOutcome = outcome.Fail
		}
		if outcome.Pass != nil {
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
	}

	secretFound := foundSecret.SecretExists
	if secretFound && analyzer.Key != "" {
		secretFound = foundSecret.Key == analyzer.Key && foundSecret.KeyExists
	}
	// Track whether the matched branch had a configured outcome, so we only fall
	// back to a default message when none was supplied — a configured outcome with
	// an intentionally empty message (e.g. URI-only) must be preserved verbatim.
	outcomeConfigured := false
	if secretFound {
		result.IsPass = true
		if passOutcome != nil {
			result.Message = passOutcome.Message
			result.URI = passOutcome.URI
			outcomeConfigured = true
		}
	} else {
		result.IsFail = true
		if failOutcome != nil {
			result.Message = failOutcome.Message
			result.URI = failOutcome.URI
			outcomeConfigured = true
		}
	}

	if !outcomeConfigured {
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
