package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"encoding/json"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/internal/util"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"sigs.k8s.io/yaml"
)

type LintResult struct {
	FilePath string
	Errors   []LintError
	Warnings []LintWarning
}

type LintError struct {
	Line    int
	Column  int
	Message string
	Field   string
}

type LintWarning struct {
	Line    int
	Column  int
	Message string
	Field   string
}

type LintOptions struct {
	FilePaths []string
	Fix       bool
	Format    string // "text" or "json"
}

// LintFiles validates v1beta3 troubleshoot specs for syntax and structural errors
func LintFiles(opts LintOptions) ([]LintResult, error) {
	results := []LintResult{}

	// Load known analyzer/collector types from schemas (best effort)
	ensureKnownTypesLoaded()

	for _, filePath := range opts.FilePaths {
		// Read entire file once
		fileBytes, readErr := os.ReadFile(filePath)
		if readErr != nil {
			return nil, errors.Wrapf(readErr, "failed to read file %s", filePath)
		}
		fileContent := string(fileBytes)

		// Split into YAML documents
		docs := util.SplitYAML(fileContent)

		// Pre-compute starting line number for each doc within the file (1-based)
		docStarts := make([]int, len(docs))
		runningStart := 1
		for i, d := range docs {
			docStarts[i] = runningStart
			// Count lines in this doc
			runningStart += util.EstimateNumberOfLines(d)
			// Account for the '---' separator line between documents
			if i < len(docs)-1 {
				runningStart += 1
			}
		}

		// Lint each document, in parallel
		type docOutcome struct {
			errs    []LintError
			warns   []LintWarning
			newDoc  string
			changed bool
		}
		outcomes := make([]docOutcome, len(docs))
		var wg sync.WaitGroup
		wg.Add(len(docs))
		for i := range docs {
			i := i
			go func() {
				defer wg.Done()
				// Compute lint result for this doc, optionally applying fixes in-memory
				res, finalDoc, _ /*changed*/, _ := lintContentInMemory(docs[i], opts.Fix)

				// Adjust line numbers to file coordinates
				lineOffset := docStarts[i] - 1
				for idx := range res.Errors {
					if res.Errors[idx].Line > 0 {
						res.Errors[idx].Line += lineOffset
					}
				}
				for idx := range res.Warnings {
					if res.Warnings[idx].Line > 0 {
						res.Warnings[idx].Line += lineOffset
					}
				}

				changed := finalDoc != docs[i]
				outcomes[i] = docOutcome{
					errs:    res.Errors,
					warns:   res.Warnings,
					newDoc:  finalDoc,
					changed: changed,
				}
			}()
		}
		wg.Wait()

		// Assemble per-file result
		fileResult := LintResult{FilePath: filePath}
		writeNeeded := false
		newDocs := make([]string, len(docs))
		for i, oc := range outcomes {
			fileResult.Errors = append(fileResult.Errors, oc.errs...)
			fileResult.Warnings = append(fileResult.Warnings, oc.warns...)
			if oc.changed {
				writeNeeded = true
			}
			if oc.newDoc == "" {
				newDocs[i] = docs[i]
			} else {
				newDocs[i] = oc.newDoc
			}
		}

		if writeNeeded {
			// Reassemble with the same delimiter used by util.SplitYAML
			updated := strings.Join(newDocs, "\n---\n")
			if writeErr := os.WriteFile(filePath, []byte(updated), 0644); writeErr != nil {
				return nil, errors.Wrapf(writeErr, "failed to write fixed content to %s", filePath)
			}
		}

		results = append(results, fileResult)
	}

	return results, nil
}

