package generators

import (
	"bytes"
	"fmt"
	"go/format"
	"strings"
	"text/template"
)

// AnalyzerGenerator generates analyzers from requirement specifications
type AnalyzerGenerator struct {
	templates  map[AnalyzerType]*template.Template
	ruleEngine *RuleEngine
	validator  *GeneratedAnalyzerValidator
	categories map[RequirementCategory]AnalyzerTemplate
}

// AnalyzerType defines the type of analyzer to generate
type AnalyzerType string

const (
	AnalyzerTypeKubernetes AnalyzerType = "kubernetes"
	AnalyzerTypeResources  AnalyzerType = "resources"
	AnalyzerTypeStorage    AnalyzerType = "storage"
	AnalyzerTypeNetwork    AnalyzerType = "network"
	AnalyzerTypeSecurity   AnalyzerType = "security"
	AnalyzerTypeCustom     AnalyzerType = "custom"
	AnalyzerTypeVendor     AnalyzerType = "vendor"
	AnalyzerTypeReplicated AnalyzerType = "replicated"
)

// AnalyzerTemplate defines the structure for analyzer templates
type AnalyzerTemplate struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Type         AnalyzerType      `json:"type"`
	Template     string            `json:"template"`
	Variables    map[string]string `json:"variables"`
	Imports      []string          `json:"imports"`
	Methods      []string          `json:"methods"`
	TestTemplate string            `json:"test_template"`
}

// GeneratedAnalyzer represents a generated analyzer
type GeneratedAnalyzer struct {
	Name         string                   `json:"name"`
	Description  string                   `json:"description"`
	Type         AnalyzerType             `json:"type"`
	Source       string                   `json:"source"`
	TestSource   string                   `json:"test_source"`
	Metadata     AnalyzerMetadata         `json:"metadata"`
	Requirements []CategorizedRequirement `json:"requirements"`
}

// AnalyzerMetadata contains metadata about generated analyzers
type AnalyzerMetadata struct {
	GeneratedAt   string            `json:"generated_at"`
	SourceSpec    string            `json:"source_spec"`
	Version       string            `json:"version"`
	Tags          []string          `json:"tags"`
	Dependencies  []string          `json:"dependencies"`
	Configuration map[string]string `json:"configuration"`
}

// GenerationOptions configures analyzer generation
type GenerationOptions struct {
	PackageName     string            `json:"package_name"`
	OutputPath      string            `json:"output_path"`
	GenerateTests   bool              `json:"generate_tests"`
	ValidateOutput  bool              `json:"validate_output"`
	FormatCode      bool              `json:"format_code"`
	AddComments     bool              `json:"add_comments"`
	CustomVariables map[string]string `json:"custom_variables"`
	Imports         []string          `json:"additional_imports"`
}

// GenerationResult contains the results of analyzer generation
type GenerationResult struct {
	GeneratedAnalyzers []GeneratedAnalyzer `json:"generated_analyzers"`
	Errors             []GenerationError   `json:"errors"`
	Warnings           []GenerationWarning `json:"warnings"`
	Summary            GenerationSummary   `json:"summary"`
}

// GenerationError represents an error during generation
type GenerationError struct {
	Type        string `json:"type"`
	Message     string `json:"message"`
	Requirement string `json:"requirement,omitempty"`
	Template    string `json:"template,omitempty"`
	Line        int    `json:"line,omitempty"`
}

