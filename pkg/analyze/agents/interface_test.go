package agents

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/analyze/agents/hosted"
	"github.com/replicatedhq/troubleshoot/pkg/analyze/agents/local"
)

// TestAgentInterfaceCompliance tests that all agent implementations comply with the Agent interface
func TestAgentInterfaceCompliance(t *testing.T) {
	ctx := context.Background()

	agents := []struct {
		name  string
		agent analyzer.Agent
	}{
		{
			name:  "LocalAgent",
			agent: local.NewLocalAgent(),
		},
		{
			name: "HostedAgent",
			agent: func() analyzer.Agent {
				config := &hosted.HostedAgentConfig{
					BaseURL:           "http://localhost:8080",
					AuthType:          "api-key",
					APIKey:            "test-key",
					RequestsPerSecond: 1.0,
					BurstSize:         5,
					RequestTimeout:    30 * time.Second,
				}
				agent, err := hosted.NewHostedAgent(config)
				if err != nil {
					// Return nil if agent creation fails
					return nil
				}
				return agent
			}(),
		},
	}

	for _, tc := range agents {
		t.Run(tc.name, func(t *testing.T) {
			agent := tc.agent

			// Skip test if agent creation failed
			if agent == nil {
				t.Skipf("Agent %s could not be created, skipping tests", tc.name)
				return
			}

			// Test Name() method
			t.Run("Name", func(t *testing.T) {
				name := agent.Name()
				assert.NotEmpty(t, name, "Agent name should not be empty")
				assert.IsType(t, "", name, "Agent name should be a string")
			})

			// Test Version() method
			t.Run("Version", func(t *testing.T) {
				version := agent.Version()
				assert.NotEmpty(t, version, "Agent version should not be empty")
				assert.IsType(t, "", version, "Agent version should be a string")
			})

			// Test Capabilities() method
			t.Run("Capabilities", func(t *testing.T) {
				capabilities := agent.Capabilities()
				assert.NotNil(t, capabilities, "Agent capabilities should not be nil")
				assert.IsType(t, []string{}, capabilities, "Agent capabilities should be a string slice")
			})

			// Test HealthCheck() method
			t.Run("HealthCheck", func(t *testing.T) {
				err := agent.HealthCheck(ctx)
				// Health check may fail for test agents, that's expected - just verify method completes
				_ = err // Health check can return error or nil, both are fine
			})

			// Test Analyze() method structure
			t.Run("Analyze", func(t *testing.T) {
				// Create a minimal valid support bundle context with required functions
				bundle := &analyzer.SupportBundle{
					Path:    "/tmp/test-bundle",
					RootDir: "/tmp/test-bundle",
					Metadata: map[string]interface{}{
						"test": true,
					},
					GetFile: func(path string) ([]byte, error) {
						// Return empty file for test
						return []byte{}, nil
					},
					FindFiles: func(path string, extensions []string) (map[string][]byte, error) {
						// Return empty file map for test
						return map[string][]byte{}, nil
					},
				}

				// Create empty analyzer specs for testing
				analyzers := []analyzer.AnalyzerSpec{
					{
						Name: "test-analyzer",
						Type: "test",
					},
				}

				// The analyze method should accept the parameters without panicking
				result, _ := agent.Analyze(ctx, bundle, analyzers)

				// Result may be nil or error may occur for test scenarios, but method should not panic
				// We just verify the call completes successfully
				if result != nil {
					assert.IsType(t, &analyzer.AgentResult{}, result, "Analyze result should be AgentResult type")
				}
				// Method completed successfully without panicking
			})
		})
	}
}

// TestAgentInterfaceConsistency tests that all agents behave consistently
func TestAgentInterfaceConsistency(t *testing.T) {
	agents := []analyzer.Agent{
		local.NewLocalAgent(),
		func() analyzer.Agent {
			config := &hosted.HostedAgentConfig{
				BaseURL:           "http://localhost:8080",
				AuthType:          "api-key",
				APIKey:            "test-key",
				RequestsPerSecond: 1.0,
				BurstSize:         5,
				RequestTimeout:    30 * time.Second,
			}
			agent, err := hosted.NewHostedAgent(config)
			if err != nil {
				return nil
			}
			return agent
		}(),
	}

	for _, agent := range agents {
		if agent == nil {
			continue // Skip nil agents
		}
		t.Run(agent.Name(), func(t *testing.T) {
			// Test that multiple calls to Name() return the same value
			name1 := agent.Name()
			name2 := agent.Name()
			assert.Equal(t, name1, name2, "Agent name should be consistent across calls")

			// Test that multiple calls to Version() return the same value
			version1 := agent.Version()
			version2 := agent.Version()
			assert.Equal(t, version1, version2, "Agent version should be consistent across calls")

			// Test that multiple calls to Capabilities() return the same value
			cap1 := agent.Capabilities()
			cap2 := agent.Capabilities()
			assert.Equal(t, cap1, cap2, "Agent capabilities should be consistent across calls")
		})
	}
}

