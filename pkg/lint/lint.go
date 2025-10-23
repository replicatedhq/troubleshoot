package lint

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/internal/util"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/preflight"
	"helm.sh/helm/v3/pkg/strvals"
	"sigs.k8s.io/yaml"
)

// LintFiles validates troubleshoot specs for syntax and structural errors
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

		// Check if this is a v1beta3 spec
		isV1Beta3 := detectAPIVersionFromContent(fileContent) == constants.Troubleshootv1beta3Kind
		isV1Beta2 := detectAPIVersionFromContent(fileContent) == constants.Troubleshootv1beta2Kind

		// Check if the content has Helm templates (for preflight v1beta3)
		hasTemplates := strings.Contains(fileContent, "{{") && strings.Contains(fileContent, "}}")

		// Track if we should add a warning about unused values for v1beta2
		hasUnusedValuesWarning := isV1Beta2 && (len(opts.ValuesFiles) > 0 || len(opts.SetValues) > 0)

		// If v1beta3 with templates, require values and render the template
		if isV1Beta3 && hasTemplates {
			if len(opts.ValuesFiles) == 0 && len(opts.SetValues) == 0 {
				return nil, errors.New("v1beta3 specs with Helm templates require a values file. Please provide values using --values or --set flags")
			}

			// Load values from files and --set flags
			values := make(map[string]interface{})
			for _, valuesFile := range opts.ValuesFiles {
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

				values = preflight.MergeMaps(values, fileValues)
			}

			// Apply --set values
			for _, setValue := range opts.SetValues {
				if err := strvals.ParseInto(setValue, values); err != nil {
					return nil, errors.Wrapf(err, "failed to parse --set value: %s", setValue)
				}
			}

			// Render the template
			preflight.SeedDefaultBooleans(fileContent, values)
			preflight.SeedParentMapsForValueRefs(fileContent, values)
			rendered, err := preflight.RenderWithHelmTemplate(fileContent, values)
			if err != nil {
				// If rendering fails, create a result with the render error
				// This allows us to report template syntax errors
				results = append(results, LintResult{
					FilePath: filePath,
					Errors: []LintError{
						{
							Line:    1,
							Message: fmt.Sprintf("Failed to render v1beta3 template: %v", err),
							Field:   "template",
						},
					},
				})
				continue
			}

			// Use the rendered content for linting
			fileContent = rendered
		}

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

		// Add warning if values were provided for a v1beta2 spec
		if hasUnusedValuesWarning {
			fileResult.Warnings = append([]LintWarning{
				{
					Line:    1,
					Message: "Values files provided but this is a v1beta2 spec. Values are only used with v1beta3 specs. Did you mean to use apiVersion: troubleshoot.sh/v1beta3?",
					Field:   "apiVersion",
				},
			}, fileResult.Warnings...)
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
