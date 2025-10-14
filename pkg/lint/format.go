package lint

import (
	"fmt"
	"strings"
)

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
