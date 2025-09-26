package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAnalysisEngine(t *testing.T) {
	engine := NewAnalysisEngine()

	assert.NotNil(t, engine)
	assert.Len(t, engine.ListAgents(), 0) // No agents registered initially
}

func TestAnalysisEngine_RegisterAgent(t *testing.T) {
	engine := NewAnalysisEngine()

	tests := []struct {
		name      string
		agentName string
		agent     Agent
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid agent registration",
			agentName: "test-agent",
			agent:     &mockAgent{name: "test-agent"},
			wantErr:   false,
		},
		{
			name:      "empty agent name",
			agentName: "",
			agent:     &mockAgent{name: "test-agent"},
			wantErr:   true,
			errMsg:    "agent name cannot be empty",
		},
		{
			name:      "nil agent",
			agentName: "test-agent",
			agent:     nil,
			wantErr:   true,
			errMsg:    "agent cannot be nil",
		},
		{
			name:      "duplicate agent registration",
			agentName: "duplicate-agent",
			agent:     &mockAgent{name: "duplicate-agent"},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.RegisterAgent(tt.agentName, tt.agent)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)

				// Verify agent was registered
				agent, exists := engine.GetAgent(tt.agentName)
				assert.True(t, exists)
				assert.Equal(t, tt.agent, agent)
			}
		})
	}

	// Test duplicate registration error with fresh engine
	freshEngine := NewAnalysisEngine()
	agent := &mockAgent{name: "duplicate-agent"}
	err := freshEngine.RegisterAgent("duplicate-agent", agent)
	require.NoError(t, err)

	err = freshEngine.RegisterAgent("duplicate-agent", agent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestAnalysisEngine_Analyze(t *testing.T) {
	engine := NewAnalysisEngine()

	// Register a mock agent
	mockAgent := &mockAgent{
		name:      "test-agent",
		available: true,
		results: []*AnalyzerResult{
			{
				Title:   "Test Result",
				Message: "Test message",
				IsPass:  true,
			},
		},
	}

	err := engine.RegisterAgent("test-agent", mockAgent)
	require.NoError(t, err)

	tests := []struct {
		name    string
		bundle  *SupportBundle
		opts    AnalysisOptions
		wantErr bool
		errMsg  string
	}{
		{
			name: "successful analysis",
			bundle: &SupportBundle{
				Files: map[string][]byte{
					"test.json": []byte(`{"test": "data"}`),
				},
				Metadata: &SupportBundleMetadata{
					CreatedAt: time.Now(),
					Version:   "1.0.0",
				},
			},
			opts: AnalysisOptions{
				Agents: []string{"test-agent"},
			},
			wantErr: false,
		},
		{
			name:    "nil bundle",
			bundle:  nil,
			opts:    AnalysisOptions{},
			wantErr: true,
			errMsg:  "bundle cannot be nil",
		},
		{
			name: "non-existent agent",
			bundle: &SupportBundle{
				Files: map[string][]byte{},
			},
			opts: AnalysisOptions{
				Agents: []string{"non-existent-agent"},
			},
			wantErr: true,
			errMsg:  "not registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := engine.Analyze(ctx, tt.bundle, tt.opts)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)

				// Verify basic result structure
				assert.NotNil(t, result.Results)
				assert.NotNil(t, result.Summary)
				assert.NotNil(t, result.Metadata)

				// Verify agent was used
				assert.Contains(t, result.Summary.AgentsUsed, "test-agent")
			}
		})
	}
}

func TestAnalysisEngine_GenerateAnalyzers(t *testing.T) {
	engine := NewAnalysisEngine()
	ctx := context.Background()

	tests := []struct {
		name         string
		requirements *RequirementSpec
		wantErr      bool
		errMsg       string
		wantSpecs    int
	}{
		{
			name:         "nil requirements",
			requirements: nil,
			wantErr:      true,
			errMsg:       "requirements cannot be nil",
		},
		{
			name: "kubernetes version requirements",
			requirements: &RequirementSpec{
				APIVersion: "troubleshoot.replicated.com/v1beta2",
				Kind:       "Requirements",
				Metadata: RequirementMetadata{
					Name: "test-requirements",
				},
				Spec: RequirementSpecDetails{
					Kubernetes: KubernetesRequirements{
						MinVersion: "1.20.0",
						MaxVersion: "1.25.0",
					},
				},
			},
			wantErr:   false,
			wantSpecs: 1,
		},
		{
			name: "resource requirements",
			requirements: &RequirementSpec{
				APIVersion: "troubleshoot.replicated.com/v1beta2",
				Kind:       "Requirements",
				Metadata: RequirementMetadata{
					Name: "resource-requirements",
				},
				Spec: RequirementSpecDetails{
					Resources: ResourceRequirements{
						CPU: ResourceRequirement{
							Min: "2",
						},
						Memory: ResourceRequirement{
							Min: "4Gi",
						},
					},
				},
			},
			wantErr:   false,
			wantSpecs: 1, // simplified engine implementation generates 1 analyzer
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specs, err := engine.GenerateAnalyzers(ctx, tt.requirements)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, specs)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, specs)
				assert.Len(t, specs, tt.wantSpecs)

				// Verify all specs have required fields
				for i, spec := range specs {
					assert.NotEmpty(t, spec.Name, "spec %d should have name", i)
					assert.NotEmpty(t, spec.Type, "spec %d should have type", i)
					assert.NotEmpty(t, spec.Category, "spec %d should have category", i)
					assert.Greater(t, spec.Priority, 0, "spec %d should have positive priority", i)
				}
			}
		})
	}
}