func lintContentInMemory(content string, fix bool) (LintResult, string, bool, error) {
	// Compute result for the provided content
	compute := func(body string) LintResult {
		res := LintResult{Errors: []LintError{}, Warnings: []LintWarning{}}

		// Check if content contains template expressions
		hasTemplates := strings.Contains(body, "{{") && strings.Contains(body, "}}")

		// Validate YAML syntax (but be lenient with templated files)
		var parsed map[string]interface{}
		if err := yaml.Unmarshal([]byte(body), &parsed); err != nil {
			// If the content has templates, YAML parsing may fail - that's expected for v1beta3 only
			if !hasTemplates {
				res.Errors = append(res.Errors, LintError{
					Line:    extractLineFromError(err),
					Message: fmt.Sprintf("YAML syntax error: %s", err.Error()),
				})
				return res
			}

			// Attempt to detect apiVersion from raw content
			detectedAPIVersion := detectAPIVersionFromContent(body)
			if detectedAPIVersion == "" {
				res.Errors = append(res.Errors, LintError{
					Line:    findLineNumber(body, "apiVersion"),
					Field:   "apiVersion",
					Message: "Missing or unreadable 'apiVersion' field",
				})
				return res
			}

			if detectedAPIVersion == constants.Troubleshootv1beta2Kind {
				// v1beta2 does not support templating
				addTemplatingErrorsForAllLines(&res, body)
				return res
			}

			// For v1beta3 with templates, we can't parse YAML strictly, so just check template syntax
			templateErrors, templateValueRefs := checkTemplateSyntax(body)
			res.Errors = append(res.Errors, templateErrors...)

			// Add warning about template values for v1beta3
			if detectedAPIVersion == constants.Troubleshootv1beta3Kind && len(templateValueRefs) > 0 {
				res.Warnings = append(res.Warnings, LintWarning{
					Line:    1,
					Field:   "template-values",
					Message: fmt.Sprintf("Template values that must be provided at runtime: %s", strings.Join(templateValueRefs, ", ")),
				})
			}

			return res
		}

		// Determine apiVersion from parsed YAML
		apiVersion := ""
		if v, ok := parsed["apiVersion"].(string); ok {
			apiVersion = v
		}
		if apiVersion == "" {
			res.Errors = append(res.Errors, LintError{
				Line:    findLineNumber(body, "apiVersion"),
				Field:   "apiVersion",
				Message: "Missing or empty 'apiVersion' field",
			})
			return res
		}

		// Templating policy: only v1beta3 supports templating
		if apiVersion == constants.Troubleshootv1beta2Kind && hasTemplates {
			addTemplatingErrorsForAllLines(&res, body)
		}

		// Check required fields
		res.Errors = append(res.Errors, checkRequiredFields(parsed, body)...)

		// Check template syntax and collect template value references
		templateErrors, templateValueRefs := checkTemplateSyntax(body)
		res.Errors = append(res.Errors, templateErrors...)

		// Check for kind-specific requirements
		if kind, ok := parsed["kind"].(string); ok {
			switch kind {
			case "Preflight":
				res.Errors = append(res.Errors, checkPreflightSpec(parsed, body)...)
				// Validate analyzer entries
				res.Errors = append(res.Errors, validateAnalyzers(parsed, body)...)
			case "SupportBundle":
				res.Errors = append(res.Errors, checkSupportBundleSpec(parsed, body)...)
				// Validate analyzers if present in SupportBundle specs as well
				res.Errors = append(res.Errors, validateAnalyzers(parsed, body)...)
				// Validate collector entries (collectors and hostCollectors)
				res.Errors = append(res.Errors, validateCollectors(parsed, body, "collectors")...)
				res.Errors = append(res.Errors, validateCollectors(parsed, body, "hostCollectors")...)
			}
		}

		// Check for common issues
		res.Warnings = append(res.Warnings, checkCommonIssues(parsed, body, apiVersion, templateValueRefs)...)

		return res
	}

	// Initial lint
	result := compute(content)

	// Apply fixes if requested (multi-pass within a single invocation), in-memory
	changed := false
	if fix && (len(result.Errors) > 0 || len(result.Warnings) > 0) {
		const maxFixPasses = 3
		for pass := 0; pass < maxFixPasses; pass++ {
			updatedContent, fixed, err := applyFixesInMemory(content, result)
			if err != nil {
				return result, content, changed, err
			}
			if !fixed {
				break
			}
			changed = true
			content = updatedContent
			// Recompute without applying fixes in this cycle
			result = compute(content)
			if len(result.Errors) == 0 && len(result.Warnings) == 0 {
				break
			}
		}
	}

	return result, content, changed, nil
}

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

// detectAPIVersionFromContent tries to extract apiVersion from raw YAML text
func detectAPIVersionFromContent(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "apiVersion:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				val := strings.TrimSpace(parts[1])
				// strip quotes if present
				val = strings.Trim(val, "'\"")
				return val
			}
		}
	}
	return ""
}

