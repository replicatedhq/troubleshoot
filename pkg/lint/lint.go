package lint

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/pkg/errors"
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

	for _, filePath := range opts.FilePaths {
		result, err := lintFile(filePath, opts.Fix)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	return results, nil
}

func lintFile(filePath string, fix bool) (LintResult, error) {
	result := LintResult{
		FilePath: filePath,
		Errors:   []LintError{},
		Warnings: []LintWarning{},
	}

	// Read file
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return result, errors.Wrapf(err, "failed to read file %s", filePath)
	}

	// Check for v1beta3 apiVersion
	if !strings.Contains(string(content), constants.Troubleshootv1beta3Kind) {
		result.Errors = append(result.Errors, LintError{
			Line:    1,
			Message: fmt.Sprintf("File must contain apiVersion: %s", constants.Troubleshootv1beta3Kind),
			Field:   "apiVersion",
		})
		// Try to fix wrong apiVersion
		if fix {
			fixed, err := applyFixes(filePath, string(content), result)
			if err != nil {
				return result, err
			}
			if fixed {
				// Re-lint to verify fixes
				return lintFile(filePath, false)
			}
		}
		return result, nil
	}

	// Check if file contains template expressions
	hasTemplates := strings.Contains(string(content), "{{") && strings.Contains(string(content), "}}")

	// Validate YAML syntax (but be lenient with templated files)
	var parsed map[string]interface{}
	yamlParseErr := yaml.Unmarshal(content, &parsed)
	if yamlParseErr != nil {
		// If the file has templates, YAML parsing may fail - that's expected
		// We'll still try to validate what we can
		if !hasTemplates {
			result.Errors = append(result.Errors, LintError{
				Line:    extractLineFromError(yamlParseErr),
				Message: fmt.Sprintf("YAML syntax error: %s", yamlParseErr.Error()),
			})
			// Don't return yet - we want to try to fix this error
			// Continue to applyFixes at the end
			if fix {
				fixed, err := applyFixes(filePath, string(content), result)
				if err != nil {
					return result, err
				}
				if fixed {
					// Re-lint to verify fixes
					return lintFile(filePath, false)
				}
			}
			return result, nil
		}
		// For templated files, we can't parse YAML strictly, so just check template syntax
		result.Errors = append(result.Errors, checkTemplateSyntax(string(content))...)
		// Continue to applyFixes for templates too
		if fix {
			fixed, err := applyFixes(filePath, string(content), result)
			if err != nil {
				return result, err
			}
			if fixed {
				// Re-lint to verify fixes
				return lintFile(filePath, false)
			}
		}
		return result, nil
	}

	// Check required fields
	result.Errors = append(result.Errors, checkRequiredFields(parsed, string(content))...)

	// Check template syntax
	result.Errors = append(result.Errors, checkTemplateSyntax(string(content))...)

	// Check for kind-specific requirements
	if kind, ok := parsed["kind"].(string); ok {
		switch kind {
		case "Preflight":
			result.Errors = append(result.Errors, checkPreflightSpec(parsed, string(content))...)
		case "SupportBundle":
			result.Errors = append(result.Errors, checkSupportBundleSpec(parsed, string(content))...)
		}
	}

	// Check for common issues
	result.Warnings = append(result.Warnings, checkCommonIssues(parsed, string(content))...)

	// Apply fixes if requested
	if fix && (len(result.Errors) > 0 || len(result.Warnings) > 0) {
		fixed, err := applyFixes(filePath, string(content), result)
		if err != nil {
			return result, err
		}
		if fixed {
			// Re-lint to verify fixes
			return lintFile(filePath, false)
		}
	}

	return result, nil
}