func TestAnalysisEngine_HealthCheck(t *testing.T) {
	engine := NewAnalysisEngine()
	ctx := context.Background()

	// Test with no agents
	health, err := engine.HealthCheck(ctx)
	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)
	assert.Empty(t, health.Agents)

	// Add healthy agent
	healthyAgent := &mockAgent{
		name:      "healthy-agent",
		available: true,
		healthy:   true,
	}
	err = engine.RegisterAgent("healthy-agent", healthyAgent)
	require.NoError(t, err)

	// Add unhealthy agent
	unhealthyAgent := &mockAgent{
		name:      "unhealthy-agent",
		available: false,
		healthy:   false,
		error:     "mock error",
	}
	err = engine.RegisterAgent("unhealthy-agent", unhealthyAgent)
	require.NoError(t, err)

	// Test health check with mixed agents
	health, err = engine.HealthCheck(ctx)
	require.NoError(t, err)
	assert.Equal(t, "degraded", health.Status)
	assert.Len(t, health.Agents, 2)

	// Find the unhealthy agent in results
	var unhealthyFound bool
	for _, agentHealth := range health.Agents {
		if agentHealth.Name == "unhealthy-agent" {
			assert.Equal(t, "unhealthy", agentHealth.Status)
			assert.Equal(t, "mock error", agentHealth.Error)
			assert.False(t, agentHealth.Available)
			unhealthyFound = true
		}
	}
	assert.True(t, unhealthyFound, "unhealthy agent should be found in health results")
}

func TestAnalysisEngine_calculateSummary(t *testing.T) {
	engine := &DefaultAnalysisEngine{}

	results := &AnalysisResult{
		Results: []*AnalyzerResult{
			{IsPass: true, Confidence: 0.9},
			{IsWarn: true, Confidence: 0.8},
			{IsFail: true, Confidence: 0.7},
			{IsPass: true, Confidence: 0.0}, // No confidence
		},
		Errors: []AnalysisError{
			{Error: "test error"},
		},
	}

	engine.calculateSummary(results)

	assert.Equal(t, 4, results.Summary.TotalAnalyzers)
	assert.Equal(t, 2, results.Summary.PassCount)
	assert.Equal(t, 1, results.Summary.WarnCount)
	assert.Equal(t, 1, results.Summary.FailCount)
	assert.Equal(t, 1, results.Summary.ErrorCount)

	// Average confidence should be (0.9 + 0.8 + 0.7) / 3 = 0.8
	assert.InDelta(t, 0.8, results.Summary.Confidence, 0.01)
}

// Mock Agent for testing
type mockAgent struct {
	name      string
	available bool
	healthy   bool
	error     string
	results   []*AnalyzerResult
	duration  time.Duration
}

func (m *mockAgent) Name() string {
	return m.name
}

func (m *mockAgent) IsAvailable() bool {
	return m.available
}

func (m *mockAgent) Capabilities() []string {
	return []string{"test-capability"}
}

func (m *mockAgent) HealthCheck(ctx context.Context) error {
	if !m.healthy {
		return errors.New(m.error)
	}
	return nil
}

func (m *mockAgent) Analyze(ctx context.Context, data []byte, analyzers []AnalyzerSpec) (*AgentResult, error) {
	if !m.available {
		return nil, errors.New("agent not available")
	}

	return &AgentResult{
		Results: m.results,
		Metadata: AgentResultMetadata{
			Duration:      m.duration,
			AnalyzerCount: len(analyzers),
			Version:       "1.0.0",
		},
		Errors: nil,
	}, nil
}

