package local

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLocalAgent(t *testing.T) {
	tests := []struct {
		name string
		opts *LocalAgentOptions
	}{
		{
			name: "with nil options",
			opts: nil,
		},
		{
			name: "with custom options",
			opts: &LocalAgentOptions{
				EnablePlugins:  true,
				PluginDir:      "/tmp/plugins",
				MaxConcurrency: 5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := NewLocalAgent(tt.opts)

			assert.NotNil(t, agent)
			assert.Equal(t, "local", agent.Name())
			assert.True(t, agent.IsAvailable())
			assert.NotEmpty(t, agent.Capabilities())
			assert.Contains(t, agent.Capabilities(), "cluster-analysis")
			assert.Contains(t, agent.Capabilities(), "offline-analysis")
		})
	}
}

func TestLocalAgent_HealthCheck(t *testing.T) {
	agent := NewLocalAgent(nil)
	ctx := context.Background()

	// Test healthy agent
	err := agent.HealthCheck(ctx)
	assert.NoError(t, err)

	// Test disabled agent
	agent.enabled = false
	err = agent.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestLocalAgent_RegisterPlugin(t *testing.T) {
	agent := NewLocalAgent(nil)

	tests := []struct {
		name    string
		plugin  AnalyzerPlugin
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid plugin",
			plugin:  &mockPlugin{name: "test-plugin"},
			wantErr: false,
		},
		{
			name:    "nil plugin",
			plugin:  nil,
			wantErr: true,
			errMsg:  "plugin cannot be nil",
		},
		{
			name:    "empty plugin name",
			plugin:  &mockPlugin{name: ""},
			wantErr: true,
			errMsg:  "plugin name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := agent.RegisterPlugin(tt.plugin)

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

	// Test duplicate plugin registration
	plugin := &mockPlugin{name: "duplicate-plugin"}
	err := agent.RegisterPlugin(plugin)
	require.NoError(t, err)

	err = agent.RegisterPlugin(plugin)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestLocalAgent_Analyze(t *testing.T) {
	agent := NewLocalAgent(nil)
	ctx := context.Background()

	// Test bundle data
	bundle := &analyzer.SupportBundle{
		Files: map[string][]byte{
			"cluster-resources/pods/default.json": []byte(`[
				{
					"metadata": {"name": "test-pod", "namespace": "default"},
					"status": {"phase": "Running"}
				}
			]`),
			"cluster-resources/deployments/default.json": []byte(`[
				{
					"metadata": {"name": "test-deployment"},
					"status": {"replicas": 3, "readyReplicas": 3}
				}
			]`),
			"cluster-resources/events/default.json": []byte(`[
				{
					"type": "Normal",
					"reason": "Started",
					"message": "Container started"
				}
			]`),
		},
		Metadata: &analyzer.SupportBundleMetadata{
			CreatedAt: time.Now(),
			Version:   "1.0.0",
		},
	}

	bundleData, err := json.Marshal(bundle)
	require.NoError(t, err)

	tests := []struct {
		name      string
		data      []byte
		analyzers []analyzer.AnalyzerSpec
		enabled   bool
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "successful analysis with auto-discovery",
			data:      bundleData,
			analyzers: nil, // Will auto-discover
			enabled:   true,
			wantErr:   false,
		},
		{
			name: "successful analysis with specific analyzers",
			data: bundleData,
			analyzers: []analyzer.AnalyzerSpec{
				{
					Name:     "pod-status-check",
					Type:     "workload",
					Category: "pods",
					Config: map[string]interface{}{
						"filePath": "cluster-resources/pods/default.json",
					},
				},
			},
			enabled: true,
			wantErr: false,
		},
		{
			name:      "disabled agent",
			data:      bundleData,
			analyzers: nil,
			enabled:   false,
			wantErr:   true,
			errMsg:    "not enabled",
		},
		{
			name:      "invalid bundle data",
			data:      []byte("invalid json"),
			analyzers: nil,
			enabled:   true,
			wantErr:   true,
			errMsg:    "unmarshal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent.enabled = tt.enabled

			result, err := agent.Analyze(ctx, tt.data, tt.analyzers)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.NotEmpty(t, result.Results)

				// Verify all results have agent name set
				for _, r := range result.Results {
					assert.Equal(t, "local", r.AgentName)
					assert.NotEmpty(t, r.Title)
					assert.True(t, r.IsPass || r.IsWarn || r.IsFail)
				}

				assert.Equal(t, "1.0.0", result.Metadata.Version)
				assert.Greater(t, result.Metadata.Duration.Nanoseconds(), int64(0))
			}
		})
	}
}