// addTemplatingErrorsForAllLines records an error for each line containing template braces in versions that do not support templating
func addTemplatingErrorsForAllLines(result *LintResult, content string) {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, "{{") && strings.Contains(line, "}}") {
			result.Errors = append(result.Errors, LintError{
				Line:    i + 1,
				Message: "Templating is not supported in v1beta2 specs",
				Field:   "template",
			})
		}
	}
}

func checkTemplateSyntax(content string) ([]LintError, []string) {
	errors := []LintError{}
	lines := strings.Split(content, "\n")
	templateValueRefs := map[string]bool{}

	// Check for unmatched braces
	for i, line := range lines {
		// Count opening and closing braces
		opening := strings.Count(line, "{{")
		closing := strings.Count(line, "}}")

		if opening != closing {
			errors = append(errors, LintError{
				Line:    i + 1,
				Message: fmt.Sprintf("Unmatched template braces: %d opening, %d closing", opening, closing),
			})
		}

		// Check for common template syntax issues
		// Look for templates that might be missing the leading dot
		if strings.Contains(line, "{{") && strings.Contains(line, "}}") {
			// Extract template expressions
			templateExpr := extractTemplateBetweenBraces(line)
			for _, expr := range templateExpr {
				trimmed := strings.TrimSpace(expr)

				// Skip empty expressions
				if trimmed == "" {
					continue
				}

				// Skip comments: {{/* ... */}}
				if strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*/") {
					continue
				}

				// Track template value references for warning (check this before skipping control structures)
				if strings.Contains(trimmed, ".Values.") {
					// Extract the value path
					valuePattern := regexp.MustCompile(`\.Values\.(\w+(?:\.\w+)*)`)
					matches := valuePattern.FindAllStringSubmatch(trimmed, -1)
					for _, match := range matches {
						if len(match) > 1 {
							templateValueRefs[match[1]] = true
						}
					}
				}

				// Skip control structures (if, else, end, range, with, etc.)
				if isControlStructure(trimmed) {
					continue
				}

				// Skip template variables (start with $)
				if strings.HasPrefix(trimmed, "$") {
					continue
				}

				// Skip expressions that start with a dot (valid references)
				if strings.HasPrefix(trimmed, ".") {
					continue
				}

				// Skip string literals
				if strings.HasPrefix(trimmed, "\"") || strings.HasPrefix(trimmed, "'") {
					continue
				}

				// Skip numeric literals
				if regexp.MustCompile(`^[0-9]+$`).MatchString(trimmed) {
					continue
				}

				// Skip function calls (contain parentheses or pipes)
				if strings.Contains(trimmed, "(") || strings.Contains(trimmed, "|") {
					continue
				}

				// Skip known Helm functions/keywords
				helmFunctions := []string{"toYaml", "toJson", "include", "required", "default", "quote", "nindent", "indent", "upper", "lower", "trim"}
				isFunction := false
				for _, fn := range helmFunctions {
					if strings.HasPrefix(trimmed, fn+" ") || trimmed == fn {
						isFunction = true
						break
					}
				}
				if isFunction {
					continue
				}

				// If we got here, it might be missing a leading dot
				errors = append(errors, LintError{
					Line:    i + 1,
					Message: fmt.Sprintf("Template expression may be missing leading dot: {{ %s }}", expr),
				})
			}
		}
	}

	// Collect template values that need to be provided at runtime
	var valueList []string
	for val := range templateValueRefs {
		valueList = append(valueList, val)
	}
	// Sort for consistent output
	sort.Strings(valueList)

	return errors, valueList
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

// findCollectorLine locates the starting line of the Nth entry in a collectors list
func findCollectorLine(content string, field string, index int) int {
	return findListItemLine(content, field, index)
}

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
	if apiVersion == constants.Troubleshootv1beta3Kind && len(templateValueRefs) > 0 {
		warnings = append(warnings, LintWarning{
			Line:    1,
			Field:   "template-values",
			Message: fmt.Sprintf("Template values that must be provided at runtime: %s", strings.Join(templateValueRefs, ", ")),
		})
	}

	return warnings
}

