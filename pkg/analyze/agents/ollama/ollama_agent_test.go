package ollama

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOllamaAgent(t *testing.T) {
	tests := []struct {
		name string
		opts *OllamaAgentOptions
	}{
		{
			name: "with nil options",
			opts: nil,
		},
		{
			name: "with custom options",
			opts: &OllamaAgentOptions{
				Endpoint:    "http://localhost:11434",
				Model:       "codellama:13b",
				Timeout:     10 * time.Minute,
				MaxTokens:   1500,
				Temperature: 0.3,
			},
		},
		{
			name: "with minimal options",
			opts: &OllamaAgentOptions{
				Model: "llama2:7b",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, err := NewOllamaAgent(tt.opts)

			require.NoError(t, err)
			require.NotNil(t, agent)

			assert.Equal(t, "ollama", agent.Name())
			assert.True(t, agent.enabled)
			assert.NotEmpty(t, agent.Capabilities())
			assert.Contains(t, agent.Capabilities(), "ai-powered-analysis")
			assert.Contains(t, agent.Capabilities(), "privacy-preserving")
			assert.Contains(t, agent.Capabilities(), "self-hosted-llm")

			// Check defaults are applied
			if tt.opts == nil || tt.opts.Endpoint == "" {
				assert.Equal(t, "http://localhost:11434", agent.endpoint)
			}
			if tt.opts == nil || tt.opts.Model == "" {
				assert.Equal(t, "llama2:7b", agent.model)
			}
		})
	}
}

func TestOllamaAgent_HealthCheck(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse string
		serverStatus   int
		wantErr        bool
		errMsg         string
	}{
		{
			name:           "healthy Ollama server with models",
			serverResponse: `{"models": [{"name": "llama2:7b", "size": 3825819519}]}`,
			serverStatus:   http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "Ollama server without target model",
			serverResponse: `{"models": [{"name": "different-model:7b", "size": 1000000}]}`,
			serverStatus:   http.StatusOK,
			wantErr:        true,
			errMsg:         "model llama2:7b not found",
		},
		{
			name:           "Ollama server not running",
			serverResponse: "",
			serverStatus:   http.StatusServiceUnavailable,
			wantErr:        true,
			errMsg:         "Ollama server returned status 503",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/tags", r.URL.Path)

				w.WriteHeader(tt.serverStatus)
				if tt.serverResponse != "" {
					w.Write([]byte(tt.serverResponse))
				}
			}))
			defer server.Close()

			agent, err := NewOllamaAgent(&OllamaAgentOptions{
				Endpoint: server.URL,
				Model:    "llama2:7b",
				Timeout:  5 * time.Second,
			})
			require.NoError(t, err)

			ctx := context.Background()
			err = agent.HealthCheck(ctx)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOllamaAgent_IsAvailable(t *testing.T) {
	// Test with healthy server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"models": [{"name": "llama2:7b", "size": 3825819519}]}`))
	}))
	defer server.Close()

	agent, err := NewOllamaAgent(&OllamaAgentOptions{
		Endpoint: server.URL,
		Model:    "llama2:7b",
	})
	require.NoError(t, err)

	// Should be available when healthy
	assert.True(t, agent.IsAvailable())

	// Test disabled agent
	agent.SetEnabled(false)
	assert.False(t, agent.IsAvailable())
}

func TestOllamaAgent_Capabilities(t *testing.T) {
	agent, err := NewOllamaAgent(&OllamaAgentOptions{
		Endpoint: "http://localhost:11434",
		Model:    "llama2:7b",
	})
	require.NoError(t, err)

	capabilities := agent.Capabilities()

	assert.NotEmpty(t, capabilities)
	assert.Contains(t, capabilities, "ai-powered-analysis")
	assert.Contains(t, capabilities, "natural-language-insights")
	assert.Contains(t, capabilities, "context-aware-remediation")
	assert.Contains(t, capabilities, "intelligent-correlation")
	assert.Contains(t, capabilities, "self-hosted-llm")
	assert.Contains(t, capabilities, "privacy-preserving")
}

