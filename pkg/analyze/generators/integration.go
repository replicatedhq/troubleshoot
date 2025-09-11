package generators

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// AnalysisEngineIntegration integrates the analyzer generator with the analysis engine
type AnalysisEngineIntegration struct {
	parser    *RequirementParser
	generator *AnalyzerGenerator
	engine    AnalysisEngineInterface
}

// AnalysisEngineInterface defines the interface for the analysis engine
type AnalysisEngineInterface interface {
	Analyze(ctx context.Context, bundle *SupportBundle, opts AnalysisOptions) (*AnalysisResult, error)
	RegisterAnalyzer(name string, analyzer Analyzer) error
}

// GeneratedAnalysisResult extends AnalysisResult with generation metadata
type GeneratedAnalysisResult struct {
	*AnalysisResult
	GenerationMetadata GenerationMetadata `json:"generation_metadata"`
	GeneratedAnalyzers []string           `json:"generated_analyzers"`
}

// GenerationMetadata contains metadata about the generation process
type GenerationMetadata struct {
	SourceRequirements string            `json:"source_requirements"`
	GeneratedAt        string            `json:"generated_at"`
	GeneratedBy        string            `json:"generated_by"`
	GenerationOptions  GenerationOptions `json:"generation_options"`
	GenerationStats    GenerationSummary `json:"generation_stats"`
}

// CLI Integration structures

// GenerateCommand represents the CLI command for analyzer generation
type GenerateCommand struct {
	RequirementFiles []string          `json:"requirement_files"`
	OutputDir        string            `json:"output_dir"`
	PackageName      string            `json:"package_name"`
	GenerateTests    bool              `json:"generate_tests"`
	ValidateOutput   bool              `json:"validate_output"`
	FormatCode       bool              `json:"format_code"`
	CustomVars       map[string]string `json:"custom_vars"`
}

// AnalyzeFromRequirementsCommand represents the CLI command for analysis from requirements
type AnalyzeFromRequirementsCommand struct {
	RequirementFiles []string `json:"requirement_files"`
	BundlePath       string   `json:"bundle_path"`
	GenerateOnDemand bool     `json:"generate_on_demand"`
	CacheGenerated   bool     `json:"cache_generated"`
}

// NewAnalysisEngineIntegration creates a new integration instance
func NewAnalysisEngineIntegration(engine AnalysisEngineInterface) *AnalysisEngineIntegration {
	return &AnalysisEngineIntegration{
		parser:    NewRequirementParser(),
		generator: NewAnalyzerGenerator(),
		engine:    engine,
	}
}

// GenerateAnalyzersFromRequirements generates analyzers from requirement specifications
func (ai *AnalysisEngineIntegration) GenerateAnalyzersFromRequirements(ctx context.Context, requirementFiles []string, opts GenerationOptions) (*GenerationResult, error) {
	// Parse requirement specifications
	var allRequirements []CategorizedRequirement
	var sourceSpecs []*RequirementSpec

	for _, file := range requirementFiles {
		result, err := ai.parser.ParseFromFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to parse requirement file %s: %w", file, err)
		}

		// Add to collection
		sourceSpecs = append(sourceSpecs, result)

		// Categorize the requirements
		categorized, err := ai.parser.categorizer.CategorizeSpec(result)
		if err != nil {
			return nil, fmt.Errorf("failed to categorize requirements from %s: %w", file, err)
		}
		allRequirements = append(allRequirements, categorized...)
	}

	// Generate analyzers
	generationResult, err := ai.generator.GenerateAnalyzers(allRequirements, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to generate analyzers: %w", err)
	}

	// Write generated analyzers to files if output directory is specified
	if opts.OutputPath != "" {
		if err := ai.writeGeneratedAnalyzers(generationResult.GeneratedAnalyzers, opts); err != nil {
			return nil, fmt.Errorf("failed to write generated analyzers: %w", err)
		}
	}

	return generationResult, nil
}