func checkRequiredFields(parsed map[string]interface{}, content string) []LintError {
	errors := []LintError{}

	// Check apiVersion
	if apiVersion, ok := parsed["apiVersion"].(string); !ok || apiVersion == "" {
		errors = append(errors, LintError{
			Line:    findLineNumber(content, "apiVersion"),
			Field:   "apiVersion",
			Message: "Missing or empty 'apiVersion' field",
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
			Message: fmt.Sprintf("Invalid kind '%s'. Must be 'Preflight' or 'SupportBundle'", kind),
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
				Message: "Missing or empty 'metadata.name' field",
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

func checkTemplateSyntax(content string) []LintError {
	errors := []LintError{}
	lines := strings.Split(content, "\n")

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

				// Skip control structures (if, else, end, range, with, etc.)
				if isControlStructure(trimmed) {
					continue
				}

				// Skip comments: {{/* ... */}}
				if strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*/") {
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

func checkCommonIssues(parsed map[string]interface{}, content string) []LintWarning {
	warnings := []LintWarning{}

	// Check for missing docStrings in analyzers
	spec, ok := parsed["spec"].(map[string]interface{})
	if !ok {
		return warnings
	}

	if analyzers, ok := spec["analyzers"].([]interface{}); ok {
		for i, analyzer := range analyzers {
			if analyzerMap, ok := analyzer.(map[string]interface{}); ok {
				if _, hasDocString := analyzerMap["docString"]; !hasDocString {
					warnings = append(warnings, LintWarning{
						Line:    findAnalyzerLine(content, i),
						Field:   fmt.Sprintf("spec.analyzers[%d].docString", i),
						Message: "Analyzer missing docString (recommended for v1beta3)",
					})
				}
			}
		}
	}

	return warnings
}

func applyFixes(filePath, content string, result LintResult) (bool, error) {
	fixed := false
	newContent := content
	lines := strings.Split(newContent, "\n")

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
			// YAML parsers often report the error on the line AFTER the actual problem
			if strings.Contains(err.Message, "could not find expected ':'") {
				// Check current line first
				if !strings.Contains(line, ":") {
					trimmed := strings.TrimSpace(line)
					indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
					line = indent + trimmed + ":"
					fixed = true
				} else if lineNum > 1 {
					// Check previous line (where the colon is likely missing)
					prevLine := lines[lineNum-2]
					if !strings.Contains(prevLine, ":") && strings.TrimSpace(prevLine) != "" {
						trimmed := strings.TrimSpace(prevLine)
						indent := prevLine[:len(prevLine)-len(strings.TrimLeft(prevLine, " \t"))]
						lines[lineNum-2] = indent + trimmed + ":"
						fixed = true
					}
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

	// Fix 4: Add missing required top-level fields
	for _, err := range result.Errors {
		if err.Field == "apiVersion" && strings.Contains(err.Message, "Missing or empty 'apiVersion'") {
			// Add apiVersion at the beginning
			lines = append([]string{"apiVersion: " + constants.Troubleshootv1beta3Kind}, lines...)
			fixed = true
		} else if err.Field == "kind" && strings.Contains(err.Message, "Missing or empty 'kind'") {
			// Try to determine if it should be Preflight or SupportBundle based on filename
			kind := "Preflight"
			if strings.Contains(strings.ToLower(filePath), "bundle") {
				kind = "SupportBundle"
			}
			// Add kind after apiVersion
			insertIndex := 0
			for i, line := range lines {
				if strings.Contains(line, "apiVersion:") {
					insertIndex = i + 1
					break
				}
			}
			newLines := make([]string, 0, len(lines)+1)
			newLines = append(newLines, lines[:insertIndex]...)
			newLines = append(newLines, "kind: "+kind)
			newLines = append(newLines, lines[insertIndex:]...)
			lines = newLines
			fixed = true
		} else if err.Field == "metadata" && strings.Contains(err.Message, "Missing 'metadata'") {
			// Add metadata section after kind
			insertIndex := 0
			for i, line := range lines {
				if strings.Contains(line, "kind:") {
					insertIndex = i + 1
					break
				}
			}
			newLines := make([]string, 0, len(lines)+2)
			newLines = append(newLines, lines[:insertIndex]...)
			newLines = append(newLines, "metadata:")
			newLines = append(newLines, "  name: my-spec")
			newLines = append(newLines, lines[insertIndex:]...)
			lines = newLines
			fixed = true
		}
	}

	// Write fixed content back to file if changes were made
	if fixed {
		newContent = strings.Join(lines, "\n")
		if err := ioutil.WriteFile(filePath, []byte(newContent), 0644); err != nil {
			return false, errors.Wrapf(err, "failed to write fixed content to %s", filePath)
		}
	}

	return fixed, nil
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
	lines := strings.Split(content, "\n")
	analyzerCount := 0
	inAnalyzers := false

	for i, line := range lines {
		if strings.Contains(line, "analyzers:") {
			inAnalyzers = true
			continue
		}
		if inAnalyzers && strings.HasPrefix(strings.TrimSpace(line), "- ") {
			if analyzerCount == index {
				return i + 1
			}
			analyzerCount++
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
