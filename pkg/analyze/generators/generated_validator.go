package generators

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// GeneratedAnalyzerValidator validates generated analyzer code
type GeneratedAnalyzerValidator struct {
	rules []AnalyzerValidationRule
}

// AnalyzerValidationRule defines validation rules for generated analyzers
type AnalyzerValidationRule struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	Check       AnalyzerValidationCheck    `json:"-"`
	Severity    AnalyzerValidationSeverity `json:"severity"`
	Category    string                     `json:"category"`
}

// AnalyzerValidationCheck is a function that validates an analyzer
type AnalyzerValidationCheck func(analyzer GeneratedAnalyzer) []AnalyzerValidationResult

// AnalyzerValidationSeverity defines the severity of validation issues
type AnalyzerValidationSeverity string

const (
	AnalyzerSeverityError   AnalyzerValidationSeverity = "error"
	AnalyzerSeverityWarning AnalyzerValidationSeverity = "warning"
	AnalyzerSeverityInfo    AnalyzerValidationSeverity = "info"
)

// AnalyzerValidationResult represents the result of a validation check
type AnalyzerValidationResult struct {
	Rule     string                     `json:"rule"`
	Severity AnalyzerValidationSeverity `json:"severity"`
	Message  string                     `json:"message"`
	Line     int                        `json:"line,omitempty"`
	Column   int                        `json:"column,omitempty"`
	Fix      string                     `json:"fix,omitempty"`
	Category string                     `json:"category"`
}

// CodeQualityMetrics represents code quality metrics for generated analyzers
type CodeQualityMetrics struct {
	LinesOfCode          int     `json:"lines_of_code"`
	CyclomaticComplexity int     `json:"cyclomatic_complexity"`
	TestCoverage         float64 `json:"test_coverage"`
	DocumentationRatio   float64 `json:"documentation_ratio"`
	ImportCount          int     `json:"import_count"`
	FunctionCount        int     `json:"function_count"`
	StructCount          int     `json:"struct_count"`
}

// NewGeneratedAnalyzerValidator creates a new generated analyzer validator
func NewGeneratedAnalyzerValidator() *GeneratedAnalyzerValidator {
	return &GeneratedAnalyzerValidator{
		rules: getDefaultAnalyzerValidationRules(),
	}
}

// Validate validates a generated analyzer
func (v *GeneratedAnalyzerValidator) Validate(analyzer GeneratedAnalyzer) ([]ValidationError, []ValidationWarning) {
	var errors []ValidationError
	var warnings []ValidationWarning

	// Run all validation rules
	for _, rule := range v.rules {
		results := rule.Check(analyzer)

		for _, result := range results {
			switch result.Severity {
			case AnalyzerSeverityError:
				errors = append(errors, ValidationError{
					Path:     analyzer.Name,
					Field:    result.Category,
					Rule:     result.Rule,
					Message:  result.Message,
					Severity: SeverityError,
				})
			case AnalyzerSeverityWarning:
				warnings = append(warnings, ValidationWarning{
					Path:       analyzer.Name,
					Field:      result.Category,
					Rule:       result.Rule,
					Message:    result.Message,
					Suggestion: result.Fix,
				})
			}
		}
	}

	return errors, warnings
}

// ValidateAndGetMetrics validates analyzer and returns quality metrics
func (v *GeneratedAnalyzerValidator) ValidateAndGetMetrics(analyzer GeneratedAnalyzer) ([]ValidationError, []ValidationWarning, CodeQualityMetrics) {
	errors, warnings := v.Validate(analyzer)
	metrics := v.calculateMetrics(analyzer)

	return errors, warnings, metrics
}

// AddValidationRule adds a custom validation rule
func (v *GeneratedAnalyzerValidator) AddValidationRule(rule AnalyzerValidationRule) {
	v.rules = append(v.rules, rule)
}

// calculateMetrics calculates code quality metrics
func (v *GeneratedAnalyzerValidator) calculateMetrics(analyzer GeneratedAnalyzer) CodeQualityMetrics {
	metrics := CodeQualityMetrics{}

	// Parse the source code
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, analyzer.Name+".go", analyzer.Source, parser.ParseComments)
	if err != nil {
		return metrics
	}

	// Calculate basic metrics
	metrics.LinesOfCode = len(strings.Split(analyzer.Source, "\n"))
	metrics.ImportCount = len(node.Imports)

	// Count functions and structs
	ast.Inspect(node, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.FuncDecl:
			metrics.FunctionCount++
		case *ast.StructType:
			metrics.StructCount++
		}
		return true
	})

	// Calculate documentation ratio (comments vs code)
	commentLines := 0
	if node.Comments != nil {
		for _, commentGroup := range node.Comments {
			commentLines += len(commentGroup.List)
		}
	}

	if metrics.LinesOfCode > 0 {
		metrics.DocumentationRatio = float64(commentLines) / float64(metrics.LinesOfCode)
	}

	// Calculate test coverage if test source exists
	if analyzer.TestSource != "" {
		testLines := len(strings.Split(analyzer.TestSource, "\n"))
		if metrics.LinesOfCode > 0 {
			metrics.TestCoverage = float64(testLines) / float64(metrics.LinesOfCode)
		}
	}

	return metrics
}

