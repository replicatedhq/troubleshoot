package analyzer

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPhase2_MultiAgentIntegration tests Phase 2 multi-agent coordination
func TestPhase2_MultiAgentIntegration(t *testing.T) {
	ctx := context.Background()
	engine := NewAnalysisEngine()

	// Register multiple agents to test Phase 2 coordination
	localAgent := &integrationTestAgent{
		name:      "local",
		available: true,
		results: []*AnalyzerResult{
			{
				IsPass:     true,
				Title:      "Local Pod Check",
				Message:    "Local analysis passed",
				AgentName:  "local",
				Confidence: 0.9,
			},
		},
	}

	hostedAgent := &integrationTestAgent{
		name:      "hosted",
		available: true,
		results: []*AnalyzerResult{
			{
				IsWarn:     true,
				Title:      "Hosted AI Analysis",
				Message:    "AI detected potential issue",
				AgentName:  "hosted",
				Confidence: 0.8,
			},
		},
	}

	ollamaAgent := &integrationTestAgent{
		name:      "ollama",
		available: true,
		results: []*AnalyzerResult{
			{
				IsFail:     true,
				Title:      "Ollama Deep Analysis",
				Message:    "LLM found critical issue",
				AgentName:  "ollama",
				Confidence: 0.85,
				Remediation: &RemediationStep{
					Description:   "LLM-suggested remediation",
					Priority:      9,
					Category:      "ai-suggested",
					IsAutomatable: false,
				},
			},
		},
	}

	// Register all agents
	require.NoError(t, engine.RegisterAgent("local", localAgent))
	require.NoError(t, engine.RegisterAgent("hosted", hostedAgent))
	require.NoError(t, engine.RegisterAgent("ollama", ollamaAgent))

	// Test multi-agent analysis
	bundle := &SupportBundle{
		Files: map[string][]byte{
			"test.json": []byte(`{"test": "data"}`),
		},
		Metadata: &SupportBundleMetadata{
			CreatedAt: time.Now(),
			Version:   "1.0.0",
		},
	}

	// Test with multiple agents
	opts := AnalysisOptions{
		Agents:             []string{"local", "hosted", "ollama"},
		IncludeRemediation: true,
	}

	result, err := engine.Analyze(ctx, bundle, opts)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify multi-agent coordination
	assert.Len(t, result.Summary.AgentsUsed, 3)
	assert.Contains(t, result.Summary.AgentsUsed, "local")
	assert.Contains(t, result.Summary.AgentsUsed, "hosted")
	assert.Contains(t, result.Summary.AgentsUsed, "ollama")

	// Verify results from all agents
	assert.Len(t, result.Results, 3)

	agentResults := make(map[string]*AnalyzerResult)
	for _, r := range result.Results {
		agentResults[r.AgentName] = r
	}

	assert.Contains(t, agentResults, "local")
	assert.Contains(t, agentResults, "hosted")
	assert.Contains(t, agentResults, "ollama")

	// Verify summary counts
	assert.Equal(t, 1, result.Summary.PassCount)
	assert.Equal(t, 1, result.Summary.WarnCount)
	assert.Equal(t, 1, result.Summary.FailCount)

	// Verify remediation from LLM agent
	assert.NotEmpty(t, result.Remediation)
	assert.Equal(t, "ai-suggested", result.Remediation[0].Category)
}

// TestPhase2_AgentFallback tests fallback mechanisms
func TestPhase2_AgentFallback(t *testing.T) {
	ctx := context.Background()
	engine := NewAnalysisEngine()

	// Register agents with different availability
	availableAgent := &integrationTestAgent{
		name:      "available",
		available: true,
		results: []*AnalyzerResult{
			{IsPass: true, Title: "Available Agent Result", AgentName: "available"},
		},
	}

	unavailableAgent := &integrationTestAgent{
		name:      "unavailable",
		available: false,
	}

	require.NoError(t, engine.RegisterAgent("available", availableAgent))
	require.NoError(t, engine.RegisterAgent("unavailable", unavailableAgent))

	bundle := &SupportBundle{
		Files:    map[string][]byte{"test.json": []byte(`{}`)},
		Metadata: &SupportBundleMetadata{CreatedAt: time.Now()},
	}

	// Test with mixed availability
	opts := AnalysisOptions{
		Agents: []string{"available", "unavailable"},
	}

	result, err := engine.Analyze(ctx, bundle, opts)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should only use available agent
	assert.Len(t, result.Summary.AgentsUsed, 1)
	assert.Contains(t, result.Summary.AgentsUsed, "available")
	assert.Len(t, result.Results, 1)
	assert.Equal(t, "available", result.Results[0].AgentName)
}

// TestPhase2_AgentHealthCheck tests health checking for all agent types
func TestPhase2_AgentHealthCheck(t *testing.T) {
	ctx := context.Background()
	engine := NewAnalysisEngine()

	// Register agents with different health states
	healthyAgent := &integrationTestAgent{
		name:      "healthy",
		available: true,
		healthy:   true,
	}

	unhealthyAgent := &integrationTestAgent{
		name:      "unhealthy",
		available: true,
		healthy:   false,
		error:     "simulated agent error",
	}

	require.NoError(t, engine.RegisterAgent("healthy", healthyAgent))
	require.NoError(t, engine.RegisterAgent("unhealthy", unhealthyAgent))

	health, err := engine.HealthCheck(ctx)
	require.NoError(t, err)
	require.NotNil(t, health)

	// Should be degraded due to unhealthy agent
	assert.Equal(t, "degraded", health.Status)
	assert.Len(t, health.Agents, 2)

	// Find and verify agent health states
	healthMap := make(map[string]AgentHealth)
	for _, agentHealth := range health.Agents {
		healthMap[agentHealth.Name] = agentHealth
	}

	assert.Equal(t, "healthy", healthMap["healthy"].Status)
	assert.True(t, healthMap["healthy"].Available)

	assert.Equal(t, "unhealthy", healthMap["unhealthy"].Status)
	assert.Equal(t, "simulated agent error", healthMap["unhealthy"].Error)
	assert.True(t, healthMap["unhealthy"].Available)
}

// integrationTestAgent is used for testing (avoiding import cycles)
type integrationTestAgent struct {
	name      string
	available bool
	healthy   bool
	error     string
	results   []*AnalyzerResult
}

func (a *integrationTestAgent) Name() string {
	return a.name
}

func (a *integrationTestAgent) IsAvailable() bool {
	return a.available
}

func (a *integrationTestAgent) Capabilities() []string {
	return []string{"test-capability"}
}

func (a *integrationTestAgent) HealthCheck(ctx context.Context) error {
	if !a.healthy {
		if a.error != "" {
			return errors.New(a.error)
		}
		return errors.New("agent unhealthy")
	}
	return nil
}

func (a *integrationTestAgent) Analyze(ctx context.Context, data []byte, analyzers []AnalyzerSpec) (*AgentResult, error) {
	if !a.available {
		return nil, errors.New("agent not available")
	}

	return &AgentResult{
		Results: a.results,
		Metadata: AgentResultMetadata{
			Duration:      time.Millisecond * 50,
			AnalyzerCount: len(analyzers),
			Version:       "1.0.0",
		},
	}, nil
}