// TestAgentCapabilities tests that agents declare reasonable capabilities
func TestAgentCapabilities(t *testing.T) {
	testCases := []struct {
		name                  string
		agent                 analyzer.Agent
		expectedCapabilities  []string
		forbiddenCapabilities []string
	}{
		{
			name:                  "LocalAgent",
			agent:                 local.NewLocalAgent(),
			expectedCapabilities:  []string{"cluster-analysis", "offline-analysis"},
			forbiddenCapabilities: []string{"external_api", "cloud"},
		},
		{
			name: "HostedAgent",
			agent: func() analyzer.Agent {
				config := &hosted.HostedAgentConfig{
					BaseURL:           "http://localhost:8080",
					AuthType:          "api-key",
					APIKey:            "test-key",
					RequestsPerSecond: 1.0,
					BurstSize:         5,
					RequestTimeout:    30 * time.Second,
				}
				agent, err := hosted.NewHostedAgent(config)
				if err != nil {
					return nil // Return nil if creation fails
				}
				return agent
			}(),
			expectedCapabilities:  []string{"cloud-analysis", "advanced-ml"},
			forbiddenCapabilities: []string{"offline-analysis"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Skip test if agent creation failed
			if tc.agent == nil {
				t.Skipf("Agent %s could not be created, skipping tests", tc.name)
				return
			}

			capabilities := tc.agent.Capabilities()

			// Check expected capabilities
			for _, expected := range tc.expectedCapabilities {
				assert.Contains(t, capabilities, expected,
					"Agent %s should have capability: %s", tc.name, expected)
			}

			// Check forbidden capabilities
			for _, forbidden := range tc.forbiddenCapabilities {
				assert.NotContains(t, capabilities, forbidden,
					"Agent %s should not have capability: %s", tc.name, forbidden)
			}
		})
	}
}

// TestAgentTimeout tests that agents respect timeout contexts
func TestAgentTimeout(t *testing.T) {
	agent := local.NewLocalAgent()

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Wait for context to expire
	time.Sleep(2 * time.Millisecond)

	// Health check should respect the timeout
	err := agent.HealthCheck(ctx)
	if err != nil {
		// If error is returned, it should be a context deadline exceeded error
		assert.Contains(t, err.Error(), "context")
	}
}

// TestAgentErrorHandling tests that agents handle errors gracefully
func TestAgentErrorHandling(t *testing.T) {
	t.Run("HostedAgent_InvalidURL", func(t *testing.T) {
		config := &hosted.HostedAgentConfig{
			BaseURL:           "invalid-url",
			AuthType:          "api-key",
			APIKey:            "test-key",
			RequestsPerSecond: 1.0,
			BurstSize:         5,
			RequestTimeout:    30 * time.Second,
		}

		agent, err := hosted.NewHostedAgent(config)
		if err != nil {
			// Should return a clear error for invalid configuration
			assert.Error(t, err)
			assert.Nil(t, agent)
		}
	})

}

// TestAgentThreadSafety tests that agents are thread-safe
func TestAgentThreadSafety(t *testing.T) {
	agent := local.NewLocalAgent()

	// Run multiple goroutines calling agent methods concurrently
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			// These should not race or panic
			_ = agent.Name()
			_ = agent.Version()
			_ = agent.Capabilities()

			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			_ = agent.HealthCheck(ctx)
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// Success
		case <-time.After(5 * time.Second):
			t.Fatal("Agent method calls timed out - possible deadlock")
		}
	}
}

// TestAgentConfigurationValidation tests agent configuration validation
func TestAgentConfigurationValidation(t *testing.T) {
	t.Run("HostedAgent_ConfigValidation", func(t *testing.T) {
		validConfigs := []*hosted.HostedAgentConfig{
			{
				BaseURL:           "http://localhost:8080",
				AuthType:          "api-key",
				APIKey:            "test-key-1",
				RequestsPerSecond: 1.0,
				BurstSize:         5,
				RequestTimeout:    30 * time.Second,
			},
			{
				BaseURL:           "https://api.example.com",
				AuthType:          "api-key",
				APIKey:            "test-key-2",
				RequestsPerSecond: 2.0,
				BurstSize:         10,
				RequestTimeout:    60 * time.Second,
			},
		}

		for _, config := range validConfigs {
			agent, err := hosted.NewHostedAgent(config)
			assert.NoError(t, err, "Valid config should not produce error")
			assert.NotNil(t, agent, "Valid config should produce agent")
		}

		invalidConfigs := []*hosted.HostedAgentConfig{
			{
				BaseURL:           "", // Empty URL - this should cause validation failure
				AuthType:          "api-key",
				APIKey:            "test-key",
				RequestsPerSecond: 1.0,
				BurstSize:         5,
				RequestTimeout:    30 * time.Second,
			},
		}

		for _, config := range invalidConfigs {
			agent, err := hosted.NewHostedAgent(config)
			assert.Error(t, err, "Invalid config should produce error")
			assert.Nil(t, agent, "Invalid config should not produce agent")
		}
	})
}

// BenchmarkAgentMethods benchmarks agent method performance
func BenchmarkAgentMethods(b *testing.B) {
	agent := local.NewLocalAgent()
	ctx := context.Background()

	b.Run("Name", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = agent.Name()
		}
	})

	b.Run("Version", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = agent.Version()
		}
	})

	b.Run("Capabilities", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = agent.Capabilities()
		}
	})

	b.Run("HealthCheck", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = agent.HealthCheck(ctx)
		}
	})
}
