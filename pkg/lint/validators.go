package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"encoding/json"
)

func checkRequiredFields(parsed map[string]interface{}, content string) []LintError {
	errors := []LintError{}

	// Check apiVersion
	if apiVersion, ok := parsed["apiVersion"].(string); !ok || apiVersion == "" {
		errors = append(errors, LintError{
			Line:    findLineNumber(content, "apiVersion"),
			Field:   "apiVersion",
			Message: "Missing 'apiVersion'",
		})
	}

	// Check kind
	if kind, ok := parsed["kind"].(string); !ok || kind == "" {
		errors = append(errors, LintError{
			Line:    findLineNumber(content, "kind"),
			Field:   "kind",
			Message: "Missing or empty 'kind' field",
		})
	} else if kind != "Preflight" && kind != "SupportBundle" {
		errors = append(errors, LintError{
			Line:    findLineNumber(content, "kind"),
			Field:   "kind",
			Message: fmt.Sprintf("Expected kind 'Preflight' or 'SupportBundle' (found '%s')", kind),
		})
	}

	// Check metadata
	if _, ok := parsed["metadata"]; !ok {
		errors = append(errors, LintError{
			Line:    findLineNumber(content, "metadata"),
			Field:   "metadata",
			Message: "Missing 'metadata' section",
		})
	} else if metadata, ok := parsed["metadata"].(map[string]interface{}); ok {
		if name, ok := metadata["name"].(string); !ok || name == "" {
			errors = append(errors, LintError{
				Line:    findLineNumber(content, "name"),
				Field:   "metadata.name",
				Message: "Missing 'metadata.name'",
			})
		}
	}

	// Check spec
	if _, ok := parsed["spec"]; !ok {
		errors = append(errors, LintError{
			Line:    findLineNumber(content, "spec"),
			Field:   "spec",
			Message: "Missing 'spec' section",
		})
	}

	return errors
}

func checkPreflightSpec(parsed map[string]interface{}, content string) []LintError {
	errors := []LintError{}

	spec, ok := parsed["spec"].(map[string]interface{})
	if !ok {
		return errors
	}

	// Check for analyzers
	analyzers, hasAnalyzers := spec["analyzers"]
	if !hasAnalyzers {
		errors = append(errors, LintError{
			Line:    findLineNumber(content, "spec:"),
			Field:   "spec.analyzers",
			Message: "Preflight spec must contain 'analyzers'",
		})
	} else if analyzersList, ok := analyzers.([]interface{}); ok {
		if len(analyzersList) == 0 {
			errors = append(errors, LintError{
				Line:    findLineNumber(content, "analyzers"),
				Field:   "spec.analyzers",
				Message: "Preflight spec must have at least one analyzer",
			})
		}
	}

	return errors
}

func checkSupportBundleSpec(parsed map[string]interface{}, content string) []LintError {
	errors := []LintError{}

	spec, ok := parsed["spec"].(map[string]interface{})
	if !ok {
		return errors
	}

	// Check for collectors
	collectors, hasCollectors := spec["collectors"]
	_, hasHostCollectors := spec["hostCollectors"]

	if !hasCollectors && !hasHostCollectors {
		errors = append(errors, LintError{
			Line:    findLineNumber(content, "spec:"),
			Field:   "spec.collectors",
			Message: "SupportBundle spec must contain 'collectors' or 'hostCollectors'",
		})
	} else {
		// Check if collectors list is empty
		if hasCollectors {
			if collectorsList, ok := collectors.([]interface{}); ok && len(collectorsList) == 0 {
				errors = append(errors, LintError{
					Line:    findLineNumber(content, "collectors"),
					Field:   "spec.collectors",
					Message: "Collectors list is empty",
				})
			}
		}
	}

	return errors
}