// Validation check functions

func checkSyntaxValidity(analyzer GeneratedAnalyzer) []AnalyzerValidationResult {
	var results []AnalyzerValidationResult

	// Parse the source code to check syntax
	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, analyzer.Name+".go", analyzer.Source, 0)
	if err != nil {
		results = append(results, AnalyzerValidationResult{
			Rule:     "syntax_validity",
			Severity: AnalyzerSeverityError,
			Message:  fmt.Sprintf("Syntax error: %v", err),
			Category: "syntax",
		})
	}

	return results
}

func checkRequiredMethods(analyzer GeneratedAnalyzer) []AnalyzerValidationResult {
	var results []AnalyzerValidationResult

	requiredMethods := []string{"Analyze", "Name", "Version"}

	for _, method := range requiredMethods {
		if !strings.Contains(analyzer.Source, fmt.Sprintf("func (%s)", method)) &&
			!strings.Contains(analyzer.Source, fmt.Sprintf("func %s(", method)) {
			results = append(results, AnalyzerValidationResult{
				Rule:     "required_methods",
				Severity: AnalyzerSeverityError,
				Message:  fmt.Sprintf("Required method '%s' is missing", method),
				Category: "structure",
				Fix:      fmt.Sprintf("Add the %s method to the analyzer", method),
			})
		}
	}

	return results
}

func checkNamingConventions(analyzer GeneratedAnalyzer) []AnalyzerValidationResult {
	var results []AnalyzerValidationResult

	// Check if analyzer name follows conventions
	if !strings.HasSuffix(analyzer.Name, "Analyzer") {
		results = append(results, AnalyzerValidationResult{
			Rule:     "naming_conventions",
			Severity: AnalyzerSeverityWarning,
			Message:  "Analyzer name should end with 'Analyzer'",
			Category: "naming",
			Fix:      "Rename to " + analyzer.Name + "Analyzer",
		})
	}

	// Check if name uses PascalCase
	if !isPascalCase(analyzer.Name) {
		results = append(results, AnalyzerValidationResult{
			Rule:     "naming_conventions",
			Severity: AnalyzerSeverityWarning,
			Message:  "Analyzer name should use PascalCase",
			Category: "naming",
		})
	}

	return results
}

func checkDocumentation(analyzer GeneratedAnalyzer) []AnalyzerValidationResult {
	var results []AnalyzerValidationResult

	// Check if analyzer has description
	if analyzer.Description == "" {
		results = append(results, AnalyzerValidationResult{
			Rule:     "documentation",
			Severity: AnalyzerSeverityWarning,
			Message:  "Analyzer should have a description",
			Category: "documentation",
			Fix:      "Add a description to the analyzer metadata",
		})
	}

	// Check if main struct has documentation
	lines := strings.Split(analyzer.Source, "\n")
	hasDocComment := false

	for i, line := range lines {
		if strings.Contains(line, "type") && strings.Contains(line, "struct") {
			// Check if previous line is a comment
			if i > 0 && (strings.HasPrefix(strings.TrimSpace(lines[i-1]), "//") ||
				strings.HasPrefix(strings.TrimSpace(lines[i-1]), "/*")) {
				hasDocComment = true
			}
			break
		}
	}

	if !hasDocComment {
		results = append(results, AnalyzerValidationResult{
			Rule:     "documentation",
			Severity: AnalyzerSeverityWarning,
			Message:  "Main struct should have documentation comment",
			Category: "documentation",
			Fix:      "Add a comment above the struct definition",
		})
	}

	return results
}

func checkErrorHandling(analyzer GeneratedAnalyzer) []AnalyzerValidationResult {
	var results []AnalyzerValidationResult

	// Check if error handling is present
	if !strings.Contains(analyzer.Source, "error") {
		results = append(results, AnalyzerValidationResult{
			Rule:     "error_handling",
			Severity: AnalyzerSeverityWarning,
			Message:  "Analyzer should include proper error handling",
			Category: "robustness",
			Fix:      "Add error handling to analyzer methods",
		})
	}

	// Check for proper error checking patterns
	if strings.Contains(analyzer.Source, "err != nil") {
		// Good - has error checking
	} else if strings.Contains(analyzer.Source, "error") {
		results = append(results, AnalyzerValidationResult{
			Rule:     "error_handling",
			Severity: AnalyzerSeverityWarning,
			Message:  "Error handling present but may not be checking errors properly",
			Category: "robustness",
			Fix:      "Ensure all errors are properly checked with 'if err != nil'",
		})
	}

	return results
}

