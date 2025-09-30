package artifacts

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewArtifactManager(t *testing.T) {
	tempDir := t.TempDir()
	am := NewArtifactManager(tempDir)

	assert.NotNil(t, am)
	assert.Equal(t, tempDir, am.outputDir)
	assert.NotNil(t, am.formatters)
	assert.NotNil(t, am.generators)
	assert.NotNil(t, am.validators)

	// Check default formatters are registered
	_, exists := am.formatters["json"]
	assert.True(t, exists)
	_, exists = am.formatters["yaml"]
	assert.True(t, exists)
	_, exists = am.formatters["html"]
	assert.True(t, exists)
	_, exists = am.formatters["text"]
	assert.True(t, exists)
}

func TestArtifactManager_GenerateArtifacts(t *testing.T) {
	tempDir := t.TempDir()
	am := NewArtifactManager(tempDir)
	ctx := context.Background()

	// Create sample analysis result
	result := &analyzer.AnalysisResult{
		Results: []*analyzer.AnalyzerResult{
			{
				IsPass:     true,
				Title:      "Pod Status Check",
				Message:    "All pods are healthy",
				Category:   "pods",
				AgentName:  "local",
				Confidence: 0.9,
				Insights:   []string{"No issues detected"},
			},
			{
				IsFail:     true,
				Title:      "Node Resources Check",
				Message:    "Insufficient memory on node1",
				Category:   "nodes",
				AgentName:  "local",
				Confidence: 0.8,
				Remediation: &analyzer.RemediationStep{
					Description:   "Add more memory or reduce workload",
					Priority:      8,
					Category:      "infrastructure",
					IsAutomatable: false,
				},
			},
		},
		Remediation: []analyzer.RemediationStep{
			{
				Description:   "Scale down non-critical workloads",
				Priority:      7,
				Category:      "workload",
				IsAutomatable: true,
				Command:       "kubectl scale deployment non-critical --replicas=1",
			},
		},
		Summary: analyzer.AnalysisSummary{
			TotalAnalyzers: 2,
			PassCount:      1,
			FailCount:      1,
			Duration:       "30s",
			AgentsUsed:     []string{"local"},
		},
		Metadata: analyzer.AnalysisMetadata{
			Timestamp:     time.Now(),
			EngineVersion: "1.0.0",
			Agents: []analyzer.AgentMetadata{
				{
					Name:        "local",
					Duration:    "30s",
					ResultCount: 2,
				},
			},
		},
	}

	tests := []struct {
		name    string
		opts    *ArtifactOptions
		wantErr bool
		errMsg  string
	}{
		{
			name:    "default options",
			opts:    nil,
			wantErr: false,
		},
		{
			name: "multiple formats",
			opts: &ArtifactOptions{
				Formats:             []string{"json", "yaml", "html", "text"},
				IncludeMetadata:     true,
				IncludeCorrelations: true,
			},
			wantErr: false,
		},
		{
			name: "minimal options",
			opts: &ArtifactOptions{
				Formats:         []string{"json"},
				IncludeMetadata: false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifacts, err := am.GenerateArtifacts(ctx, result, tt.opts)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, artifacts)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, artifacts)
				assert.NotEmpty(t, artifacts)

				// Verify primary analysis.json artifact exists
				var analysisArtifact *Artifact
				for _, artifact := range artifacts {
					if artifact.Name == "analysis.json" {
						analysisArtifact = artifact
						break
					}
				}

				require.NotNil(t, analysisArtifact, "analysis.json artifact should exist")
				assert.Equal(t, "analysis", analysisArtifact.Type)
				assert.Equal(t, "json", analysisArtifact.Format)
				assert.Equal(t, "application/json", analysisArtifact.ContentType)
				assert.Greater(t, analysisArtifact.Size, int64(0))
				assert.NotEmpty(t, analysisArtifact.Path)

				// Verify file exists on disk
				_, err := os.Stat(analysisArtifact.Path)
				assert.NoError(t, err)

				// Verify content is valid JSON
				var parsedResult analyzer.AnalysisResult
				err = json.Unmarshal(analysisArtifact.Content, &parsedResult)
				assert.NoError(t, err)
			}
		})
	}
}

