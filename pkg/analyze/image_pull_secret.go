package analyzer

import (
	"encoding/json"

	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
)

func analyzeImagePullSecret(analyzer *troubleshootv1beta1.ImagePullSecret, getChildCollectedFileContents func(string) (map[string][]byte, error)) (*AnalyzeResult, error) {
	imagePullSecrets, err := getChildCollectedFileContents("cluster-resources/image-pull-secrets")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get file contents for image pull secrets")
	}

	var failOutcome *troubleshootv1beta1.Outcome
	var passOutcome *troubleshootv1beta1.Outcome
	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			failOutcome = outcome
		} else if outcome.Pass != nil {
			passOutcome = outcome
		}
	}

	result := AnalyzeResult{
		Title:   analyzer.CheckName,
		IconKey: "kubernetes_image_pull_secret",
		IsFail:  true,
		Message: failOutcome.Fail.Message,
		URI:     failOutcome.Fail.URI,
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
				result.Message = passOutcome.Pass.Message
				result.URI = passOutcome.Pass.URI
			}
		}
	}

	return &result, nil
}