func TestLocalAgent_discoverAnalyzers(t *testing.T) {
	agent := NewLocalAgent(nil)

	bundle := &analyzer.SupportBundle{
		Files: map[string][]byte{
			"cluster-resources/pods/default.json":                        []byte("{}"),
			"cluster-resources/deployments/default.json":                 []byte("{}"),
			"cluster-resources/services/default.json":                    []byte("{}"),
			"cluster-resources/events/default.json":                      []byte("{}"),
			"cluster-resources/nodes.json":                               []byte("{}"),
			"cluster-resources/pods/logs/default/test-pod/container.log": []byte("log data"),
		},
	}

	specs := agent.discoverAnalyzers(bundle)

	assert.NotEmpty(t, specs)

	// Check that we have the expected analyzer types
	foundTypes := make(map[string]bool)
	for _, spec := range specs {
		foundTypes[spec.Name] = true

		// Verify all specs have required fields
		assert.NotEmpty(t, spec.Name)
		assert.NotEmpty(t, spec.Type)
		assert.NotEmpty(t, spec.Category)
		assert.Greater(t, spec.Priority, 0)
		assert.NotNil(t, spec.Config)
	}

	assert.True(t, foundTypes["ai-pod-analysis"] || foundTypes["pod-status-check"])
	assert.True(t, foundTypes["ai-deployment-analysis"] || foundTypes["deployment-status-check"])
	assert.True(t, foundTypes["service-check"])
	assert.True(t, foundTypes["ai-event-analysis"] || foundTypes["event-analysis"])
	assert.True(t, foundTypes["ai-resource-analysis"] || foundTypes["node-resources-check"])
	assert.True(t, foundTypes["ai-log-analysis"] || foundTypes["log-analysis"])
}

func TestLocalAgent_analyzePodStatus(t *testing.T) {
	agent := NewLocalAgent(nil)
	ctx := context.Background()

	tests := []struct {
		name     string
		podData  string
		wantPass bool
		wantWarn bool
		wantFail bool
	}{
		{
			name: "healthy pods",
			podData: `[
				{"metadata": {"name": "pod1"}, "status": {"phase": "Running"}},
				{"metadata": {"name": "pod2"}, "status": {"phase": "Running"}}
			]`,
			wantPass: true,
		},
		{
			name: "pods with warnings",
			podData: `[
				{"metadata": {"name": "pod1"}, "status": {"phase": "Running"}},
				{"metadata": {"name": "pod2"}, "status": {"phase": "Pending"}},
				{"metadata": {"name": "pod3"}, "status": {"phase": "Pending"}}
			]`,
			wantWarn: true,
		},
		{
			name: "failed pods",
			podData: `[
				{"metadata": {"name": "pod1"}, "status": {"phase": "Running"}},
				{"metadata": {"name": "pod2"}, "status": {"phase": "Failed"}}
			]`,
			wantFail: true,
		},
		{
			name:     "no pods",
			podData:  `[]`,
			wantWarn: true,
		},
		{
			name:     "invalid JSON",
			podData:  `invalid json`,
			wantFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundle := &analyzer.SupportBundle{
				Files: map[string][]byte{
					"test-pods.json": []byte(tt.podData),
				},
			}

			spec := analyzer.AnalyzerSpec{
				Name:     "pod-status-check",
				Type:     "workload",
				Category: "pods",
				Config: map[string]interface{}{
					"filePath": "test-pods.json",
				},
			}

			result, err := agent.analyzePodStatus(ctx, bundle, spec)
			require.NoError(t, err)
			require.NotNil(t, result)

			assert.Equal(t, "Pod Status Analysis", result.Title)
			assert.Equal(t, "pods", result.Category)

			if tt.wantPass {
				assert.True(t, result.IsPass, "expected pass status")
			} else if tt.wantWarn {
				assert.True(t, result.IsWarn, "expected warn status")
			} else if tt.wantFail {
				assert.True(t, result.IsFail, "expected fail status")
			}

			assert.NotEmpty(t, result.Message)
		})
	}
}

