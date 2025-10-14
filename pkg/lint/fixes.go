package lint

import (
	"regexp"
	"strings"

	"github.com/replicatedhq/troubleshoot/pkg/constants"
)

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

// insertMissingFieldUnderListItem inserts "fieldName: <placeholder>" under list item at startIdx
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