// checkCommonIssues aggregates advisory warnings based on best practices
func checkCommonIssues(parsed map[string]interface{}, content string, apiVersion string, templateValueRefs []string) []LintWarning {
	warnings := []LintWarning{}

	// Check for missing docStrings in analyzers and collectors
	spec, ok := parsed["spec"].(map[string]interface{})
	if !ok {
		return warnings
	}

	// Check if any analyzers are missing docString
	analyzersMissingDocString := false
	if analyzers, ok := spec["analyzers"].([]interface{}); ok {
		for _, analyzer := range analyzers {
			if analyzerMap, ok := analyzer.(map[string]interface{}); ok {
				// Check if docString exists at the analyzer level (v1beta3)
				if _, hasDocString := analyzerMap["docString"]; !hasDocString {
					analyzersMissingDocString = true
					break
				}
			}
		}
	}

	// Check if any collectors are missing docString
	collectorsMissingDocString := false
	if collectors, ok := spec["collectors"].([]interface{}); ok {
		for _, collector := range collectors {
			if collectorMap, ok := collector.(map[string]interface{}); ok {
				// Get the actual collector type (first key-value pair)
				for _, collectorSpec := range collectorMap {
					if specMap, ok := collectorSpec.(map[string]interface{}); ok {
						if _, hasDocString := specMap["docString"]; !hasDocString {
							collectorsMissingDocString = true
							break
						}
					}
					// Only check the first key since collectors have single type
					break
				}
			}
			if collectorsMissingDocString {
				break
			}
		}
	}

	// Add consolidated warnings if any items are missing docString
	if analyzersMissingDocString && collectorsMissingDocString {
		warnings = append(warnings, LintWarning{
			Line:    findLineNumber(content, "spec:"),
			Field:   "spec",
			Message: "Some analyzers and collectors are missing docString (recommended for v1beta3)",
		})
	} else if analyzersMissingDocString {
		warnings = append(warnings, LintWarning{
			Line:    findLineNumber(content, "analyzers:"),
			Field:   "spec.analyzers",
			Message: "Some analyzers are missing docString (recommended for v1beta3)",
		})
	} else if collectorsMissingDocString {
		warnings = append(warnings, LintWarning{
			Line:    findLineNumber(content, "collectors:"),
			Field:   "spec.collectors",
			Message: "Some collectors are missing docString (recommended for v1beta3)",
		})
	}

	// Add warning about template values that need to be provided at runtime (v1beta3 only)
	if apiVersion == "troubleshoot.sh/v1beta3" && len(templateValueRefs) > 0 {
		warnings = append(warnings, LintWarning{
			Line:    1,
			Field:   "template-values",
			Message: fmt.Sprintf("Template values that must be provided at runtime: %s", strings.Join(templateValueRefs, ", ")),
		})
	}

	return warnings
}

// --- Schema-backed quick validation (best-effort) ---

type schemaTypeInfo struct {
	required   map[string]struct{}
	properties map[string]struct{}
}

var (
	knownAnalyzerTypes  map[string]struct{}
	knownCollectorTypes map[string]struct{}
	analyzerTypeInfo    map[string]schemaTypeInfo
	collectorTypeInfo   map[string]schemaTypeInfo
	knownTypesLoaded    bool
)

func ensureKnownTypesLoaded() {
	if knownTypesLoaded {
		return
	}
	knownAnalyzerTypes = map[string]struct{}{}
	knownCollectorTypes = map[string]struct{}{}
	analyzerTypeInfo = map[string]schemaTypeInfo{}
	collectorTypeInfo = map[string]schemaTypeInfo{}

	// Analyzer schema (v1beta2)
	loadKeysFromSchema(
		filepath.Join("schemas", "analyzer-troubleshoot-v1beta2.json"),
		[]string{"properties", "spec", "properties", "analyzers", "items", "properties"},
		knownAnalyzerTypes,
	)
	loadTypeInfoFromSchema(
		filepath.Join("schemas", "analyzer-troubleshoot-v1beta2.json"),
		[]string{"properties", "spec", "properties", "analyzers", "items", "properties"},
		analyzerTypeInfo,
	)
	// Collector schema (v1beta2)
	loadKeysFromSchema(
		filepath.Join("schemas", "collector-troubleshoot-v1beta2.json"),
		[]string{"properties", "spec", "properties", "collectors", "items", "properties"},
		knownCollectorTypes,
	)
	loadTypeInfoFromSchema(
		filepath.Join("schemas", "collector-troubleshoot-v1beta2.json"),
		[]string{"properties", "spec", "properties", "collectors", "items", "properties"},
		collectorTypeInfo,
	)

	knownTypesLoaded = true
}

// loadKeysFromSchema walks a JSON object by keysPath and adds map keys at that node into dest
func loadKeysFromSchema(schemaPath string, keysPath []string, dest map[string]struct{}) {
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return
	}
	node := interface{}(obj)
	for _, key := range keysPath {
		m, ok := node.(map[string]interface{})
		if !ok {
			return
		}
		node, ok = m[key]
		if !ok {
			return
		}
	}
	props, ok := node.(map[string]interface{})
	if !ok {
		return
	}
	for k := range props {
		dest[k] = struct{}{}
	}
}

