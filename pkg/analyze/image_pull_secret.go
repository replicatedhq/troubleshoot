package analyzer

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

func analyzeImagePullSecret(analyzer *troubleshootv1beta2.ImagePullSecret, getChildCollectedFileContents getChildCollectedFileContents) (*AnalyzeResult, error) {
	var excludeFiles = []string{}
	imagePullSecrets, err := getChildCollectedFileContents("cluster-resources/image-pull-secrets", excludeFiles)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get file contents for image pull secrets")
	}

	var failOutcome *troubleshootv1beta2.SingleOutcome
	var passOutcome *troubleshootv1beta2.SingleOutcome
	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			failOutcome = outcome.Fail
		} else if outcome.Pass != nil {
			passOutcome = outcome.Pass
		}
	}
	title := analyzer.CheckName
	if title == "" {
		title = "Image Pull Secrets"
	}

	result := AnalyzeResult{
		Title:   title,
		IconKey: "kubernetes_image_pull_secret",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/image-pull-secret.svg?w=16&h=14",
		IsFail:  true,
	}
	if failOutcome != nil {
		result.Message = failOutcome.Message
		result.URI = failOutcome.URI
	}

	for _, v := range imagePullSecrets {
		registryAndUsername := make(map[string]string)
		if err := json.Unmarshal(v, &registryAndUsername); err != nil {
			return nil, errors.Wrap(err, "failed to parse registry secret")
		}

		for registry, _ := range registryAndUsername {
			if registry == analyzer.RegistryName {
				result.IsPass = true
				result.IsFail = false
				if passOutcome != nil {
					result.Message = passOutcome.Message
					result.URI = passOutcome.URI
				}
			}
		}
	}

	if result.Message == "" {
		if result.IsPass {
			result.Message = fmt.Sprintf("Credentials to pull from: %s found", analyzer.RegistryName)
		} else {
			result.Message = fmt.Sprintf("Credentials to pull from: %s not found", analyzer.RegistryName)
		}
	}
	return &result, nil
}