func applyFixesInMemory(content string, result LintResult) (string, bool, error) {
	fixed := false
	newContent := content
	lines := strings.Split(newContent, "\n")

	// Fix A: If templating errors exist in a v1beta2 file, upgrade apiVersion to v1beta3 (minimal, deterministic)
	hasTemplateInV1beta2 := false
	for _, e := range result.Errors {
		if e.Field == "template" && strings.Contains(e.Message, "not supported in v1beta2") {
			hasTemplateInV1beta2 = true
			break
		}
	}
	if hasTemplateInV1beta2 {
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "apiVersion:") && strings.Contains(line, constants.Troubleshootv1beta2Kind) {
				indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
				lines[i] = indent + "apiVersion: " + constants.Troubleshootv1beta3Kind
				fixed = true
				break
			}
		}
	}

	// Sort errors by line number (descending) to avoid line number shifts when editing
	errorsByLine := make(map[int][]LintError)
	for _, err := range result.Errors {
		if err.Line > 0 {
			errorsByLine[err.Line] = append(errorsByLine[err.Line], err)
		}
	}

	// Process errors line by line
	for lineNum, errs := range errorsByLine {
		if lineNum > len(lines) {
			continue
		}

		line := lines[lineNum-1]
		originalLine := line

		for _, err := range errs {
			// Fix 1: Add missing colon
			if strings.Contains(err.Message, "could not find expected ':'") {
				if !strings.Contains(line, ":") {
					trimmed := strings.TrimSpace(line)
					indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
					line = indent + trimmed + ":"
					fixed = true
				}
			}

			// Fix 2: Add missing leading dot in template expressions
			if strings.Contains(err.Message, "Template expression may be missing leading dot:") {
				// Extract the expression from the error message
				re := regexp.MustCompile(`Template expression may be missing leading dot: \{\{ (.+?) \}\}`)
				matches := re.FindStringSubmatch(err.Message)
				if len(matches) > 1 {
					badExpr := matches[1]
					// Add the leading dot
					fixedExpr := "." + badExpr
					// Replace in the line
					line = strings.Replace(line, "{{ "+badExpr+" }}", "{{ "+fixedExpr+" }}", 1)
					line = strings.Replace(line, "{{"+badExpr+"}}", "{{"+fixedExpr+"}}", 1)
					line = strings.Replace(line, "{{- "+badExpr+" }}", "{{- "+fixedExpr+" }}", 1)
					line = strings.Replace(line, "{{- "+badExpr+" -}}", "{{- "+fixedExpr+" -}}", 1)
					fixed = true
				}
			}

			// Fix 3: Fix wrong apiVersion
			if strings.Contains(err.Message, "File must contain apiVersion:") && err.Field == "apiVersion" {
				if strings.Contains(line, "apiVersion:") && !strings.Contains(line, constants.Troubleshootv1beta3Kind) {
					// Replace existing apiVersion with correct one
					indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
					line = indent + "apiVersion: " + constants.Troubleshootv1beta3Kind
					fixed = true
				}
			}
		}

		// Update the line if it changed
		if line != originalLine {
			lines[lineNum-1] = line
		}
	}

	// Fix B: Wrap mapping under required list fields (collectors, hostCollectors, analyzers)
	for _, err := range result.Errors {
		if strings.HasPrefix(err.Message, "Expected 'collectors' to be a list") {
			if wrapFirstChildAsList(&lines, "collectors:") {
				fixed = true
			}
		}
		if strings.HasPrefix(err.Message, "Expected 'hostCollectors' to be a list") {
			if wrapFirstChildAsList(&lines, "hostCollectors:") || convertScalarToEmptyList(&lines, "hostCollectors:") {
				fixed = true
			}
		}
		if strings.HasPrefix(err.Message, "Expected 'analyzers' to be a list") {
			if wrapFirstChildAsList(&lines, "analyzers:") {
				fixed = true
			}
		}
	}

	// Fix C: Add missing required fields with empty placeholders (non-assumptive)
	// Collectors
	for _, err := range result.Errors {
		if strings.HasPrefix(err.Message, "Missing required field '") && strings.Contains(err.Message, " for collector '") {
			// Parse field and collector type
			// e.g., Missing required field 'namespace' for collector 'ceph'
			fieldName := between(err.Message, "Missing required field '", "'")
			collectorType := betweenAfter(err.Message, "collector '", "'")
			if fieldName == "" || collectorType == "" {
				continue
			}
			// Only handle simple case where the list item is in {} form: "- type: {}"
			// Find the list item line from current content
			cur := strings.Join(lines, "\n")
			lineNum := findCollectorLine(cur, "collectors", indexFromField(err.Field))
			if lineNum > 0 {
				li := lineNum - 1
				if strings.Contains(lines[li], "- "+collectorType+": {}") {
					indent := lines[li][:len(lines[li])-len(strings.TrimLeft(lines[li], " \t"))]
					childIndent := indent + "    "
					// choose placeholder: outcomes -> [] ; others -> ""
					placeholder := "\"\""
					if fieldName == "outcomes" {
						placeholder = "[]"
					}
					lines[li] = strings.Replace(lines[li], ": {}", ":\n"+childIndent+fieldName+": "+placeholder, 1)
					fixed = true
				} else if strings.Contains(lines[li], "- "+collectorType+":") {
					// Multi-line mapping; insert missing field under this item
					if insertMissingFieldUnderListItem(&lines, li, fieldName) {
						fixed = true
					}
				}
			}
		}
	}
	// Analyzers
	for _, err := range result.Errors {
		if strings.HasPrefix(err.Message, "Missing required field '") && strings.Contains(err.Message, " for analyzer '") {
			fieldName := between(err.Message, "Missing required field '", "'")
			analyzerType := betweenAfter(err.Message, "analyzer '", "'")
			if fieldName == "" || analyzerType == "" {
				continue
			}
			cur := strings.Join(lines, "\n")
			lineNum := findAnalyzerLine(cur, indexFromField(err.Field))
			if lineNum > 0 {
				li := lineNum - 1
				if strings.Contains(lines[li], "- "+analyzerType+": {}") {
					indent := lines[li][:len(lines[li])-len(strings.TrimLeft(lines[li], " \t"))]
					childIndent := indent + "    "
					placeholder := "\"\""
					if fieldName == "outcomes" {
						placeholder = "[]"
					}
					lines[li] = strings.Replace(lines[li], ": {}", ":\n"+childIndent+fieldName+": "+placeholder, 1)
					fixed = true
				} else if strings.Contains(lines[li], "- "+analyzerType+":") {
					if insertMissingFieldUnderListItem(&lines, li, fieldName) {
						fixed = true
					}
				}
			}
		}
	}

	// Return fixed content if changes were made
	if fixed {
		newContent = strings.Join(lines, "\n")
		return newContent, true, nil
	}

	return content, false, nil
}