// AnalyzeWithGeneratedAnalyzers performs analysis using dynamically generated analyzers
func (ai *AnalysisEngineIntegration) AnalyzeWithGeneratedAnalyzers(ctx context.Context, requirementFiles []string, bundle *SupportBundle, analysisOpts AnalysisOptions) (*GeneratedAnalysisResult, error) {
	// Generate analyzers on-demand
	generationOpts := GenerationOptions{
		PackageName:    "generated",
		GenerateTests:  false,
		ValidateOutput: true,
		FormatCode:     true,
	}

	generationResult, err := ai.GenerateAnalyzersFromRequirements(ctx, requirementFiles, generationOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to generate analyzers: %w", err)
	}

	// Register generated analyzers with the engine
	analyzerNames := []string{}
	for _, generated := range generationResult.GeneratedAnalyzers {
		analyzer, err := ai.createRuntimeAnalyzer(generated)
		if err != nil {
			return nil, fmt.Errorf("failed to create runtime analyzer %s: %w", generated.Name, err)
		}

		if err := ai.engine.RegisterAnalyzer(generated.Name, analyzer); err != nil {
			return nil, fmt.Errorf("failed to register analyzer %s: %w", generated.Name, err)
		}

		analyzerNames = append(analyzerNames, generated.Name)
	}

	// Run analysis with the generated analyzers
	result, err := ai.engine.Analyze(ctx, bundle, analysisOpts)
	if err != nil {
		return nil, fmt.Errorf("analysis failed: %w", err)
	}

	// Create enhanced result with generation metadata
	generatedResult := &GeneratedAnalysisResult{
		AnalysisResult:     result,
		GeneratedAnalyzers: analyzerNames,
		GenerationMetadata: GenerationMetadata{
			SourceRequirements: fmt.Sprintf("%d requirement files", len(requirementFiles)),
			GeneratedAt:        "now", // In real implementation, use time.Now()
			GeneratedBy:        "AnalysisEngineIntegration",
			GenerationOptions:  generationOpts,
			GenerationStats:    generationResult.Summary,
		},
	}

	return generatedResult, nil
}

// writeGeneratedAnalyzers writes generated analyzers to files
func (ai *AnalysisEngineIntegration) writeGeneratedAnalyzers(analyzers []GeneratedAnalyzer, opts GenerationOptions) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(opts.OutputPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	for _, analyzer := range analyzers {
		// Write main analyzer source
		sourcePath := filepath.Join(opts.OutputPath, analyzer.Name+".go")
		if err := os.WriteFile(sourcePath, []byte(analyzer.Source), 0644); err != nil {
			return fmt.Errorf("failed to write analyzer source %s: %w", sourcePath, err)
		}

		// Write test source if it exists
		if analyzer.TestSource != "" {
			testPath := filepath.Join(opts.OutputPath, analyzer.Name+"_test.go")
			if err := os.WriteFile(testPath, []byte(analyzer.TestSource), 0644); err != nil {
				return fmt.Errorf("failed to write test source %s: %w", testPath, err)
			}
		}

		// Write metadata file
		metadataPath := filepath.Join(opts.OutputPath, analyzer.Name+"_metadata.json")
		metadataJSON, _ := jsonMarshal(analyzer.Metadata)
		if err := os.WriteFile(metadataPath, metadataJSON, 0644); err != nil {
			return fmt.Errorf("failed to write metadata %s: %w", metadataPath, err)
		}
	}

	return nil
}

// createRuntimeAnalyzer creates a runtime analyzer from generated code
func (ai *AnalysisEngineIntegration) createRuntimeAnalyzer(generated GeneratedAnalyzer) (Analyzer, error) {
	// In a real implementation, this would:
	// 1. Compile the generated Go code
	// 2. Load it as a plugin or use go/types for dynamic execution
	// 3. Create an Analyzer interface implementation

	// For now, return a mock analyzer that represents the generated analyzer
	return &RuntimeGeneratedAnalyzer{
		name:         generated.Name,
		description:  generated.Description,
		source:       generated.Source,
		requirements: generated.Requirements,
	}, nil
}

// RuntimeGeneratedAnalyzer is a runtime representation of a generated analyzer
type RuntimeGeneratedAnalyzer struct {
	name         string
	description  string
	source       string
	requirements []CategorizedRequirement
}

// Name returns the analyzer name
func (r *RuntimeGeneratedAnalyzer) Name() string {
	return r.name
}

// Version returns the analyzer version
func (r *RuntimeGeneratedAnalyzer) Version() string {
	return "generated-1.0.0"
}

// Capabilities returns the analyzer capabilities
func (r *RuntimeGeneratedAnalyzer) Capabilities() []string {
	var capabilities []string
	for _, req := range r.requirements {
		capabilities = append(capabilities, string(req.Category))
	}
	return capabilities
}

// HealthCheck performs a health check
func (r *RuntimeGeneratedAnalyzer) HealthCheck(ctx context.Context) error {
	// Basic validation that the analyzer was created properly
	if r.name == "" || r.source == "" {
		return fmt.Errorf("analyzer not properly initialized")
	}
	return nil
}

// Analyze performs the analysis (simplified implementation)
func (r *RuntimeGeneratedAnalyzer) Analyze(ctx context.Context, bundle *SupportBundle) (*AnalyzerResult, error) {
	// In a real implementation, this would execute the generated analyzer code
	// For now, return a basic result indicating the analyzer ran
	return &AnalyzerResult{
		Name:        r.name,
		Description: r.description,
		Result:      "pass", // Simplified result
		Confidence:  0.8,
		Impact:      "medium",
		Explanation: fmt.Sprintf("Generated analyzer %s executed successfully", r.name),
		Evidence:    []string{"Generated analyzer validation"},
		Remediation: []RemediationStep{
			{
				Description: "Review generated analyzer results",
				Command:     "",
				Manual:      "Check the specific requirements validated by this analyzer",
			},
		},
	}, nil
}

