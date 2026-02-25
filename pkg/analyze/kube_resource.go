package analyzer

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	iutils "github.com/replicatedhq/troubleshoot/pkg/interfaceutils"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
)

var Filemap = map[string]string{
	"deployment":                     constants.CLUSTER_RESOURCES_DEPLOYMENTS,
	"daemonset":                      constants.CLUSTER_RESOURCES_DAEMONSETS,
	"statefulset":                    constants.CLUSTER_RESOURCES_STATEFULSETS,
	"networkpolicy":                  constants.CLUSTER_RESOURCES_NETWORK_POLICY,
	"pod":                            constants.CLUSTER_RESOURCES_PODS,
	"ingress":                        constants.CLUSTER_RESOURCES_INGRESS,
	"service":                        constants.CLUSTER_RESOURCES_SERVICES,
	"resourcequota":                  constants.CLUSTER_RESOURCES_RESOURCE_QUOTA,
	"job":                            constants.CLUSTER_RESOURCES_JOBS,
	"persistentvolumeclaim":          constants.CLUSTER_RESOURCES_PVCS,
	"pvc":                            constants.CLUSTER_RESOURCES_PVCS,
	"replicaset":                     constants.CLUSTER_RESOURCES_REPLICASETS,
	"configmap":                      constants.CLUSTER_RESOURCES_CONFIGMAPS,
	"validatingwebhookconfiguration": fmt.Sprintf("%s.json", constants.CLUSTER_RESOURCES_VALIDATING_WEBHOOK_CONFIGURATIONS),
	"mutatingwebhookconfiguration":   fmt.Sprintf("%s.json", constants.CLUSTER_RESOURCES_MUTATING_WEBHOOK_CONFIGURATIONS),
	"namespace":                      fmt.Sprintf("%s.json", constants.CLUSTER_RESOURCES_NAMESPACES),
	"persistentvolume":               fmt.Sprintf("%s.json", constants.CLUSTER_RESOURCES_PVS),
	"pv":                             fmt.Sprintf("%s.json", constants.CLUSTER_RESOURCES_PVS),
	"node":                           fmt.Sprintf("%s.json", constants.CLUSTER_RESOURCES_NODES),
	"storageclass":                   fmt.Sprintf("%s.json", constants.CLUSTER_RESOURCES_STORAGE_CLASS),
}

type AnalyzeClusterResource struct {
	analyzer *troubleshootv1beta2.ClusterResource
}

func (a *AnalyzeClusterResource) Title() string {
	title := a.analyzer.CheckName
	if title == "" {
		title = a.analyzer.CollectorName
	}

	return title
}

func (a *AnalyzeClusterResource) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeClusterResource) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	result, err := a.analyzeResource(a.analyzer, getFile)
	if err != nil {
		return nil, err
	}
	return []*AnalyzeResult{result}, nil
}

// FindResource locates and returns a kubernetes resource as an interface{} from a support bundle based on some basic selectors
// if clusterScoped is false and namespace is not provided, it will default to looking in the "default" namespace
func FindResource(kind string, clusterScoped bool, namespace string, name string, getFileContents getCollectedFileContents) (interface{}, error) {

	var datapath string

	// lowercase the kind to avoid case sensitivity
	kind = strings.ToLower(kind)

	resourceLocation, ok := Filemap[kind]

	if !ok {
		return nil, errors.New("failed to find resource")
	}

	datapath = filepath.Join(constants.CLUSTER_RESOURCES_DIR, resourceLocation)
	if !clusterScoped {
		if namespace == "" {
			namespace = "default"
		}
		datapath = filepath.Join(constants.CLUSTER_RESOURCES_DIR, resourceLocation, fmt.Sprintf("%s.json", namespace))
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
		resourceName, err := iutils.GetAtPath(item, "metadata.name")
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to find resource with name: %s", name)
		}
		if resourceName == name {
			return item, nil
		}
	}

	return nil, nil

}

func compareWhentoResource(w string, actual interface{}) (bool, error) {

	// check our "when" has operators
	var whenSplit []string
	if strings.ContainsAny(w, "!=<>") {
		whenSplit = strings.Split(strings.TrimSpace(w), " ")
	} else {
		return false, errors.New("no operators found")
	}

	// let's first check if we can cast "actual" as an int, that should inform us what comparison we're doing
	actualAsInt, ok := actual.(int)
	if ok {
		// it's an int! we can do integer comparison here
		// we're going to re-use an ill-fitting bit of code from the deployment analyzer for now
		return compareActualToWhen(w, actualAsInt)
	}

	// if we've fallen through here we're going to have to try a bit harder to work out what we're comparing
	// let's try making it a string
	actualAsString, ok := actual.(string)
	if !ok {
		return false, errors.New("could not cast found value as string")
	}

	// now we can try checking if it's a "quantity"
	actualASQuantity, err := resource.ParseQuantity(actualAsString)
	if err == nil {
		// it's probably a size, we can do some comparison here
		// but I'm being lazy here so we'll convert our last argument to an int and throw it back at our existing int comparison function
		whenAsQuantity, err := resource.ParseQuantity(whenSplit[1])
		if err != nil {
			// our when wasn't a size! naughty user
			return false, errors.New("Cannot compare size with not size")
		}
		whenIntAsString := strconv.FormatInt(whenAsQuantity.Value(), 10)
		// re-use that same compare function from earlier, might as well
		return compareActualToWhen(whenSplit[0]+" "+whenIntAsString, int(actualASQuantity.Value()))

	}

	return false, errors.New("could not match comparison method for result")
}