func checkTestCoverage(analyzer GeneratedAnalyzer) []AnalyzerValidationResult {
	var results []AnalyzerValidationResult

	if analyzer.TestSource == "" {
		results = append(results, AnalyzerValidationResult{
			Rule:     "test_coverage",
			Severity: AnalyzerSeverityWarning,
			Message:  "Analyzer should have test coverage",
			Category: "testing",
			Fix:      "Generate test cases for the analyzer",
		})
	} else {
		// Check if tests are comprehensive
		testLines := len(strings.Split(analyzer.TestSource, "\n"))
		sourceLines := len(strings.Split(analyzer.Source, "\n"))

		ratio := float64(testLines) / float64(sourceLines)
		if ratio < 0.3 { // Less than 30% test to source ratio
			results = append(results, AnalyzerValidationResult{
				Rule:     "test_coverage",
				Severity: AnalyzerSeverityInfo,
				Message:  "Test coverage appears low - consider adding more test cases",
				Category: "testing",
				Fix:      "Add more comprehensive test cases",
			})
		}
	}

	return results
}

func checkComplexity(analyzer GeneratedAnalyzer) []AnalyzerValidationResult {
	var results []AnalyzerValidationResult

	// Simple complexity check - count nested if statements and loops
	lines := strings.Split(analyzer.Source, "\n")
	maxNestingLevel := 0
	currentNesting := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Count opening braces that increase nesting
		if strings.Contains(trimmed, "if ") ||
			strings.Contains(trimmed, "for ") ||
			strings.Contains(trimmed, "switch ") {
			currentNesting++
			if currentNesting > maxNestingLevel {
				maxNestingLevel = currentNesting
			}
		}

		// Count closing braces
		if trimmed == "}" {
			currentNesting--
			if currentNesting < 0 {
				currentNesting = 0
			}
		}
	}

	if maxNestingLevel > 4 {
		results = append(results, AnalyzerValidationResult{
			Rule:     "complexity",
			Severity: AnalyzerSeverityWarning,
			Message:  "Code has high complexity (deep nesting)",
			Category: "maintainability",
			Fix:      "Consider refactoring to reduce nesting levels",
		})
	}

	return results
}

func checkRequirementCoverage(analyzer GeneratedAnalyzer) []AnalyzerValidationResult {
	var results []AnalyzerValidationResult

	// Check if all requirements are addressed in the code
	for _, req := range analyzer.Requirements {
		// Simple check - see if requirement path or keywords appear in code
		found := false

		for _, keyword := range req.Keywords {
			if strings.Contains(strings.ToLower(analyzer.Source), strings.ToLower(keyword)) {
				found = true
				break
			}
		}

		if !found {
			results = append(results, AnalyzerValidationResult{
				Rule:     "requirement_coverage",
				Severity: AnalyzerSeverityWarning,
				Message:  fmt.Sprintf("Requirement '%s' may not be addressed in the analyzer", req.Path),
				Category: "completeness",
				Fix:      fmt.Sprintf("Ensure the analyzer checks for requirement: %s", req.Path),
			})
		}
	}

	return results
}

// Helper functions

func isPascalCase(s string) bool {
	if len(s) == 0 {
		return false
	}

	// First character should be uppercase
	if s[0] < 'A' || s[0] > 'Z' {
		return false
	}

	// No underscores or hyphens in PascalCase
	if strings.Contains(s, "_") || strings.Contains(s, "-") {
		return false
	}

	return true
}

// getDefaultAnalyzerValidationRules returns default validation rules
func getDefaultAnalyzerValidationRules() []AnalyzerValidationRule {
	return []AnalyzerValidationRule{
		{
			Name:        "syntax_validity",
			Description: "Check that generated code has valid Go syntax",
			Check:       checkSyntaxValidity,
			Severity:    AnalyzerSeverityError,
			Category:    "syntax",
		},
		{
			Name:        "required_methods",
			Description: "Check that analyzer implements required methods",
			Check:       checkRequiredMethods,
			Severity:    AnalyzerSeverityError,
			Category:    "structure",
		},
		{
			Name:        "naming_conventions",
			Description: "Check that naming follows Go conventions",
			Check:       checkNamingConventions,
			Severity:    AnalyzerSeverityWarning,
			Category:    "naming",
		},
		{
			Name:        "documentation",
			Description: "Check that analyzer has proper documentation",
			Check:       checkDocumentation,
			Severity:    AnalyzerSeverityWarning,
			Category:    "documentation",
		},
		{
			Name:        "error_handling",
			Description: "Check that analyzer has proper error handling",
			Check:       checkErrorHandling,
			Severity:    AnalyzerSeverityWarning,
			Category:    "robustness",
		},
		{
			Name:        "test_coverage",
			Description: "Check that analyzer has adequate test coverage",
			Check:       checkTestCoverage,
			Severity:    AnalyzerSeverityWarning,
			Category:    "testing",
		},
		{
			Name:        "complexity",
			Description: "Check that analyzer code is not overly complex",
			Check:       checkComplexity,
			Severity:    AnalyzerSeverityWarning,
			Category:    "maintainability",
		},
		{
			Name:        "requirement_coverage",
			Description: "Check that all requirements are addressed",
			Check:       checkRequirementCoverage,
			Severity:    AnalyzerSeverityWarning,
			Category:    "completeness",
		},
	}
}
