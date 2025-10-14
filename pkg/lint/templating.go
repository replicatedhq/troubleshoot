package lint

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

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