// wrapFirstChildAsList prefixes the first child mapping line under the given key with '- '
func wrapFirstChildAsList(lines *[]string, key string) bool {
	arr := *lines
	// find key line index
	baseIdx := -1
	for i, l := range arr {
		if strings.Contains(l, key) {
			baseIdx = i
			break
		}
	}
	if baseIdx == -1 {
		return false
	}
	baseIndent := arr[baseIdx][:len(arr[baseIdx])-len(strings.TrimLeft(arr[baseIdx], " \t"))]
	// find first child line with greater indent
	for j := baseIdx + 1; j < len(arr); j++ {
		line := arr[j]
		if strings.TrimSpace(line) == "" {
			continue
		}
		// stop when indentation goes back to or less than base
		if !strings.HasPrefix(line, baseIndent+" ") && !strings.HasPrefix(line, baseIndent+"\t") {
			break
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") {
			// already a list
			return false
		}
		// prefix '- '
		childIndent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		arr[j] = childIndent + "- " + strings.TrimSpace(line)
		*lines = arr
		return true
	}
	return false
}

// convertScalarToEmptyList changes `key: <scalar>` to `key: []` on the same line
func convertScalarToEmptyList(lines *[]string, key string) bool {
	arr := *lines
	for i, l := range arr {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, key) {
			// If already ends with ':' leave for wrapper; else replace value with []
			if strings.HasSuffix(trimmed, ":") {
				return false
			}
			// Replace everything after the first ':' with [] preserving indentation/key
			parts := strings.SplitN(l, ":", 2)
			if len(parts) == 2 {
				arr[i] = parts[0] + ": []"
				*lines = arr
				return true
			}
		}
	}
	return false
}

// indexFromField extracts the numeric index from a path like spec.collectors[1] or spec.analyzers[0]
func indexFromField(field string) int {
	// find [number]
	start := strings.Index(field, "[")
	end := strings.Index(field, "]")
	if start == -1 || end == -1 || end <= start+1 {
		return 0
	}
	numStr := field[start+1 : end]
	// naive parse
	n := 0
	for _, ch := range numStr {
		if ch < '0' || ch > '9' {
			return 0
		}
		n = n*10 + int(ch-'0')
	}
	return n
}