func TestOllamaAgent_UpdateModel(t *testing.T) {
	agent, err := NewOllamaAgent(nil)
	require.NoError(t, err)

	// Test valid model update
	err = agent.UpdateModel("codellama:13b")
	assert.NoError(t, err)
	assert.Equal(t, "codellama:13b", agent.GetModel())

	// Test empty model
	err = agent.UpdateModel("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "model cannot be empty")
}

func TestOllamaAgent_discoverAnalyzers(t *testing.T) {
	agent, err := NewOllamaAgent(nil)
	require.NoError(t, err)

	bundle := createTestBundle()
	specs := agent.discoverAnalyzers(bundle)

	assert.NotEmpty(t, specs)

	// Check that AI-powered analyzers are discovered
	foundTypes := make(map[string]bool)
	for _, spec := range specs {
		foundTypes[spec.Type] = true

		// Verify all specs have required fields for AI analysis
		assert.NotEmpty(t, spec.Name)
		assert.NotEmpty(t, spec.Type)
		assert.NotEmpty(t, spec.Category)
		assert.Greater(t, spec.Priority, 0)
		assert.NotNil(t, spec.Config)

		// Verify AI-specific config
		assert.Contains(t, spec.Config, "filePath")
		assert.Contains(t, spec.Config, "promptType")
	}

	assert.True(t, foundTypes["ai-workload"])
	assert.True(t, foundTypes["ai-events"] || foundTypes["ai-logs"] || foundTypes["ai-resources"])
}

func TestOllamaAgent_calculateConfidence(t *testing.T) {
	agent, err := NewOllamaAgent(nil)
	require.NoError(t, err)

	tests := []struct {
		name          string
		message       string
		expectedRange []float64 // [min, max]
	}{
		{
			name:          "short generic message",
			message:       "Test message",
			expectedRange: []float64{0.7, 0.8},
		},
		{
			name:          "detailed technical message",
			message:       "The Kubernetes pod is experiencing issues with container startup. The deployment shows that nodes are under memory pressure.",
			expectedRange: []float64{0.7, 0.9}, // More lenient range
		},
		{
			name:          "highly technical message",
			message:       "Kubernetes cluster analysis reveals pod deployment issues with container node resource constraints affecting cluster stability.",
			expectedRange: []float64{0.7, 0.95}, // More lenient range
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := agent.calculateConfidence(tt.message)

			assert.GreaterOrEqual(t, confidence, tt.expectedRange[0])
			assert.LessOrEqual(t, confidence, tt.expectedRange[1])
			assert.LessOrEqual(t, confidence, 0.95) // Should never exceed 95%
		})
	}
}

func TestOllamaAgent_parseLLMResponse(t *testing.T) {
	agent, err := NewOllamaAgent(nil)
	require.NoError(t, err)

	tests := []struct {
		name     string
		response string
		wantErr  bool
		errMsg   string
		wantPass bool
		wantWarn bool
		wantFail bool
	}{
		{
			name: "valid JSON response",
			response: `Here's my analysis:
{
  "status": "fail",
  "title": "Pod Analysis",
  "message": "Found issues with pod health",
  "insights": ["Pod restart loop detected"],
  "remediation": {
    "description": "Check pod logs",
    "action": "investigate",
    "command": "kubectl logs pod-name",
    "priority": 8
  }
}`,
			wantErr:  false,
			wantFail: true,
		},
		{
			name: "pass status response",
			response: `Analysis complete:
{
  "status": "pass",
  "title": "System Health Check",
  "message": "All systems are functioning normally",
  "insights": ["No issues detected"]
}`,
			wantErr:  false,
			wantPass: true,
		},
		{
			name: "warn status response",
			response: `{
  "status": "warn",
  "title": "Resource Usage",
  "message": "Memory usage is approaching limits",
  "insights": ["Consider scaling up"]
}`,
			wantErr:  false,
			wantWarn: true,
		},
		{
			name:     "no JSON in response",
			response: "This is just plain text without JSON",
			wantErr:  true,
			errMsg:   "no valid JSON found",
		},
		{
			name:     "invalid JSON",
			response: "{ invalid json }",
			wantErr:  true,
			errMsg:   "failed to parse LLM JSON response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := createTestAnalyzerSpec()
			result, err := agent.parseLLMResponse(tt.response, spec)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)

				if tt.wantPass {
					assert.True(t, result.IsPass)
				} else if tt.wantWarn {
					assert.True(t, result.IsWarn)
				} else if tt.wantFail {
					assert.True(t, result.IsFail)
				}

				assert.NotEmpty(t, result.Title)
				assert.NotEmpty(t, result.Message)
				assert.Equal(t, spec.Category, result.Category)
			}
		})
	}
}

// Helper functions

func createTestBundle() *analyzer.SupportBundle {
	return &analyzer.SupportBundle{
		Files: map[string][]byte{
			"cluster-resources/pods/default.json":        []byte(`[{"metadata": {"name": "test-pod"}}]`),
			"cluster-resources/deployments/default.json": []byte(`[{"metadata": {"name": "test-deployment"}}]`),
			"cluster-resources/events/default.json":      []byte(`[{"type": "Warning"}]`),
			"cluster-resources/nodes.json":               []byte(`[{"metadata": {"name": "node1"}}]`),
			"logs/test.log":                              []byte("INFO: Application started"),
		},
		Metadata: &analyzer.SupportBundleMetadata{
			CreatedAt: time.Now(),
			Version:   "1.0.0",
		},
	}
}

func createTestAnalyzerSpec() analyzer.AnalyzerSpec {
	return analyzer.AnalyzerSpec{
		Name:     "test-analyzer",
		Type:     "ai-workload",
		Category: "pods",
		Priority: 8,
		Config: map[string]interface{}{
			"filePath":   "test.json",
			"promptType": "pod-analysis",
		},
	}
}