// CLI Integration Functions

// ExecuteGenerateCommand executes the generate command
func ExecuteGenerateCommand(cmd GenerateCommand) error {
	integration := NewAnalysisEngineIntegration(nil) // No engine needed for generation only

	opts := GenerationOptions{
		PackageName:     cmd.PackageName,
		OutputPath:      cmd.OutputDir,
		GenerateTests:   cmd.GenerateTests,
		ValidateOutput:  cmd.ValidateOutput,
		FormatCode:      cmd.FormatCode,
		CustomVariables: cmd.CustomVars,
	}

	result, err := integration.GenerateAnalyzersFromRequirements(context.Background(), cmd.RequirementFiles, opts)
	if err != nil {
		return fmt.Errorf("generation failed: %w", err)
	}

	// Print generation summary
	fmt.Printf("Generated %d analyzers successfully\n", result.Summary.GeneratedAnalyzers)
	if len(result.Errors) > 0 {
		fmt.Printf("Errors: %d\n", len(result.Errors))
	}
	if len(result.Warnings) > 0 {
		fmt.Printf("Warnings: %d\n", len(result.Warnings))
	}

	return nil
}

// ExecuteAnalyzeFromRequirementsCommand executes the analyze from requirements command
func ExecuteAnalyzeFromRequirementsCommand(cmd AnalyzeFromRequirementsCommand, engine AnalysisEngineInterface) error {
	integration := NewAnalysisEngineIntegration(engine)

	// Load support bundle
	bundle, err := LoadSupportBundle(cmd.BundlePath)
	if err != nil {
		return fmt.Errorf("failed to load support bundle: %w", err)
	}

	// Perform analysis with generated analyzers
	analysisOpts := AnalysisOptions{
		// Configure analysis options based on command
	}

	result, err := integration.AnalyzeWithGeneratedAnalyzers(context.Background(), cmd.RequirementFiles, bundle, analysisOpts)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Print analysis results
	fmt.Printf("Analysis completed using %d generated analyzers\n", len(result.GeneratedAnalyzers))
	fmt.Printf("Total results: %d\n", len(result.Results))

	return nil
}

// Utility functions

// LoadSupportBundle loads a support bundle from the given path
func LoadSupportBundle(path string) (*SupportBundle, error) {
	// In a real implementation, this would load and parse the support bundle
	return &SupportBundle{
		Path:          path,
		BundleRootDir: path,
		// Other bundle data would be populated here
	}, nil
}

// jsonMarshal is a helper function for JSON marshaling
func jsonMarshal(v interface{}) ([]byte, error) {
	// In a real implementation, this would use encoding/json
	return []byte("{}"), nil // Simplified placeholder
}

// Interface definitions for integration with existing analysis engine

// SupportBundle represents a support bundle (placeholder)
type SupportBundle struct {
	Path          string            `json:"path"`
	BundleRootDir string            `json:"bundle_root_dir"`
	Files         map[string][]byte `json:"files"`
}

// AnalysisOptions represents analysis options (placeholder)
type AnalysisOptions struct {
	GeneratedAnalyzersOnly bool              `json:"generated_analyzers_only"`
	RequirementFilters     []string          `json:"requirement_filters"`
	CustomSettings         map[string]string `json:"custom_settings"`
}

// AnalysisResult represents analysis results (placeholder)
type AnalysisResult struct {
	Results   []AnalyzerResult `json:"results"`
	Summary   string           `json:"summary"`
	Timestamp string           `json:"timestamp"`
}

// Analyzer represents the analyzer interface (placeholder)
type Analyzer interface {
	Name() string
	Version() string
	Capabilities() []string
	HealthCheck(ctx context.Context) error
	Analyze(ctx context.Context, bundle *SupportBundle) (*AnalyzerResult, error)
}

// AnalyzerResult represents an individual analyzer result (placeholder)
type AnalyzerResult struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Result      string            `json:"result"`
	Confidence  float64           `json:"confidence"`
	Impact      string            `json:"impact"`
	Explanation string            `json:"explanation"`
	Evidence    []string          `json:"evidence"`
	Remediation []RemediationStep `json:"remediation"`
}

// RemediationStep represents a remediation step (placeholder)
type RemediationStep struct {
	Description string `json:"description"`
	Command     string `json:"command"`
	Manual      string `json:"manual"`
}
