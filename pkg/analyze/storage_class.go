package analyzer

import (
	"encoding/json"
	"fmt"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
)

func analyzeStorageClass(analyzer *troubleshootv1beta2.StorageClass, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	storageClassesData, err := getCollectedFileContents("cluster-resources/storage-classes.json")
	if err != nil {
		return nil, err
	}

	var storageClasses storagev1beta1.StorageClassList
	if err := json.Unmarshal(storageClassesData, &storageClasses); err != nil {
		return nil, err
	}

	title := analyzer.CheckName
	if title == "" {
		if analyzer.StorageClassName != "" {
			title = fmt.Sprintf("Storage class %s", analyzer.StorageClassName)
		} else {
			title = "Default Storage Class"
		}
	}

	result := AnalyzeResult{
		Title:   title,
		IconKey: "kubernetes_storage_class",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/storage-class.svg?w=12&h=12",
	}

	for _, storageClass := range storageClasses.Items {
		val, _ := storageClass.Annotations["storageclass.kubernetes.io/is-default-class"]
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