// loadTypeInfoFromSchema records required/properties for each type under the node
func loadTypeInfoFromSchema(schemaPath string, keysPath []string, dest map[string]schemaTypeInfo) {
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return
	}
	node := interface{}(obj)
	for _, key := range keysPath {
		m, ok := node.(map[string]interface{})
		if !ok {
			return
		}
		node, ok = m[key]
		if !ok {
			return
		}
	}
	typesNode, ok := node.(map[string]interface{})
	if !ok {
		return
	}
	for typeName, raw := range typesNode {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		info := schemaTypeInfo{required: map[string]struct{}{}, properties: map[string]struct{}{}}
		if req, ok := m["required"].([]interface{}); ok {
			for _, r := range req {
				if s, ok := r.(string); ok {
					info.required[s] = struct{}{}
				}
			}
		}
		if props, ok := m["properties"].(map[string]interface{}); ok {
			for prop := range props {
				info.properties[prop] = struct{}{}
			}
		}
		dest[typeName] = info
	}
}

// validateAnalyzers delegates to the generic typed-list validator
func validateAnalyzers(parsed map[string]interface{}, content string) []LintError {
	return validateTypedList(parsed, content, "analyzers", "analyzer", knownAnalyzerTypes, analyzerTypeInfo)
}

func validateCollectors(parsed map[string]interface{}, content string, field string) []LintError {
	return validateTypedList(parsed, content, field, "collector", knownCollectorTypes, collectorTypeInfo)
}

// validateTypedList provides generic validation for lists of typed single-key objects
func validateTypedList(
	parsed map[string]interface{},
	content string,
	listKey string,
	subject string,
	knownTypes map[string]struct{},
	typeInfo map[string]schemaTypeInfo,
) []LintError {
	var errs []LintError
	spec, ok := parsed["spec"].(map[string]interface{})
	if !ok {
		return errs
	}
	raw, exists := spec[listKey]
	if !exists {
		return errs
	}
	list, ok := raw.([]interface{})
	if !ok {
		errs = append(errs, LintError{
			Line:    findLineNumber(content, listKey+":"),
			Field:   "spec." + listKey,
			Message: fmt.Sprintf("Expected '%s' to be a list", listKey),
		})
		return errs
	}
	for i, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			errs = append(errs, LintError{
				Line:    findListItemLine(content, listKey, i),
				Field:   fmt.Sprintf("spec.%s[%d]", listKey, i),
				Message: fmt.Sprintf("Expected %s entry to be a mapping", subject),
			})
			continue
		}
		// Count non-docString keys (docString is metadata in v1beta3, not a type)
		typeCount := 0
		var typ string
		var body interface{}
		for k, v := range m {
			if k == "docString" {
				// docString is metadata, not a type - skip it
				continue
			} else {
				typeCount++
				typ, body = k, v
			}
		}

		// Check that we have exactly one type (excluding docString)
		if typeCount != 1 {
			errs = append(errs, LintError{
				Line:    findListItemLine(content, listKey, i),
				Field:   fmt.Sprintf("spec.%s[%d]", listKey, i),
				Message: fmt.Sprintf("%s entry must specify exactly one %s type", strings.Title(subject), subject),
			})
			continue
		}

		// If no actual type was found (only docString), skip further validation
		if typ == "" {
			continue
		}
		if len(knownTypes) > 0 {
			if _, ok := knownTypes[typ]; !ok {
				errs = append(errs, LintError{
					Line:    findListItemLine(content, listKey, i),
					Field:   fmt.Sprintf("spec.%s[%d]", listKey, i),
					Message: fmt.Sprintf("Unknown %s type '%s'", subject, typ),
				})
			}
		}
		bodyMap, ok := body.(map[string]interface{})
		if !ok {
			errs = append(errs, LintError{
				Line:    findListItemLine(content, listKey, i),
				Field:   fmt.Sprintf("spec.%s[%d].%s", listKey, i, typ),
				Message: fmt.Sprintf("Expected %s definition to be a mapping", subject),
			})
			continue
		}
		if ti, ok := typeInfo[typ]; ok {
			for req := range ti.required {
				if _, ok := bodyMap[req]; !ok {
					errs = append(errs, LintError{
						Line:    findListItemLine(content, listKey, i),
						Field:   fmt.Sprintf("spec.%s[%d].%s.%s", listKey, i, typ, req),
						Message: fmt.Sprintf("Missing required field '%s' for %s '%s'", req, subject, typ),
					})
				}
			}
			for k := range bodyMap {
				if _, ok := ti.properties[k]; !ok {
					errs = append(errs, LintError{
						Line:    findListItemLine(content, listKey, i),
						Field:   fmt.Sprintf("spec.%s[%d].%s.%s", listKey, i, typ, k),
						Message: fmt.Sprintf("Unknown field '%s' for %s '%s'", k, subject, typ),
					})
				}
			}
		}
	}
	return errs
}
