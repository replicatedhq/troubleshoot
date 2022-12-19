package analyzer

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	iutils "github.com/replicatedhq/troubleshoot/pkg/interfaceutils"
	"gopkg.in/yaml.v2"
)

var Filemap = map[string]string{
	"Deployment":           "deployments",
	"StatefulSet":          "statefulsets",
	"NetworkPolicy":        "network-policy",
	"Pod":                  "pods",
	"Ingress":              "ingress",
	"Service":              "services",
	"ResourceQuota":        "resource-quotas",
	"Job":                  "jobs",
	"PersistentVoumeClaim": "pvcs",
	"pvc":                  "pvcs",
	"ReplicaSet":           "replicasets",
	"Namespace":            "namespaces.json",
	"PersistentVolume":     "pvs.json",
	"pv":                   "pvs.json",
	"Node":                 "nodes.json",
	"StorageClass":         "storage-classes.json",
}

// FindResource locates and returns a kubernetes resource as an interface{} from a support bundle based on some basic selectors
// if clusterScoped is false and namespace is not provided, it will default to looking in the "default" namespace
func FindResource(kind string, clusterScoped bool, namespace string, name string, getFileContents getCollectedFileContents) (interface{}, error) {

	var datapath string

	resourceLocation, ok := Filemap[kind]

	if !ok {
		return nil, errors.New("failed to find resource")
	}

	datapath = filepath.Join("cluster-resources", resourceLocation)
	if !clusterScoped {
		if namespace == "" {
			namespace = "default"
		}
		datapath = filepath.Join("cluster-resources", resourceLocation, fmt.Sprintf("%s.json", namespace))
	}

	file, err := getFileContents(datapath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read collected resources")
	}

	var resource interface{}
	err = yaml.Unmarshal(file, &resource)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse data as yaml doc")
	}

	items, err := iutils.GetAtPath(resource, "items")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get items from file")
	}
	itemslice := items.([]interface{})
	for _, item := range itemslice {
		name, err := iutils.GetAtPath(item, "metadata.name")
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to find resource with name: %s", name)
		}
		if name == name {
			return item, nil
		}
	}

	return nil, nil

}

func analyzeResource(analyzer *troubleshootv1beta2.ClusterResource, getFileContents getCollectedFileContents) (*AnalyzeResult, error) {

	selected, err := FindResource(analyzer.Kind, analyzer.ClusterScoped, analyzer.Namespace, analyzer.Name, getFileContents)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find resource")
	}

	actual, err := iutils.GetAtPath(selected, analyzer.YamlPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get object at path: %s", analyzer.YamlPath)
	}

	var expected interface{}
	err = yaml.Unmarshal([]byte(analyzer.ExpectedValue), &expected)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse expected value as yaml doc")
	}

	title := analyzer.CheckName
	if title == "" {
		title = analyzer.CollectorName
	}

	result := &AnalyzeResult{
		Title:   title,
		IconKey: "kubernetes_text_analyze",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
	}

	equal := reflect.DeepEqual(actual, expected)

	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			when := false
			if outcome.Fail.When != "" {
				when, err = strconv.ParseBool(outcome.Fail.When)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to process when statement: %s", outcome.Fail.When)
				}
			}

			if when == equal {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI
				return result, nil
			}
		} else if outcome.Warn != nil {
			when := false
			if outcome.Warn.When != "" {
				when, err = strconv.ParseBool(outcome.Warn.When)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to process when statement: %s", outcome.Warn.When)
				}
			}

			if when == equal {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI
				return result, nil
			}
		} else if outcome.Pass != nil {
			when := true // default to passing when values are equal
			if outcome.Pass.When != "" {
				when, err = strconv.ParseBool(outcome.Pass.When)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to process when statement: %s", outcome.Pass.When)
				}
			}

			if when == equal {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI
				return result, nil
			}
		}
	}

	return &AnalyzeResult{
		Title:   title,
		IconKey: "kubernetes_text_analyze",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
		IsFail:  true,
		Message: "Invalid analyzer",
	}, nil
}
