package analyzer

import (
	"encoding/json"
	"fmt"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
)

func analyzeStorageClass(analyzer *troubleshootv1beta1.StorageClass, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	storageClassesData, err := getCollectedFileContents("cluster-resources/storage-classes.json")
	if err != nil {
		return nil, err
	}

	var storageClasses []storagev1beta1.StorageClass
	if err := json.Unmarshal(storageClassesData, &storageClasses); err != nil {
		return nil, err
	}

	title := analyzer.CheckName
	if title == "" {
		title = fmt.Sprintf("Storage class %s", analyzer.StorageClassName)
	}

	result := AnalyzeResult{
		Title:   title,
		IconKey: "kubernetes_storage_class",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/storage-class.svg?w=12&h=12",
	}

	for _, storageClass := range storageClasses {
		if storageClass.Name == analyzer.StorageClassName {
			result.IsPass = true
			for _, outcome := range analyzer.Outcomes {
				if outcome.Pass != nil {
					result.Message = outcome.Pass.Message
					result.URI = outcome.Pass.URI
				}
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

	return &result, nil
}