// between extracts substring between prefix and suffix (first occurrences)
func between(s, prefix, suffix string) string {
	i := strings.Index(s, prefix)
	if i == -1 {
		return ""
	}
	s2 := s[i+len(prefix):]
	j := strings.Index(s2, suffix)
	if j == -1 {
		return ""
	}
	return s2[:j]
}

// betweenAfter extracts substring between prefix and suffix starting search after prefix
func betweenAfter(s, prefix, suffix string) string {
	i := strings.Index(s, prefix)
	if i == -1 {
		return ""
	}
	s2 := s[i+len(prefix):]
	j := strings.Index(s2, suffix)
	if j == -1 {
		return ""
	}
	return s2[:j]
}

// insertMissingFieldUnderListItem inserts "fieldName: <placeholder>" as first child under list item at startIdx
// Placeholder is [] for outcomes, "" otherwise. Preserves indentation by using the next child indentation if available
func insertMissingFieldUnderListItem(lines *[]string, startIdx int, fieldName string) bool {
	arr := *lines
	baseLine := arr[startIdx]
	baseIndent := baseLine[:len(baseLine)-len(strings.TrimLeft(baseLine, " \t"))]
	// Determine child indentation: prefer next non-empty line's indent if deeper than base
	childIndent := baseIndent + "  "
	insertPos := startIdx + 1
	for j := startIdx + 1; j < len(arr); j++ {
		if strings.TrimSpace(arr[j]) == "" {
			insertPos = j + 1
			continue
		}
		lineIndent := arr[j][:len(arr[j])-len(strings.TrimLeft(arr[j], " \t"))]
		if len(lineIndent) > len(baseIndent) {
			childIndent = lineIndent
			insertPos = j
		}
		break
	}
	// Choose placeholder
	placeholder := "\"\""
	if fieldName == "outcomes" {
		placeholder = "[]"
	}
	// Insert new line
	newLine := childIndent + fieldName + ": " + placeholder
	// Avoid duplicate insert if the field already exists within this block
	for k := startIdx + 1; k < len(arr); k++ {
		if strings.TrimSpace(arr[k]) == "" {
			continue
		}
		// Stop when block ends (indentation returns to base or less)
		kIndent := arr[k][:len(arr[k])-len(strings.TrimLeft(arr[k], " \t"))]
		if len(kIndent) <= len(baseIndent) {
			break
		}
		if strings.HasPrefix(strings.TrimSpace(arr[k]), fieldName+":") {
			return false
		}
	}
	arr = append(arr[:insertPos], append([]string{newLine}, arr[insertPos:]...)...)
	*lines = arr
	return true
}

func findLineNumber(content, search string) int {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, search) {
			return i + 1
		}
	}
	return 0
}

func findAnalyzerLine(content string, index int) int {
	return findListItemLine(content, "analyzers", index)
}

// findListItemLine locates the starting line of the Nth entry in a list under listKey
func findListItemLine(content, listKey string, index int) int {
	lines := strings.Split(content, "\n")
	count := 0
	inList := false
	for i, line := range lines {
		if strings.Contains(line, listKey+":") {
			inList = true
			continue
		}
		if inList && strings.HasPrefix(strings.TrimSpace(line), "- ") {
			if count == index {
				return i + 1
			}
			count++
		}
		if inList && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && strings.TrimSpace(line) != "" {
			break
		}
	}
	return 0
}

func extractLineFromError(err error) int {
	// Try to extract line number from YAML error message
	re := regexp.MustCompile(`line (\d+)`)
	matches := re.FindStringSubmatch(err.Error())
	if len(matches) > 1 {
		var line int
		fmt.Sscanf(matches[1], "%d", &line)
		return line
	}
	return 0
}