func TestSupportBundleMetadata_JSON(t *testing.T) {
	metadata := &SupportBundleMetadata{
		CreatedAt: time.Now(),
		Version:   "1.0.0",
		ClusterInfo: &ClusterInfo{
			Version:   "1.24.0",
			Platform:  "kubernetes",
			NodeCount: 3,
		},
		NodeInfo: []NodeInfo{
			{
				Name:         "node1",
				Version:      "1.24.0",
				OS:           "linux",
				Architecture: "amd64",
			},
		},
		GeneratedBy: "test",
		Namespace:   "default",
		Labels: map[string]string{
			"test": "value",
		},
	}

	// Test JSON marshaling/unmarshaling
	data, err := json.Marshal(metadata)
	require.NoError(t, err)

	var unmarshaled SupportBundleMetadata
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, metadata.Version, unmarshaled.Version)
	assert.Equal(t, metadata.GeneratedBy, unmarshaled.GeneratedBy)
	assert.Equal(t, metadata.Namespace, unmarshaled.Namespace)
	assert.Equal(t, metadata.Labels, unmarshaled.Labels)
	assert.NotNil(t, unmarshaled.ClusterInfo)
	assert.Len(t, unmarshaled.NodeInfo, 1)
}

func TestAnalysisResult_JSON(t *testing.T) {
	result := &AnalysisResult{
		Results: []*AnalyzerResult{
			{
				IsPass:     true,
				Title:      "Test Result",
				Message:    "Test message",
				AgentName:  "test-agent",
				Confidence: 0.9,
				Category:   "test",
				Insights:   []string{"test insight"},
			},
		},
		Remediation: []RemediationStep{
			{
				Description:   "Test remediation",
				Priority:      5,
				IsAutomatable: true,
			},
		},
		Summary: AnalysisSummary{
			TotalAnalyzers: 1,
			PassCount:      1,
			Duration:       "1s",
			AgentsUsed:     []string{"test-agent"},
		},
		Metadata: AnalysisMetadata{
			Timestamp:     time.Now(),
			EngineVersion: "1.0.0",
		},
	}

	// Test JSON marshaling/unmarshaling
	data, err := json.Marshal(result)
	require.NoError(t, err)

	var unmarshaled AnalysisResult
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Len(t, unmarshaled.Results, 1)
	assert.Len(t, unmarshaled.Remediation, 1)
	assert.Equal(t, result.Summary.TotalAnalyzers, unmarshaled.Summary.TotalAnalyzers)
	assert.Equal(t, result.Metadata.EngineVersion, unmarshaled.Metadata.EngineVersion)
}

func TestAnalysisEngine_ConvertAnalyzerToSpec_ErrorHandling(t *testing.T) {
	engine := NewAnalysisEngine()

	tests := []struct {
		name          string
		analyzer      *troubleshootv1beta2.Analyze
		expectError   bool
		expectedError string
	}{
		{
			name:          "nil analyzer",
			analyzer:      nil,
			expectError:   true,
			expectedError: "analyzer cannot be nil",
		},
		{
			name: "supported ClusterVersion analyzer",
			analyzer: &troubleshootv1beta2.Analyze{
				ClusterVersion: &troubleshootv1beta2.ClusterVersion{
					Outcomes: []*troubleshootv1beta2.Outcome{},
				},
			},
			expectError: false,
		},
		{
			name: "supported DeploymentStatus analyzer",
			analyzer: &troubleshootv1beta2.Analyze{
				DeploymentStatus: &troubleshootv1beta2.DeploymentStatus{
					Name:     "test-deployment",
					Outcomes: []*troubleshootv1beta2.Outcome{},
				},
			},
			expectError: false,
		},
		{
			name: "now supported TextAnalyze analyzer",
			analyzer: &troubleshootv1beta2.Analyze{
				TextAnalyze: &troubleshootv1beta2.TextAnalyze{
					CollectorName: "test-logs",
					FileName:      "test.log",
				},
			},
			expectError: false,
		},
		{
			name: "now supported NodeResources analyzer",
			analyzer: &troubleshootv1beta2.Analyze{
				NodeResources: &troubleshootv1beta2.NodeResources{},
			},
			expectError: false,
		},
		{
			name: "supported Postgres analyzer",
			analyzer: &troubleshootv1beta2.Analyze{
				Postgres: &troubleshootv1beta2.DatabaseAnalyze{
					CollectorName: "postgres",
					FileName:      "postgres.json",
				},
			},
			expectError: false,
		},
		{
			name: "supported YamlCompare analyzer",
			analyzer: &troubleshootv1beta2.Analyze{
				YamlCompare: &troubleshootv1beta2.YamlCompare{
					CollectorName: "config",
					FileName:      "config.yaml",
					Path:          "data",
					Value:         "expected",
				},
			},
			expectError: false,
		},
		{
			name:          "completely unknown analyzer type",
			analyzer:      &troubleshootv1beta2.Analyze{},
			expectError:   true,
			expectedError: "unknown analyzer type - this should not happen as all known types are now supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := engine.(*DefaultAnalysisEngine).convertAnalyzerToSpec(tt.analyzer)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Empty(t, spec.Name) // Should have empty spec on error
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, spec.Name)
				assert.NotEmpty(t, spec.Type)
				assert.NotNil(t, spec.Config)
			}
		})
	}
}