// GenerationWarning represents a warning during generation
type GenerationWarning struct {
	Type        string `json:"type"`
	Message     string `json:"message"`
	Requirement string `json:"requirement,omitempty"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// GenerationSummary provides a summary of generation results
type GenerationSummary struct {
	TotalRequirements  int            `json:"total_requirements"`
	GeneratedAnalyzers int            `json:"generated_analyzers"`
	AnalyzersByType    map[string]int `json:"analyzers_by_type"`
	GenerationTime     string         `json:"generation_time"`
	ValidationResults  map[string]int `json:"validation_results"`
	LinesOfCode        int            `json:"lines_of_code"`
}

// NewAnalyzerGenerator creates a new analyzer generator
func NewAnalyzerGenerator() *AnalyzerGenerator {
	return &AnalyzerGenerator{
		templates:  getDefaultTemplates(),
		ruleEngine: NewRuleEngine(),
		validator:  NewGeneratedAnalyzerValidator(),
		categories: getDefaultCategoryTemplates(),
	}
}

// GenerateAnalyzers generates analyzers from categorized requirements
func (g *AnalyzerGenerator) GenerateAnalyzers(requirements []CategorizedRequirement, opts GenerationOptions) (*GenerationResult, error) {
	result := &GenerationResult{
		GeneratedAnalyzers: []GeneratedAnalyzer{},
		Errors:             []GenerationError{},
		Warnings:           []GenerationWarning{},
	}

	// Group requirements by category
	reqsByCategory := g.groupRequirementsByCategory(requirements)

	// Generate analyzers for each category
	for category, reqs := range reqsByCategory {
		analyzers, errors, warnings := g.generateForCategory(category, reqs, opts)

		result.GeneratedAnalyzers = append(result.GeneratedAnalyzers, analyzers...)
		result.Errors = append(result.Errors, errors...)
		result.Warnings = append(result.Warnings, warnings...)
	}

	// Validate generated analyzers if requested
	if opts.ValidateOutput {
		validationErrors, validationWarnings := g.validateGeneratedAnalyzers(result.GeneratedAnalyzers)
		result.Errors = append(result.Errors, validationErrors...)
		result.Warnings = append(result.Warnings, validationWarnings...)
	}

	// Generate summary
	result.Summary = g.generateSummary(result)

	return result, nil
}

// AddTemplate adds a custom analyzer template
func (g *AnalyzerGenerator) AddTemplate(analyzerType AnalyzerType, tmpl AnalyzerTemplate) error {
	parsedTemplate, err := template.New(tmpl.Name).Parse(tmpl.Template)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	g.templates[analyzerType] = parsedTemplate
	g.categories[RequirementCategory(analyzerType)] = tmpl

	return nil
}

// groupRequirementsByCategory groups requirements by their category
func (g *AnalyzerGenerator) groupRequirementsByCategory(requirements []CategorizedRequirement) map[RequirementCategory][]CategorizedRequirement {
	grouped := make(map[RequirementCategory][]CategorizedRequirement)

	for _, req := range requirements {
		grouped[req.Category] = append(grouped[req.Category], req)
	}

	return grouped
}

// generateForCategory generates analyzers for a specific category
func (g *AnalyzerGenerator) generateForCategory(category RequirementCategory, requirements []CategorizedRequirement, opts GenerationOptions) ([]GeneratedAnalyzer, []GenerationError, []GenerationWarning) {
	var analyzers []GeneratedAnalyzer
	var errors []GenerationError
	var warnings []GenerationWarning

	// Get template for this category
	categoryTemplate, exists := g.categories[category]
	if !exists {
		errors = append(errors, GenerationError{
			Type:    "missing_template",
			Message: fmt.Sprintf("no template found for category %s", category),
		})
		return analyzers, errors, warnings
	}

	// Apply rules to determine what analyzers to generate
	analyzerSpecs := g.ruleEngine.ApplyRules(category, requirements)

	// Generate each analyzer
	for _, spec := range analyzerSpecs {
		analyzer, genErrors, genWarnings := g.generateAnalyzer(spec, categoryTemplate, opts)
		if analyzer != nil {
			analyzers = append(analyzers, *analyzer)
		}
		errors = append(errors, genErrors...)
		warnings = append(warnings, genWarnings...)
	}

	return analyzers, errors, warnings
}

// generateAnalyzer generates a single analyzer from a specification
func (g *AnalyzerGenerator) generateAnalyzer(spec AnalyzerSpec, tmpl AnalyzerTemplate, opts GenerationOptions) (*GeneratedAnalyzer, []GenerationError, []GenerationWarning) {
	var errors []GenerationError
	var warnings []GenerationWarning

	// Prepare template variables
	vars := g.prepareTemplateVariables(spec, tmpl, opts)

	// Execute main template
	source, err := g.executeTemplate(tmpl.Template, vars)
	if err != nil {
		errors = append(errors, GenerationError{
			Type:     "template_execution",
			Message:  fmt.Sprintf("failed to execute template: %v", err),
			Template: tmpl.Name,
		})
		return nil, errors, warnings
	}

	// Generate test source if requested
	var testSource string
	if opts.GenerateTests && tmpl.TestTemplate != "" {
		testSource, err = g.executeTemplate(tmpl.TestTemplate, vars)
		if err != nil {
			warnings = append(warnings, GenerationWarning{
				Type:    "test_generation",
				Message: fmt.Sprintf("failed to generate test: %v", err),
			})
		}
	}

	// Format code if requested
	if opts.FormatCode {
		formatted, err := g.formatGoCode(source)
		if err != nil {
			warnings = append(warnings, GenerationWarning{
				Type:    "code_formatting",
				Message: fmt.Sprintf("failed to format code: %v", err),
			})
		} else {
			source = formatted
		}

		if testSource != "" {
			formatted, err := g.formatGoCode(testSource)
			if err != nil {
				warnings = append(warnings, GenerationWarning{
					Type:    "test_formatting",
					Message: fmt.Sprintf("failed to format test code: %v", err),
				})
			} else {
				testSource = formatted
			}
		}
	}

	analyzer := &GeneratedAnalyzer{
		Name:        spec.Name,
		Description: spec.Description,
		Type:        AnalyzerType(spec.Category),
		Source:      source,
		TestSource:  testSource,
		Metadata: AnalyzerMetadata{
			GeneratedAt:  "now", // In real implementation, use time.Now()
			Version:      "1.0.0",
			Tags:         spec.Tags,
			Dependencies: tmpl.Imports,
		},
		Requirements: spec.Requirements,
	}

	return analyzer, errors, warnings
}

// prepareTemplateVariables prepares variables for template execution
func (g *AnalyzerGenerator) prepareTemplateVariables(spec AnalyzerSpec, tmpl AnalyzerTemplate, opts GenerationOptions) map[string]interface{} {
	vars := make(map[string]interface{})

	// Basic variables
	vars["Name"] = spec.Name
	vars["Description"] = spec.Description
	vars["PackageName"] = opts.PackageName
	vars["Type"] = string(spec.Category)
	vars["Requirements"] = spec.Requirements
	vars["Tags"] = spec.Tags

	// Template-specific variables
	for key, value := range tmpl.Variables {
		vars[key] = value
	}

	// Custom variables from options
	for key, value := range opts.CustomVariables {
		vars[key] = value
	}

	// Dynamic variables based on requirements
	vars["HasMinValue"] = g.hasRequirementWithMinValue(spec.Requirements)
	vars["HasMaxValue"] = g.hasRequirementWithMaxValue(spec.Requirements)
	vars["RequiredFields"] = g.getRequiredFields(spec.Requirements)
	vars["OptionalFields"] = g.getOptionalFields(spec.Requirements)

	return vars
}

// executeTemplate executes a template with given variables
func (g *AnalyzerGenerator) executeTemplate(templateStr string, vars map[string]interface{}) (string, error) {
	tmpl, err := template.New("analyzer").Parse(templateStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// formatGoCode formats Go source code
func (g *AnalyzerGenerator) formatGoCode(source string) (string, error) {
	formatted, err := format.Source([]byte(source))
	if err != nil {
		return source, err
	}
	return string(formatted), nil
}

// validateGeneratedAnalyzers validates the generated analyzers
func (g *AnalyzerGenerator) validateGeneratedAnalyzers(analyzers []GeneratedAnalyzer) ([]GenerationError, []GenerationWarning) {
	var errors []GenerationError
	var warnings []GenerationWarning

	for _, analyzer := range analyzers {
		validationErrors, validationWarnings := g.validator.Validate(analyzer)

		// Convert validation results to generation results
		for _, err := range validationErrors {
			errors = append(errors, GenerationError{
				Type:    "validation",
				Message: err.Message,
			})
		}

		for _, warn := range validationWarnings {
			warnings = append(warnings, GenerationWarning{
				Type:       "validation",
				Message:    warn.Message,
				Suggestion: warn.Suggestion,
			})
		}
	}

	return errors, warnings
}

// Helper methods

func (g *AnalyzerGenerator) hasRequirementWithMinValue(reqs []CategorizedRequirement) bool {
	for _, req := range reqs {
		if strings.Contains(strings.ToLower(req.Path), "min") {
			return true
		}
	}
	return false
}

func (g *AnalyzerGenerator) hasRequirementWithMaxValue(reqs []CategorizedRequirement) bool {
	for _, req := range reqs {
		if strings.Contains(strings.ToLower(req.Path), "max") {
			return true
		}
	}
	return false
}

func (g *AnalyzerGenerator) getRequiredFields(reqs []CategorizedRequirement) []string {
	var required []string
	for _, req := range reqs {
		if req.Priority == PriorityRequired {
			required = append(required, req.Path)
		}
	}
	return required
}

func (g *AnalyzerGenerator) getOptionalFields(reqs []CategorizedRequirement) []string {
	var optional []string
	for _, req := range reqs {
		if req.Priority == PriorityOptional {
			optional = append(optional, req.Path)
		}
	}
	return optional
}

// generateSummary generates a summary of generation results
func (g *AnalyzerGenerator) generateSummary(result *GenerationResult) GenerationSummary {
	summary := GenerationSummary{
		GeneratedAnalyzers: len(result.GeneratedAnalyzers),
		AnalyzersByType:    make(map[string]int),
		ValidationResults:  make(map[string]int),
		GenerationTime:     "now", // In real implementation, track actual time
	}

	// Count by type
	for _, analyzer := range result.GeneratedAnalyzers {
		summary.AnalyzersByType[string(analyzer.Type)]++
		summary.LinesOfCode += len(strings.Split(analyzer.Source, "\n"))
	}

	// Count validation results
	summary.ValidationResults["errors"] = len(result.Errors)
	summary.ValidationResults["warnings"] = len(result.Warnings)

	return summary
}

// getDefaultTemplates returns default analyzer templates
func getDefaultTemplates() map[AnalyzerType]*template.Template {
	templates := make(map[AnalyzerType]*template.Template)

	// We would load actual templates here
	// For now, return empty map as placeholder

	return templates
}

// getDefaultCategoryTemplates returns default category templates
func getDefaultCategoryTemplates() map[RequirementCategory]AnalyzerTemplate {
	return map[RequirementCategory]AnalyzerTemplate{
		CategoryKubernetes: {
			Name:        "kubernetes-analyzer",
			Description: "Kubernetes requirement analyzer",
			Type:        AnalyzerTypeKubernetes,
			Template:    getKubernetesTemplate(),
			Variables:   map[string]string{"AnalyzerType": "kubernetes"},
			Imports:     []string{"k8s.io/api/core/v1", "k8s.io/apimachinery/pkg/version"},
			Methods:     []string{"checkVersion", "checkAPI", "checkFeatures"},
		},
		CategoryResources: {
			Name:        "resource-analyzer",
			Description: "Resource requirement analyzer",
			Type:        AnalyzerTypeResources,
			Template:    getResourceTemplate(),
			Variables:   map[string]string{"AnalyzerType": "resources"},
			Imports:     []string{"k8s.io/api/core/v1", "k8s.io/apimachinery/pkg/api/resource"},
			Methods:     []string{"checkCPU", "checkMemory", "checkNodes"},
		},
		CategoryStorage: {
			Name:        "storage-analyzer",
			Description: "Storage requirement analyzer",
			Type:        AnalyzerTypeStorage,
			Template:    getStorageTemplate(),
			Variables:   map[string]string{"AnalyzerType": "storage"},
			Imports:     []string{"k8s.io/api/storage/v1"},
			Methods:     []string{"checkStorageClass", "checkPersistentVolume", "checkCapacity"},
		},
	}
}