func analyzeWhenField(actual interface{}, outcomes []*troubleshootv1beta2.Outcome, checkName string) (*AnalyzeResult, error) {

	result := &AnalyzeResult{
		Title:   checkName,
		IconKey: "kubernetes_text_analyze",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg?w=13&h=16",
	}

	for _, outcome := range outcomes {
		if outcome.Fail != nil {
			if outcome.Fail.When != "" {
				compareResult, err := compareWhentoResource(outcome.Fail.When, actual)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to process when statement: %s", outcome.Fail.When)
				}
				if compareResult {
					result.IsFail = true
					result.Message = outcome.Fail.Message
					result.URI = outcome.Fail.URI
					return result, nil
				}
			} else {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}
		}
		if outcome.Warn != nil {

			if outcome.Warn.When != "" {
				compareResult, err := compareWhentoResource(outcome.Fail.When, actual)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to process when statement: %s", outcome.Warn.When)
				}
				if compareResult {
					result.IsWarn = true
					result.Message = outcome.Warn.Message
					result.URI = outcome.Warn.URI
					return result, nil
				}
			} else {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return result, nil
			}

		}
		if outcome.Pass != nil {

			if outcome.Pass.When != "" {
				compareResult, err := compareWhentoResource(outcome.Pass.When, actual)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to process when statement: %s", outcome.Pass.When)
				}
				if compareResult {
					result.IsPass = true
					result.Message = outcome.Pass.Message
					result.URI = outcome.Pass.URI
					return result, nil
				}
			} else {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return result, nil
			}
		}
	}

	return &AnalyzeResult{
		Title:   checkName,
		IconKey: "kubernetes_text_analyze",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
		IsFail:  true,
		Message: "Invalid analyzer",
	}, nil
}

func (a *AnalyzeClusterResource) analyzeResource(analyzer *troubleshootv1beta2.ClusterResource, getFileContents getCollectedFileContents) (*AnalyzeResult, error) {
	selected, err := FindResource(analyzer.Kind, analyzer.ClusterScoped, analyzer.Namespace, analyzer.Name, getFileContents)
	if err != nil {
		klog.Errorf("failed to find resource: %v", err)
	}
	if err != nil || selected == nil {
		var message string
		if analyzer.ClusterScoped {
			message = fmt.Sprintf("%s %s does not exist", analyzer.Kind, analyzer.Name)
		} else {
			message = fmt.Sprintf("%s %s in namespace %s does not exist", analyzer.Kind, analyzer.Name, analyzer.Namespace)
		}
		return &AnalyzeResult{
			Title:   a.Title(),
			IconKey: "kubernetes_text_analyze",
			IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			IsFail:  true,
			Message: message,
		}, nil
	}

	actual, err := iutils.GetAtPath(selected, analyzer.YamlPath)
	if err != nil {
		klog.Errorf("invalid yaml path: %s for kind: %s: %v", analyzer.YamlPath, analyzer.Kind, err)
		return &AnalyzeResult{
			Title:   a.Title(),
			IconKey: "kubernetes_text_analyze",
			IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			IsFail:  true,
			Message: "YAML path provided is invalid",
		}, nil
	}

	var expected interface{}
	err = yaml.Unmarshal([]byte(analyzer.ExpectedValue), &expected)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse expected value as yaml doc")
	}

	actualYAML, err := yaml.Marshal(actual)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal actual value")
	}

	if analyzer.ExpectedValue != "" {
		result, err := analyzeValue(expected, actual, analyzer.Outcomes, a.Title())
		if err != nil {
			return nil, err
		}
		if result != nil {
			return result, nil
		}
	} else if analyzer.RegexPattern != "" {
		result, err := analyzeRegexPattern(analyzer.RegexPattern, actualYAML, analyzer.Outcomes, a.Title())
		if err != nil {
			return nil, err
		}
		if result != nil {
			return result, nil
		}
	} else if analyzer.RegexGroups != "" {
		result, err := analyzeRegexGroups(analyzer.RegexGroups, actualYAML, analyzer.Outcomes, a.Title())
		if err != nil {
			return nil, err
		}
		if result != nil {
			return result, nil
		}
	} else {
		// fall through to comparing from the when key
		result, err := analyzeWhenField(actual, analyzer.Outcomes, a.Title())
		if err != nil {
			return nil, err
		}
		if result != nil {
			return result, nil
		}
	}

	return &AnalyzeResult{
		Title:   a.Title(),
		IconKey: "kubernetes_text_analyze",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
		IsFail:  true,
		Message: "Invalid analyzer",
	}, nil
}

func analyzeValue(expected interface{}, actual interface{}, outcomes []*troubleshootv1beta2.Outcome, checkName string) (*AnalyzeResult, error) {
	var err error

	equal := reflect.DeepEqual(actual, expected)

	result := &AnalyzeResult{
		Title:   checkName,
		IconKey: "kubernetes_text_analyze",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg?w=13&h=16",
	}

	for _, outcome := range outcomes {
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
		Title:   checkName,
		IconKey: "kubernetes_text_analyze",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
		IsFail:  true,
		Message: "Invalid analyzer",
	}, nil
}
