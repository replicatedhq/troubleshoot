package analyzer

import (
	"encoding/json"
	"fmt"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
)

type AnalyzeStorageClass struct {
	analyzer *troubleshootv1beta2.StorageClass
}

func (a *AnalyzeStorageClass) Title() string {
	title := a.analyzer.CheckName
	if title == "" {
		if a.analyzer.StorageClassName != "" {
			title = fmt.Sprintf("Storage class %s", a.analyzer.StorageClassName)
		} else {
			title = "Default Storage Class"
		}
	}

	return title
}

func (a *AnalyzeStorageClass) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeStorageClass) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	result, err := a.analyzeStorageClass(a.analyzer, getFile)
	if err != nil {
		return nil, err
	}
	result.Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	return []*AnalyzeResult{result}, nil
}

func (a *AnalyzeStorageClass) analyzeStorageClass(analyzer *troubleshootv1beta2.StorageClass, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	storageClassesData, err := getCollectedFileContents(fmt.Sprintf("%s/%s.json", constants.CLUSTER_RESOURCES_DIR, constants.CLUSTER_RESOURCES_STORAGE_CLASS))
	if err != nil {
		return nil, err
	}

	var storageClasses storagev1beta1.StorageClassList
	if err := json.Unmarshal(storageClassesData, &storageClasses); err != nil {
		return nil, err
	}

	result := AnalyzeResult{
		Title:   a.Title(),
		IconKey: "kubernetes_storage_class",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/storage-class.svg?w=12&h=12",
	}

	for _, storageClass := range storageClasses.Items {
		val := storageClass.Annotations["storageclass.kubernetes.io/is-default-class"]
		if (storageClass.Name == analyzer.StorageClassName) || (analyzer.StorageClassName == "" && val == "true") {
			result.IsPass = true
			for _, outcome := range analyzer.Outcomes {
				if outcome.Pass != nil {
					result.Message = outcome.Pass.Message
					result.URI = outcome.Pass.URI
				}
			}
			if analyzer.StorageClassName == "" && result.Message == "" {
				result.Message = "Default Storage Class found"
			}

			return &result, nil
		}
	}

	result.IsFail = true
	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			result.Message = outcome.Fail.Message
			result.URI = outcome.Fail.URI
		}
	}
	if analyzer.StorageClassName == "" && result.Message == "" {
		result.Message = "No Default Storage Class found"
	}

	return &result, nil
}