func TestArtifactManager_generateAnalysisJSON(t *testing.T) {
	tempDir := t.TempDir()
	am := NewArtifactManager(tempDir)
	ctx := context.Background()

	result := &analyzer.AnalysisResult{
		Results: []*analyzer.AnalyzerResult{
			{
				IsPass:    true,
				Title:     "Test Check",
				Message:   "Test message",
				Category:  "test",
				AgentName: "local",
			},
		},
		Summary: analyzer.AnalysisSummary{
			TotalAnalyzers: 1,
			PassCount:      1,
			Duration:       "1s",
			AgentsUsed:     []string{"local"},
		},
		Metadata: analyzer.AnalysisMetadata{
			Timestamp:     time.Now(),
			EngineVersion: "1.0.0",
		},
	}

	opts := &ArtifactOptions{
		IncludeMetadata: true,
	}

	artifact, err := am.generateAnalysisJSON(ctx, result, opts)
	require.NoError(t, err)
	require.NotNil(t, artifact)

	assert.Equal(t, "analysis.json", artifact.Name)
	assert.Equal(t, "analysis", artifact.Type)
	assert.Equal(t, "json", artifact.Format)
	assert.Greater(t, artifact.Size, int64(0))
	assert.NotEmpty(t, artifact.Content)

	// Verify JSON is valid and contains expected data
	var parsedResult analyzer.AnalysisResult
	err = json.Unmarshal(artifact.Content, &parsedResult)
	require.NoError(t, err)

	assert.Len(t, parsedResult.Results, 1)
	assert.Equal(t, result.Results[0].Title, parsedResult.Results[0].Title)
	assert.Equal(t, result.Summary.TotalAnalyzers, parsedResult.Summary.TotalAnalyzers)
}

func TestArtifactManager_generateSummaryArtifact(t *testing.T) {
	am := NewArtifactManager(t.TempDir())
	ctx := context.Background()

	result := &analyzer.AnalysisResult{
		Results: []*analyzer.AnalyzerResult{
			{IsPass: true, Category: "pods"},
			{IsFail: true, Category: "nodes", Confidence: 0.9},
			{IsWarn: true, Category: "pods"},
		},
		Summary: analyzer.AnalysisSummary{
			TotalAnalyzers: 3,
			PassCount:      1,
			WarnCount:      1,
			FailCount:      1,
		},
		Metadata: analyzer.AnalysisMetadata{
			Agents: []analyzer.AgentMetadata{
				{Name: "local", ResultCount: 3},
			},
		},
	}

	opts := &ArtifactOptions{}

	artifact, err := am.generateSummaryArtifact(ctx, result, opts)
	require.NoError(t, err)
	require.NotNil(t, artifact)

	assert.Equal(t, "summary.json", artifact.Name)
	assert.Equal(t, "summary", artifact.Type)

	// Parse and verify summary content
	var summary struct {
		Overview   analyzer.AnalysisSummary   `json:"overview"`
		Categories map[string]int             `json:"categories"`
		TopIssues  []*analyzer.AnalyzerResult `json:"topIssues"`
	}

	err = json.Unmarshal(artifact.Content, &summary)
	require.NoError(t, err)

	assert.Equal(t, 3, summary.Overview.TotalAnalyzers)
	assert.Equal(t, map[string]int{"pods": 2, "nodes": 1}, summary.Categories)
	assert.Len(t, summary.TopIssues, 1) // Only failed results
}

func TestArtifactManager_generateRemediationGuide(t *testing.T) {
	am := NewArtifactManager(t.TempDir())
	ctx := context.Background()

	result := &analyzer.AnalysisResult{
		Remediation: []analyzer.RemediationStep{
			{
				Description:   "High priority fix",
				Priority:      9,
				Category:      "infrastructure",
				IsAutomatable: true,
				Command:       "kubectl apply -f fix.yaml",
			},
			{
				Description:   "Medium priority fix",
				Priority:      5,
				Category:      "workload",
				IsAutomatable: false,
			},
		},
	}

	opts := &ArtifactOptions{}

	artifact, err := am.generateRemediationGuide(ctx, result, opts)
	require.NoError(t, err)
	require.NotNil(t, artifact)

	assert.Equal(t, "remediation-guide.json", artifact.Name)
	assert.Equal(t, "remediation", artifact.Type)

	// Parse and verify remediation content
	var guide struct {
		Summary         string                                `json:"summary"`
		PriorityActions []analyzer.RemediationStep            `json:"priorityActions"`
		Categories      map[string][]analyzer.RemediationStep `json:"categories"`
		Automation      AutomationGuide                       `json:"automation"`
	}

	err = json.Unmarshal(artifact.Content, &guide)
	require.NoError(t, err)

	assert.Contains(t, guide.Summary, "2 remediation steps")
	assert.Len(t, guide.PriorityActions, 2)
	assert.Equal(t, 9, guide.PriorityActions[0].Priority) // Should be sorted by priority
	assert.Len(t, guide.Categories, 2)                    // infrastructure and workload
	assert.Equal(t, 1, guide.Automation.AutomatableSteps)
	assert.Equal(t, 1, guide.Automation.ManualSteps)
}

