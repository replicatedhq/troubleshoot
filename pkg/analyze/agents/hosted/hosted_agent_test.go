package hosted

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHostedAgent(t *testing.T) {
	tests := []struct {
		name    string
		opts    *HostedAgentOptions
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil options",
			opts:    nil,
			wantErr: true,
			errMsg:  "options cannot be nil",
		},
		{
			name: "missing endpoint",
			opts: &HostedAgentOptions{
				APIKey: "test-key",
			},
			wantErr: true,
			errMsg:  "endpoint is required",
		},
		{
			name: "missing API key",
			opts: &HostedAgentOptions{
				Endpoint: "https://api.example.com",
			},
			wantErr: true,
			errMsg:  "API key is required",
		},
		{
			name: "invalid endpoint URL",
			opts: &HostedAgentOptions{
				Endpoint: "://invalid-url",
				APIKey:   "test-key",
			},
			wantErr: true,
			errMsg:  "invalid endpoint URL",
		},
		{
			name: "valid configuration",
			opts: &HostedAgentOptions{
				Endpoint: "https://api.example.com",
				APIKey:   "test-key",
				Timeout:  30 * time.Second,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, err := NewHostedAgent(tt.opts)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, agent)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, agent)
				assert.Equal(t, "hosted", agent.Name())
				assert.True(t, agent.enabled)
				assert.NotEmpty(t, agent.Capabilities())
			}
		})
	}
}

func TestHostedAgent_HealthCheck(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse int
		serverBody     string
		wantErr        bool
		errMsg         string
	}{
		{
			name:           "healthy service",
			serverResponse: http.StatusOK,
			serverBody:     `{"status": "ok"}`,
			wantErr:        false,
		},
		{
			name:           "service unavailable",
			serverResponse: http.StatusServiceUnavailable,
			serverBody:     `{"error": "service down"}`,
			wantErr:        true,
			errMsg:         "health check failed with status 503",
		},
		{
			name:           "internal server error",
			serverResponse: http.StatusInternalServerError,
			serverBody:     `{"error": "internal error"}`,
			wantErr:        true,
			errMsg:         "health check failed with status 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/health", r.URL.Path)
				assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

				w.WriteHeader(tt.serverResponse)
				w.Write([]byte(tt.serverBody))
			}))
			defer server.Close()

			agent, err := NewHostedAgent(&HostedAgentOptions{
				Endpoint: server.URL,
				APIKey:   "test-key",
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

func TestHostedAgent_IsAvailable(t *testing.T) {
	// Test with healthy server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	agent, err := NewHostedAgent(&HostedAgentOptions{
		Endpoint: server.URL,
		APIKey:   "test-key",
	})
	require.NoError(t, err)

	// Should be available when healthy
	assert.True(t, agent.IsAvailable())

	// Test disabled agent
	agent.SetEnabled(false)
	assert.False(t, agent.IsAvailable())
}

func TestHostedAgent_Capabilities(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	agent, err := NewHostedAgent(&HostedAgentOptions{
		Endpoint: server.URL,
		APIKey:   "test-key",
	})
	require.NoError(t, err)

	capabilities := agent.Capabilities()

	assert.NotEmpty(t, capabilities)
	assert.Contains(t, capabilities, "advanced-analysis")
	assert.Contains(t, capabilities, "ml-powered")
	assert.Contains(t, capabilities, "correlation-detection")
}

func TestHostedAgent_UpdateCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	agent, err := NewHostedAgent(&HostedAgentOptions{
		Endpoint: server.URL,
		APIKey:   "old-key",
	})
	require.NoError(t, err)

	// Test valid credential update
	err = agent.UpdateCredentials("new-key")
	assert.NoError(t, err)

	// Test empty credential
	err = agent.UpdateCredentials("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API key cannot be empty")
}

func TestRateLimiter(t *testing.T) {
	rateLimiter := NewRateLimiter(2) // 2 requests per minute
	ctx := context.Background()

	// First two requests should succeed immediately
	start := time.Now()
	err := rateLimiter.waitForToken(ctx)
	assert.NoError(t, err)
	assert.Less(t, time.Since(start), 100*time.Millisecond)

	err = rateLimiter.waitForToken(ctx)
	assert.NoError(t, err)
	assert.Less(t, time.Since(start), 200*time.Millisecond)

	// Third request should be rate limited (but we won't wait in test)
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()

	err = rateLimiter.waitForToken(ctxWithTimeout)
	assert.Error(t, err) // Should timeout due to rate limiting
}