// extractTemplateBetweenBraces extracts template expressions from a line
func extractTemplateBetweenBraces(line string) []string {
	var expressions []string
	// Match {{ ... }} with optional whitespace trimming (-), including comments {{/* */}}
	re := regexp.MustCompile(`\{\{-?\s*(.+?)\s*-?\}\}`)
	matches := re.FindAllStringSubmatch(line, -1)
	for _, match := range matches {
		if len(match) > 1 {
			// Clean up the expression
			expr := match[1]
			// Remove */ at the end if it's part of a comment
			expr = strings.TrimSuffix(strings.TrimSpace(expr), "*/")
			expressions = append(expressions, expr)
		}
	}
	return expressions
}

// isControlStructure checks if a template expression is a control structure
func isControlStructure(expr string) bool {
	trimmed := strings.TrimSpace(expr)
	controlKeywords := []string{"if", "else", "end", "range", "with", "define", "template", "block", "include"}
	for _, keyword := range controlKeywords {
		if strings.HasPrefix(trimmed, keyword+" ") || trimmed == keyword {
			return true
		}
	}
	return false
}

// FormatResults formats lint results for output
func FormatResults(results []LintResult, format string) string {
	if format == "json" {
		return formatJSON(results)
	}
	return formatText(results)
}

func formatText(results []LintResult) string {
	var output strings.Builder
	totalErrors := 0
	totalWarnings := 0

	for _, result := range results {
		if len(result.Errors) == 0 && len(result.Warnings) == 0 {
			output.WriteString(fmt.Sprintf("✓ %s: No issues found\n", result.FilePath))
			continue
		}

		output.WriteString(fmt.Sprintf("\n%s:\n", result.FilePath))

		for _, err := range result.Errors {
			output.WriteString(fmt.Sprintf("  ✗ Error (line %d): %s\n", err.Line, err.Message))
			if err.Field != "" {
				output.WriteString(fmt.Sprintf("    Field: %s\n", err.Field))
			}
			totalErrors++
		}

		for _, warn := range result.Warnings {
			output.WriteString(fmt.Sprintf("  ⚠ Warning (line %d): %s\n", warn.Line, warn.Message))
			if warn.Field != "" {
				output.WriteString(fmt.Sprintf("    Field: %s\n", warn.Field))
			}
			totalWarnings++
		}
	}

	output.WriteString(fmt.Sprintf("\nSummary: %d error(s), %d warning(s) across %d file(s)\n", totalErrors, totalWarnings, len(results)))

	return output.String()
}

func formatJSON(results []LintResult) string {
	// Simple JSON formatting without importing encoding/json
	var output strings.Builder
	output.WriteString("{\n")
	output.WriteString("  \"results\": [\n")

	for i, result := range results {
		output.WriteString("    {\n")
		output.WriteString(fmt.Sprintf("      \"filePath\": %q,\n", result.FilePath))
		output.WriteString("      \"errors\": [\n")

		for j, err := range result.Errors {
			output.WriteString("        {\n")
			output.WriteString(fmt.Sprintf("          \"line\": %d,\n", err.Line))
			output.WriteString(fmt.Sprintf("          \"column\": %d,\n", err.Column))
			output.WriteString(fmt.Sprintf("          \"message\": %q,\n", err.Message))
			output.WriteString(fmt.Sprintf("          \"field\": %q\n", err.Field))
			output.WriteString("        }")
			if j < len(result.Errors)-1 {
				output.WriteString(",")
			}
			output.WriteString("\n")
		}

		output.WriteString("      ],\n")
		output.WriteString("      \"warnings\": [\n")

		for j, warn := range result.Warnings {
			output.WriteString("        {\n")
			output.WriteString(fmt.Sprintf("          \"line\": %d,\n", warn.Line))
			output.WriteString(fmt.Sprintf("          \"column\": %d,\n", warn.Column))
			output.WriteString(fmt.Sprintf("          \"message\": %q,\n", warn.Message))
			output.WriteString(fmt.Sprintf("          \"field\": %q\n", warn.Field))
			output.WriteString("        }")
			if j < len(result.Warnings)-1 {
				output.WriteString(",")
			}
			output.WriteString("\n")
		}

		output.WriteString("      ]\n")
		output.WriteString("    }")
		if i < len(results)-1 {
			output.WriteString(",")
		}
		output.WriteString("\n")
	}

	output.WriteString("  ]\n")
	output.WriteString("}\n")

	return output.String()
}

// HasErrors returns true if any of the results contain errors
func HasErrors(results []LintResult) bool {
	for _, result := range results {
		if len(result.Errors) > 0 {
			return true
		}
	}
	return false
}