func TestArtifactManager_Formatters(t *testing.T) {
	am := NewArtifactManager(t.TempDir())
	ctx := context.Background()

	result := &analyzer.AnalysisResult{
		Results: []*analyzer.AnalyzerResult{
			{
				IsPass:    true,
				Title:     "Test Check",
				Message:   "All systems operational",
				Category:  "test",
				AgentName: "local",
			},
		},
		Summary: analyzer.AnalysisSummary{
			TotalAnalyzers: 1,
			PassCount:      1,
		},
		Metadata: analyzer.AnalysisMetadata{
			Timestamp:     time.Now(),
			EngineVersion: "1.0.0",
		},
	}

	formats := []string{"json", "yaml", "html", "text"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			formatter, exists := am.formatters[format]
			require.True(t, exists, "formatter for %s should exist", format)

			data, err := formatter.Format(ctx, result)
			require.NoError(t, err)
			require.NotEmpty(t, data)

			// Verify content type and extension
			assert.NotEmpty(t, formatter.ContentType())
			assert.NotEmpty(t, formatter.FileExtension())
		})
	}
}

func TestArtifactManager_HelperMethods(t *testing.T) {
	am := NewArtifactManager(t.TempDir())

	results := []*analyzer.AnalyzerResult{
		{IsPass: true, Category: "pods", Confidence: 0.9},
		{IsFail: true, Category: "nodes", Confidence: 0.8},
		{IsWarn: true, Category: "pods", Confidence: 0.7},
		{IsFail: true, Category: "storage", Confidence: 0.6},
	}

	// Test categorizeResults
	categories := am.categorizeResults(results)
	expected := map[string]int{"pods": 2, "nodes": 1, "storage": 1}
	assert.Equal(t, expected, categories)

	// Test getTopIssues
	topIssues := am.getTopIssues(results, 2)
	assert.Len(t, topIssues, 2)
	assert.True(t, topIssues[0].IsFail)
	assert.True(t, topIssues[1].IsFail)
	// Should be sorted by confidence
	assert.GreaterOrEqual(t, topIssues[0].Confidence, topIssues[1].Confidence)

	// Test getTopCategories
	topCategories := am.getTopCategories(results, 2)
	assert.Len(t, topCategories, 2)
	assert.Equal(t, "pods", topCategories[0]) // Should be highest count first

	// Test countCriticalIssues
	results[0].Severity = "critical"
	results[0].IsFail = true
	critical := am.countCriticalIssues(results)
	assert.Equal(t, 1, critical)
}

func TestArtifactManager_WriteArtifact(t *testing.T) {
	am := NewArtifactManager(t.TempDir())

	artifact := &Artifact{
		Name:    "test.json",
		Content: []byte(`{"test": "data"}`),
	}

	path := filepath.Join(am.outputDir, artifact.Name)
	err := am.writeArtifact(artifact, path)
	require.NoError(t, err)

	// Verify file exists and content matches
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, artifact.Content, content)
}

func TestArtifactManager_RegisterComponents(t *testing.T) {
	am := NewArtifactManager(t.TempDir())

	// Test RegisterFormatter
	mockFormatter := &mockFormatter{
		contentType: "test/format",
		extension:   "test",
	}
	am.RegisterFormatter("test", mockFormatter)

	formatter, exists := am.formatters["test"]
	assert.True(t, exists)
	assert.Equal(t, mockFormatter, formatter)

	// Test RegisterGenerator
	mockGenerator := &mockGenerator{
		name: "Test Generator",
	}
	am.RegisterGenerator("test", mockGenerator)

	generator, exists := am.generators["test"]
	assert.True(t, exists)
	assert.Equal(t, mockGenerator, generator)

	// Test RegisterValidator
	mockValidator := &mockValidator{
		schema: "test-schema",
	}
	am.RegisterValidator("test", mockValidator)

	validator, exists := am.validators["test"]
	assert.True(t, exists)
	assert.Equal(t, mockValidator, validator)
}

// Mock implementations for testing

type mockFormatter struct {
	contentType string
	extension   string
	data        []byte
	err         error
}

func (m *mockFormatter) Format(ctx context.Context, result *analyzer.AnalysisResult) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.data != nil {
		return m.data, nil
	}
	return []byte("formatted data"), nil
}

func (m *mockFormatter) ContentType() string {
	return m.contentType
}

func (m *mockFormatter) FileExtension() string {
	return m.extension
}

type mockGenerator struct {
	name        string
	description string
	artifact    *Artifact
	err         error
}

func (m *mockGenerator) Generate(ctx context.Context, result *analyzer.AnalysisResult) (*Artifact, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.artifact != nil {
		return m.artifact, nil
	}
	return &Artifact{
		Name:    "mock-artifact.json",
		Type:    "mock",
		Format:  "json",
		Content: []byte(`{"mock": "data"}`),
	}, nil
}

func (m *mockGenerator) Name() string {
	return m.name
}

func (m *mockGenerator) Description() string {
	return m.description
}

type mockValidator struct {
	schema string
	err    error
}

func (m *mockValidator) Validate(ctx context.Context, data []byte) error {
	return m.err
}

func (m *mockValidator) Schema() string {
	return m.schema
}
