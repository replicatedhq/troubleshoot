package analyzer

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
)

type AnalyzeImagePullSecret struct {
	analyzer *troubleshootv1beta2.ImagePullSecret
}

func (a *AnalyzeImagePullSecret) Title() string {
	title := a.analyzer.CheckName
	if title == "" {
		title = "Image Pull Secrets"
	}

	return title
}

func (a *AnalyzeImagePullSecret) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeImagePullSecret) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	result, err := a.analyzeImagePullSecret(a.analyzer, findFiles)
	if err != nil {
		return nil, err
	}
	result.Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	return []*AnalyzeResult{result}, nil
}

func (a *AnalyzeImagePullSecret) analyzeImagePullSecret(analyzer *troubleshootv1beta2.ImagePullSecret, getChildCollectedFileContents getChildCollectedFileContents) (*AnalyzeResult, error) {
	var excludeFiles = []string{}
	imagePullSecrets, err := getChildCollectedFileContents(fmt.Sprintf("%s/%s", constants.CLUSTER_RESOURCES_DIR, constants.CLUSTER_RESOURCES_IMAGE_PULL_SECRETS), excludeFiles)
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

	result := AnalyzeResult{
		Title:   a.Title(),
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