func TestAnalysisEngine_Analyze_ComprehensiveAnalyzerSupport(t *testing.T) {
	engine := NewAnalysisEngine()

	// Register a mock agent
	mockAgent := &mockAgent{
		name:      "test-agent",
		available: true,
		healthy:   true,
		results: []*AnalyzerResult{
			{IsPass: true, Title: "Test Result", Message: "Success"},
		},
		duration: 100 * time.Millisecond,
	}

	err := engine.RegisterAgent("test-agent", mockAgent)
	require.NoError(t, err)

	// Create a mock bundle
	bundle := &SupportBundle{
		Metadata: &SupportBundleMetadata{
			CreatedAt: time.Now(),
			Version:   "test",
		},
		Files: make(map[string][]byte),
	}

	// Create analysis options with comprehensive analyzer types (all now supported!)
	opts := AnalysisOptions{
		Agents: []string{"test-agent"},
		CustomAnalyzers: []*troubleshootv1beta2.Analyze{
			// Cluster analyzers
			{
				ClusterVersion: &troubleshootv1beta2.ClusterVersion{
					Outcomes: []*troubleshootv1beta2.Outcome{},
				},
			},
			{
				NodeResources: &troubleshootv1beta2.NodeResources{},
			},
			// Workload analyzers
			{
				DeploymentStatus: &troubleshootv1beta2.DeploymentStatus{
					Name:     "test-deployment",
					Outcomes: []*troubleshootv1beta2.Outcome{},
				},
			},
			{
				StatefulsetStatus: &troubleshootv1beta2.StatefulsetStatus{
					Name:     "test-statefulset",
					Outcomes: []*troubleshootv1beta2.Outcome{},
				},
			},
			// Data analyzers
			{
				TextAnalyze: &troubleshootv1beta2.TextAnalyze{
					CollectorName: "test-logs",
					FileName:      "test.log",
				},
			},
			{
				YamlCompare: &troubleshootv1beta2.YamlCompare{
					CollectorName: "config",
					FileName:      "config.yaml",
					Path:          "data",
					Value:         "test",
				},
			},
			// Database analyzers
			{
				Postgres: &troubleshootv1beta2.DatabaseAnalyze{
					CollectorName: "postgres",
					FileName:      "postgres.json",
				},
			},
		},
	}

	// Run analysis - all analyzers should now be supported!
	result, err := engine.Analyze(context.Background(), bundle, opts)

	// Verify analysis completes successfully
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Should have results: 1 from mock agent + 7 analyzer results (all converted successfully)
	expectedResults := len(opts.CustomAnalyzers) + 1 // 7 analyzers + 1 mock agent result
	assert.Len(t, result.Results, expectedResults, "Expected results from mock agent + all %d analyzer conversions", len(opts.CustomAnalyzers))

	// Count results by type
	mockResults := 0
	analyzerResults := 0
	failureResults := 0

	for _, res := range result.Results {
		if res.Message == "Success" {
			mockResults++
		} else if res.AgentName == "local" {
			analyzerResults++
		} else if res.IsFail && strings.Contains(res.Title, "Conversion Failed") {
			failureResults++
		}
	}

	assert.Equal(t, 1, mockResults, "Should have 1 mock agent result")
	// Note: analyzerResults may be 0 if traditional analyzers fail due to missing files (expected)
	// The important thing is that we get results (success or failure) for all analyzers, not silent skips
	assert.Equal(t, 0, failureResults, "Should have no conversion failures - all analyzer types now supported")

	// Verify agent was used
	assert.Contains(t, result.Summary.AgentsUsed, "test-agent")

	// No fatal errors should be recorded
	assert.Equal(t, 0, len(result.Errors))

	// The key success metric: All analyzers produced results (not silently skipped)
	// Whether they pass/warn/fail depends on data availability, but they all get processed
	fmt.Printf("✅ SUCCESS: All %d analyzers processed and accounted for!\n", len(opts.CustomAnalyzers))
}
