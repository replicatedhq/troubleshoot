package preflight

import (
	"context"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/internal/specs"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/loader"
	"github.com/spf13/viper"
	"helm.sh/helm/v3/pkg/strvals"
	"k8s.io/client-go/kubernetes"
	yaml "sigs.k8s.io/yaml"
)

func readSpecs(args []string) (*loader.TroubleshootKinds, error) {
	config, err := k8sutil.GetRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert create k8s client")
	}

	// Pre-process v1beta3 specs with templates if values are provided
	processedArgs, err := preprocessV1Beta3Specs(args)
	if err != nil {
		return nil, errors.Wrap(err, "failed to preprocess v1beta3 specs")
	}

	ctx := context.Background()
	kinds, err := specs.LoadFromCLIArgs(ctx, client, processedArgs, viper.GetViper())
	if err != nil {
		return nil, err
	}

	// Load additional specs from URIs
	// only when no-uri flag is not set
	if !viper.GetBool("no-uri") {
		specs.LoadAdditionalSpecFromURIs(ctx, kinds)
	}

	ret := loader.NewTroubleshootKinds()

	// Concatenate all preflight inclusterSpecs that don't have an upload destination
	inclusterSpecs := []troubleshootv1beta2.Preflight{}
	var concatenatedSpec *troubleshootv1beta2.Preflight
	for _, v := range kinds.PreflightsV1Beta2 {
		v := v // https://golang.org/doc/faq#closures_and_goroutines
		if v.Spec.UploadResultsTo == "" {
			concatenatedSpec = ConcatPreflightSpec(concatenatedSpec, &v)
		} else {
			inclusterSpecs = append(inclusterSpecs, v)
		}
	}

	if concatenatedSpec != nil {
		inclusterSpecs = append(inclusterSpecs, *concatenatedSpec)
	}
	ret.PreflightsV1Beta2 = inclusterSpecs

	var hostSpec *troubleshootv1beta2.HostPreflight
	for _, v := range kinds.HostPreflightsV1Beta2 {
		v := v // https://golang.org/doc/faq#closures_and_goroutines
		hostSpec = ConcatHostPreflightSpec(hostSpec, &v)
	}
	if hostSpec != nil {
		ret.HostPreflightsV1Beta2 = []troubleshootv1beta2.HostPreflight{*hostSpec}
	}

	return ret, nil
}

// preprocessV1Beta3Specs processes v1beta3 specs with template rendering if values are provided
func preprocessV1Beta3Specs(args []string) ([]string, error) {
	valuesFiles := viper.GetStringSlice("values")
	setValues := viper.GetStringSlice("set")

	// If no values provided, return args unchanged
	if len(valuesFiles) == 0 && len(setValues) == 0 {
		return args, nil
	}

	// Load values from files and --set flags
	values := make(map[string]interface{})
	for _, valuesFile := range valuesFiles {
		if valuesFile == "" {
			continue
		}
		data, err := os.ReadFile(valuesFile)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read values file %s", valuesFile)
		}

		var fileValues map[string]interface{}
		if err := yaml.Unmarshal(data, &fileValues); err != nil {
			return nil, errors.Wrapf(err, "failed to parse values file %s", valuesFile)
		}

		values = mergeMaps(values, fileValues)
	}

	// Apply --set values
	for _, setValue := range setValues {
		if err := strvals.ParseInto(setValue, values); err != nil {
			return nil, errors.Wrapf(err, "failed to parse --set value: %s", setValue)
		}
	}

	// Process each arg
	processedArgs := make([]string, 0, len(args))
	for _, arg := range args {
		// Skip non-file arguments (like URLs, stdin, etc.)
		if arg == "-" || strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") ||
			strings.HasPrefix(arg, "secret/") || strings.HasPrefix(arg, "configmap/") {
			processedArgs = append(processedArgs, arg)
			continue
		}

		// Check if file exists
		if _, err := os.Stat(arg); err != nil {
			processedArgs = append(processedArgs, arg)
			continue
		}

		// Read the file
		content, err := os.ReadFile(arg)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read file %s", arg)
		}

		// Check if it's a v1beta3 spec with templates
		var parsed map[string]interface{}
		if err := yaml.Unmarshal(content, &parsed); err != nil {
			// Not valid YAML, might be templated - try to detect v1beta3
			contentStr := string(content)
			if strings.Contains(contentStr, "apiVersion: troubleshoot.sh/v1beta3") &&
				strings.Contains(contentStr, "{{") && strings.Contains(contentStr, "}}") {
				// It's a v1beta3 template, render it
				rendered, err := RenderWithHelmTemplate(contentStr, values)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to render v1beta3 template %s", arg)
				}
				// Write to temp file
				tmpFile, err := os.CreateTemp("", "preflight-rendered-*.yaml")
				if err != nil {
					return nil, errors.Wrap(err, "failed to create temp file")
				}
				if _, err := tmpFile.WriteString(rendered); err != nil {
					tmpFile.Close()
					os.Remove(tmpFile.Name())
					return nil, errors.Wrap(err, "failed to write rendered template")
				}
				tmpFile.Close()
				processedArgs = append(processedArgs, tmpFile.Name())
			} else {
				processedArgs = append(processedArgs, arg)
			}
		} else {
			// Valid YAML, check if it's v1beta3 with templates
			if apiVersion, ok := parsed["apiVersion"]; ok && apiVersion == constants.Troubleshootv1beta3Kind {
				contentStr := string(content)
				if strings.Contains(contentStr, "{{") && strings.Contains(contentStr, "}}") {
					// It's a v1beta3 template, render it
					rendered, err := RenderWithHelmTemplate(contentStr, values)
					if err != nil {
						return nil, errors.Wrapf(err, "failed to render v1beta3 template %s", arg)
					}
					// Write to temp file
					tmpFile, err := os.CreateTemp("", "preflight-rendered-*.yaml")
					if err != nil {
						return nil, errors.Wrap(err, "failed to create temp file")
					}
					if _, err := tmpFile.WriteString(rendered); err != nil {
						tmpFile.Close()
						os.Remove(tmpFile.Name())
						return nil, errors.Wrap(err, "failed to write rendered template")
					}
					tmpFile.Close()
					processedArgs = append(processedArgs, tmpFile.Name())
				} else {
					// v1beta3 but no templates
					processedArgs = append(processedArgs, arg)
				}
			} else {
				// Not v1beta3
				processedArgs = append(processedArgs, arg)
			}
		}
	}

	return processedArgs, nil
}
