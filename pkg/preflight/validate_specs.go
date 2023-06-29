package preflight

import (
	"reflect"

	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
	"github.com/replicatedhq/troubleshoot/pkg/types"
)

// validatePreflight validates the preflight spec and returns a warning if there is any
func validatePreflight(specs PreflightSpecs) *types.ExitCodeWarning {

	if specs.PreflightSpec == nil && specs.HostPreflightSpec == nil {
		return types.NewExitCodeWarning("no preflight or host preflight spec was found")
	}

	if specs.PreflightSpec != nil {
		warning := validateSpecItems(specs.PreflightSpec.Spec.Collectors, specs.PreflightSpec.Spec.Analyzers, nil, nil)
		if warning != nil {
			return warning
		}
	}

	if specs.HostPreflightSpec != nil {
		warning := validateSpecItems(nil, nil, specs.HostPreflightSpec.Spec.Collectors, specs.HostPreflightSpec.Spec.Analyzers)
		if warning != nil {
			return warning
		}
	}

	return nil
}

// validateSpecItems validates the collectors and analyzers and returns a warning if there is any
func validateSpecItems(collectors []*v1beta2.Collect, analyzers []*v1beta2.Analyze, hostCollectors []*v1beta2.HostCollect, hostAnalyzers []*v1beta2.HostAnalyze) *types.ExitCodeWarning {
	var numberOfExcludedCollectors, numberOfExcludedAnalyzers int
	var numberOfExcluderHostCollectors, numberOfExcludeHostAnalyzers int

	numberOfCollectors := len(collectors)
	numberOfAnalyzers := len(analyzers)
	numberOfHostCollectors := len(hostCollectors)
	numberOfHostAnalyzers := len(hostAnalyzers)

	// if there are no collectors or analyzers, return a warning in both preflight and host preflight
	if numberOfCollectors == 0 && numberOfHostCollectors == 0 {
		return types.NewExitCodeWarning("No collectors found")
	}

	// if there are no collectors or analyzers, return a warning in both preflight and host preflight
	if numberOfAnalyzers == 0 && numberOfHostAnalyzers == 0 {
		return types.NewExitCodeWarning("No analyzers found")
	}

	// if there are collectors or analyzers, but all of them are excluded, return a warning
	if collectors != nil || analyzers != nil {
		collectorsInterface := make([]interface{}, len(collectors))
		for i, v := range collectors {
			collectorsInterface[i] = v
		}

		analyzersInterface := make([]interface{}, len(analyzers))
		for i, v := range analyzers {
			analyzersInterface[i] = v
		}

		numberOfExcludedCollectors = countExcludedItems(collectorsInterface)
		numberOfExcludedAnalyzers = countExcludedItems(analyzersInterface)
	}

	if numberOfExcludedCollectors == numberOfCollectors {
		return types.NewExitCodeWarning("All collectors were excluded by the applied values")
	}

	if numberOfExcludedAnalyzers == numberOfAnalyzers {
		return types.NewExitCodeWarning("All analyzers were excluded by the applied values")
	}

	// if there are host collectors or analyzers, but all of them are excluded, return a warning
	if hostCollectors != nil || hostAnalyzers != nil {
		collectorsInterface := make([]interface{}, len(hostCollectors))
		for i, v := range hostCollectors {
			collectorsInterface[i] = v
		}

		analyzersInterface := make([]interface{}, len(hostAnalyzers))
		for i, v := range hostAnalyzers {
			analyzersInterface[i] = v
		}

		numberOfExcluderHostCollectors = countExcludedItems(collectorsInterface)
		numberOfExcludeHostAnalyzers = countExcludedItems(analyzersInterface)
	}

	if numberOfExcluderHostCollectors == numberOfHostCollectors {
		return types.NewExitCodeWarning("All collectors were excluded by the applied values")
	}

	if numberOfExcludeHostAnalyzers == numberOfHostAnalyzers {
		return types.NewExitCodeWarning("All analyzers were excluded by the applied values")
	}

	return nil
}

// countExcludedItems counts and returns the number of excluded items in the given items slice.
// Items are assumed to be structures that may have an "Exclude" field as bool
// If the "Exclude" field is true, the item is considered excluded.
func countExcludedItems(items []interface{}) int {
	numberOfExcludedItems := 0
	for _, item := range items {
		itemElem := reflect.ValueOf(item).Elem()

		// Loop over all fields of the current item.
		for i := 0; i < itemElem.NumField(); i++ {
			// Get the value of the current field.
			itemValue := itemElem.Field(i)
			// If the current field is a pointer to a struct, check if it has an "Exclude" field.
			if !itemValue.IsNil() {
				elem := itemValue.Elem()
				if elem.Kind() == reflect.Struct {
					// Look for a field named "Exclude" in the struct.
					excludeField := elem.FieldByName("Exclude")
					if excludeField.IsValid() {
						// Try to get the field's value as a *multitype.BoolOrString.
						excludeValue, ok := excludeField.Interface().(*multitype.BoolOrString)
						// If the field's value was successfully obtained and is not nil, and the value is true
						if ok && excludeValue != nil {
							if excludeValue.BoolOrDefaultFalse() {
								numberOfExcludedItems++
							}
						}
					}
				}
			}
		}
	}
	return numberOfExcludedItems
}