func TestLocalAgent_analyzeNodeResources(t *testing.T) {
	agent := NewLocalAgent(nil)
	ctx := context.Background()

	tests := []struct {
		name     string
		nodeData string
		wantPass bool
		wantFail bool
	}{
		{
			name: "healthy nodes",
			nodeData: `[
				{
					"metadata": {"name": "node1"},
					"status": {
						"conditions": [
							{"type": "Ready", "status": "True"}
						]
					}
				}
			]`,
			wantPass: true,
		},
		{
			name: "unhealthy nodes",
			nodeData: `[
				{
					"metadata": {"name": "node1"},
					"status": {
						"conditions": [
							{"type": "Ready", "status": "False"}
						]
					}
				}
			]`,
			wantFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundle := &analyzer.SupportBundle{
				Files: map[string][]byte{
					"test-nodes.json": []byte(tt.nodeData),
				},
			}

			spec := analyzer.AnalyzerSpec{
				Name:     "node-resources-check",
				Type:     "cluster",
				Category: "nodes",
				Config: map[string]interface{}{
					"filePath": "test-nodes.json",
				},
			}

			result, err := agent.analyzeNodeResources(ctx, bundle, spec)
			require.NoError(t, err)
			require.NotNil(t, result)

			if tt.wantPass {
				assert.True(t, result.IsPass)
			} else if tt.wantFail {
				assert.True(t, result.IsFail)
				assert.NotNil(t, result.Remediation)
			}
		})
	}
}

func TestLocalAgent_analyzeLogs(t *testing.T) {
	agent := NewLocalAgent(nil)
	ctx := context.Background()

	tests := []struct {
		name     string
		logData  string
		wantPass bool
		wantWarn bool
		wantFail bool
	}{
		{
			name:     "clean logs",
			logData:  "INFO: Application started\nINFO: Processing request\nINFO: Request completed",
			wantPass: true,
		},
		{
			name:     "logs with warnings",
			logData:  strings.Repeat("WARN: Connection timeout\n", 25),
			wantWarn: true,
		},
		{
			name:     "logs with errors",
			logData:  strings.Repeat("ERROR: Database connection failed\n", 15),
			wantFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundle := &analyzer.SupportBundle{
				Files: map[string][]byte{
					"test.log": []byte(tt.logData),
				},
			}

			spec := analyzer.AnalyzerSpec{
				Name:     "log-analysis",
				Type:     "logs",
				Category: "logging",
				Config: map[string]interface{}{
					"filePath": "test.log",
				},
			}

			result, err := agent.analyzeLogs(ctx, bundle, spec)
			require.NoError(t, err)
			require.NotNil(t, result)

			if tt.wantPass {
				assert.True(t, result.IsPass)
			} else if tt.wantWarn {
				assert.True(t, result.IsWarn)
			} else if tt.wantFail {
				assert.True(t, result.IsFail)
				assert.NotNil(t, result.Remediation)
			}

			assert.NotNil(t, result.Context)
		})
	}
}

// Mock plugin for testing
type mockPlugin struct {
	name     string
	supports map[string]bool
	result   *analyzer.AnalyzerResult
	error    error
}

func (m *mockPlugin) Name() string {
	return m.name
}

func (m *mockPlugin) Supports(analyzerType string) bool {
	if m.supports == nil {
		return false
	}
	return m.supports[analyzerType]
}

func (m *mockPlugin) Analyze(ctx context.Context, data map[string][]byte, config map[string]interface{}) (*analyzer.AnalyzerResult, error) {
	if m.error != nil {
		return nil, m.error
	}
	return m.result, nil
}
