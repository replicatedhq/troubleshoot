package analyzer

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/pkg/errors"
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
