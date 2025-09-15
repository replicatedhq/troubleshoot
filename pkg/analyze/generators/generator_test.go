package generators

import (
	"testing"
)

// TestNewAnalyzerGenerator tests creating a new analyzer generator
func TestNewAnalyzerGenerator(t *testing.T) {
	generator := NewAnalyzerGenerator()

	if generator == nil {
		t.Fatal("expected non-nil generator")
	}

	if generator.templates == nil {
		t.Fatal("expected templates to be initialized")
	}

	if generator.ruleEngine == nil {
		t.Fatal("expected rule engine to be initialized")
	}

	if generator.validator == nil {
		t.Fatal("expected validator to be initialized")
	}

	if generator.categories == nil {
		t.Fatal("expected categories to be initialized")
	}
}

// TestGenerateAnalyzers tests basic analyzer generation
func TestGenerateAnalyzers(t *testing.T) {
	generator := NewAnalyzerGenerator()

	// Create test requirements
	requirements := []CategorizedRequirement{
		{
			Path:     "kubernetes.minVersion",
			Category: CategoryKubernetes,
			Priority: PriorityRequired,
			Tags:     []string{"version", "kubernetes"},
			Keywords: []string{"kubernetes", "version"},
			Value:    "1.20.0",
		},
		{
			Path:     "resources.cpu.minCores",
			Category: CategoryResources,
			Priority: PriorityRequired,
			Tags:     []string{"cpu", "resources"},
			Keywords: []string{"cpu", "cores"},
			Value:    2,
		},
	}

	opts := GenerationOptions{
		PackageName:    "test",
		GenerateTests:  true,
		ValidateOutput: false, // Skip validation for this test
		FormatCode:     false, // Skip formatting for this test
	}

	result, err := generator.GenerateAnalyzers(requirements, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Should generate at least one analyzer
	if len(result.GeneratedAnalyzers) == 0 {
		t.Fatal("expected at least one generated analyzer")
	}

	// Check that summary is populated
	if result.Summary.GeneratedAnalyzers != len(result.GeneratedAnalyzers) {
		t.Errorf("expected summary count %d, got %d",
			len(result.GeneratedAnalyzers), result.Summary.GeneratedAnalyzers)
	}
}

// TestAddTemplate tests adding custom templates
func TestAddTemplate(t *testing.T) {
	generator := NewAnalyzerGenerator()

	template := AnalyzerTemplate{
		Name:        "test-analyzer",
		Description: "Test analyzer template",
		Type:        AnalyzerTypeCustom,
		Template:    "package {{.PackageName}}\n// Test template",
		Variables:   map[string]string{"test": "value"},
		Imports:     []string{"fmt"},
		Methods:     []string{"testMethod"},
	}

	err := generator.AddTemplate(AnalyzerTypeCustom, template)
	if err != nil {
		t.Fatalf("unexpected error adding template: %v", err)
	}

	// Verify template was added
	if _, exists := generator.templates[AnalyzerTypeCustom]; !exists {
		t.Fatal("expected template to be added")
	}

	if _, exists := generator.categories[RequirementCategory(AnalyzerTypeCustom)]; !exists {
		t.Fatal("expected category to be added")
	}
}

// TestTemplateExecution tests template execution
func TestTemplateExecution(t *testing.T) {
	generator := NewAnalyzerGenerator()

	templateStr := `package {{.PackageName}}

type {{.Name}} struct {}

func (a *{{.Name}}) Title() string {
	return "{{.Description}}"
}`

	vars := map[string]interface{}{
		"PackageName": "test",
		"Name":        "TestAnalyzer",
		"Description": "Test analyzer for testing",
	}

	result, err := generator.executeTemplate(templateStr, vars)
	if err != nil {
		t.Fatalf("unexpected error executing template: %v", err)
	}

	expectedContent := []string{
		"package test",
		"type TestAnalyzer struct {}",
		`return "Test analyzer for testing"`,
	}

	for _, expected := range expectedContent {
		if !contains(result, expected) {
			t.Errorf("expected result to contain %q, got: %s", expected, result)
		}
	}
}

// TestRequirementGrouping tests requirement grouping logic
func TestRequirementGrouping(t *testing.T) {
	generator := NewAnalyzerGenerator()

	requirements := []CategorizedRequirement{
		{
			Path:     "kubernetes.minVersion",
			Category: CategoryKubernetes,
			Priority: PriorityRequired,
		},
		{
			Path:     "kubernetes.maxVersion",
			Category: CategoryKubernetes,
			Priority: PriorityRequired,
		},
		{
			Path:     "resources.cpu.minCores",
			Category: CategoryResources,
			Priority: PriorityRequired,
		},
		{
			Path:     "resources.memory.minBytes",
			Category: CategoryResources,
			Priority: PriorityRequired,
		},
	}

	grouped := generator.groupRequirementsByCategory(requirements)

	if len(grouped[CategoryKubernetes]) != 2 {
		t.Errorf("expected 2 Kubernetes requirements, got %d",
			len(grouped[CategoryKubernetes]))
	}

	if len(grouped[CategoryResources]) != 2 {
		t.Errorf("expected 2 resource requirements, got %d",
			len(grouped[CategoryResources]))
	}
}

// TestTemplateVariablePreparation tests template variable preparation
func TestTemplateVariablePreparation(t *testing.T) {
	generator := NewAnalyzerGenerator()

	spec := AnalyzerSpec{
		Name:        "TestAnalyzer",
		Description: "Test analyzer",
		Category:    CategoryKubernetes,
		Requirements: []CategorizedRequirement{
			{
				Path:     "kubernetes.minVersion",
				Priority: PriorityRequired,
				Value:    "1.20.0",
			},
		},
		Tags: []string{"test", "kubernetes"},
	}

	template := AnalyzerTemplate{
		Variables: map[string]string{
			"TemplateVar": "template_value",
		},
	}

	opts := GenerationOptions{
		PackageName: "testpkg",
		CustomVariables: map[string]string{
			"CustomVar": "custom_value",
		},
	}

	vars := generator.prepareTemplateVariables(spec, template, opts)

	expectedVars := map[string]interface{}{
		"Name":        "TestAnalyzer",
		"Description": "Test analyzer",
		"PackageName": "testpkg",
		"Type":        "kubernetes",
		"TemplateVar": "template_value",
		"CustomVar":   "custom_value",
	}

	for key, expectedValue := range expectedVars {
		if actualValue, exists := vars[key]; !exists {
			t.Errorf("expected variable %q to exist", key)
		} else if actualValue != expectedValue {
			t.Errorf("expected %q = %v, got %v", key, expectedValue, actualValue)
		}
	}

	// Test dynamic variables
	if !vars["HasMinValue"].(bool) {
		t.Error("expected HasMinValue to be true")
	}

	requiredFields := vars["RequiredFields"].([]string)
	if len(requiredFields) != 1 || requiredFields[0] != "kubernetes.minVersion" {
		t.Errorf("expected required fields to contain kubernetes.minVersion, got %v", requiredFields)
	}
}

// TestGenerationSummary tests generation summary creation
func TestGenerationSummary(t *testing.T) {
	generator := NewAnalyzerGenerator()

	result := &GenerationResult{
		GeneratedAnalyzers: []GeneratedAnalyzer{
			{
				Type:   AnalyzerTypeKubernetes,
				Source: "line1\nline2\nline3",
			},
			{
				Type:   AnalyzerTypeResources,
				Source: "line1\nline2",
			},
		},
		Errors:   []GenerationError{{Type: "test_error"}},
		Warnings: []GenerationWarning{{Type: "test_warning"}},
	}

	summary := generator.generateSummary(result)

	if summary.GeneratedAnalyzers != 2 {
		t.Errorf("expected 2 generated analyzers, got %d", summary.GeneratedAnalyzers)
	}

	if summary.AnalyzersByType["kubernetes"] != 1 {
		t.Errorf("expected 1 kubernetes analyzer, got %d", summary.AnalyzersByType["kubernetes"])
	}

	if summary.AnalyzersByType["resources"] != 1 {
		t.Errorf("expected 1 resources analyzer, got %d", summary.AnalyzersByType["resources"])
	}

	if summary.ValidationResults["errors"] != 1 {
		t.Errorf("expected 1 error, got %d", summary.ValidationResults["errors"])
	}

	if summary.ValidationResults["warnings"] != 1 {
		t.Errorf("expected 1 warning, got %d", summary.ValidationResults["warnings"])
	}

	if summary.LinesOfCode != 5 { // 3 + 2 lines
		t.Errorf("expected 5 lines of code, got %d", summary.LinesOfCode)
	}
}
