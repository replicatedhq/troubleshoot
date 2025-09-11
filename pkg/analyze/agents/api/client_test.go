package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func TestDefaultClientConfig(t *testing.T) {
	config := DefaultClientConfig()

	assert.Equal(t, 10*time.Second, config.ConnectTimeout)
	assert.Equal(t, 30*time.Second, config.RequestTimeout)
	assert.Equal(t, 90*time.Second, config.IdleConnTimeout)
	assert.Equal(t, 10, config.MaxIdleConns)
	assert.Equal(t, 5, config.MaxIdleConnsPerHost)
	assert.Equal(t, 10, config.MaxConnsPerHost)
	assert.False(t, config.InsecureSkipVerify)
	assert.Equal(t, "troubleshoot-agent/1.0", config.UserAgent)
	assert.NotNil(t, config.DefaultHeaders)
	assert.True(t, config.EnableCompression)
	assert.False(t, config.EnableDebugLogging)
}

func TestNewHTTPClient(t *testing.T) {
	tests := []struct {
		name        string
		config      *ClientConfig
		credStore   CredentialStore
		rateLimiter RateLimiter
		retryPolicy RetryPolicy
		shouldError bool
	}{
		{
			name:        "default config",
			config:      nil,
			credStore:   nil,
			rateLimiter: nil,
			retryPolicy: DefaultRetryPolicy(),
			shouldError: false,
		},
		{
			name:        "custom config",
			config:      DefaultClientConfig(),
			credStore:   &mockCredentialStore{},
			rateLimiter: rate.NewLimiter(rate.Limit(10), 5),
			retryPolicy: DefaultRetryPolicy(),
			shouldError: false,
		},
		{
			name: "invalid proxy URL",
			config: &ClientConfig{
				ConnectTimeout: 10 * time.Second,
				RequestTimeout: 30 * time.Second,
				ProxyURL:       "invalid://proxy:url:with:too:many:colons",
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewHTTPClient(tt.config, tt.credStore, tt.rateLimiter, tt.retryPolicy)

			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.NotNil(t, client.client)
				assert.NotNil(t, client.config)
			}
		})
	}
}

func TestHTTPClient_Do_BasicRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/test", r.URL.Path)
		assert.Equal(t, "test-value", r.URL.Query().Get("param1"))
		assert.Equal(t, "troubleshoot-agent/1.0", r.Header.Get("User-Agent"))
		assert.Equal(t, "custom-value", r.Header.Get("X-Custom-Header"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "success"}`))
	}))
	defer server.Close()

	client, err := NewHTTPClient(nil, nil, nil, DefaultRetryPolicy())
	require.NoError(t, err)

	req := &Request{
		Method: "GET",
		URL:    server.URL + "/test",
		Headers: map[string]string{
			"X-Custom-Header": "custom-value",
		},
		QueryParams: map[string]string{
			"param1": "test-value",
		},
		RequestID:     "test-request-1",
		OperationName: "test-operation",
	}

	ctx := context.Background()
	resp, err := client.Do(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "OK", resp.Status)
	assert.Contains(t, resp.Headers, "Content-Type")
	assert.Equal(t, `{"status": "success"}`, string(resp.Body))
	assert.Equal(t, "test-request-1", resp.RequestID)
	assert.Greater(t, resp.RequestDuration, time.Duration(0))
}

func TestHTTPClient_Do_PostWithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body := make([]byte, r.ContentLength)
		r.Body.Read(body)
		assert.Contains(t, string(body), "test_field")
		assert.Contains(t, string(body), "test_value")

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"created": true}`))
	}))
	defer server.Close()

	client, err := NewHTTPClient(nil, nil, nil, DefaultRetryPolicy())
	require.NoError(t, err)

	req := &Request{
		Method: "POST",
		URL:    server.URL + "/create",
		Body: map[string]interface{}{
			"test_field": "test_value",
			"number":     42,
		},
	}

	ctx := context.Background()
	resp, err := client.Do(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Contains(t, string(resp.Body), "created")
}

func TestHTTPClient_Do_WithAuthentication(t *testing.T) {
	tests := []struct {
		name         string
		authType     string
		validateAuth func(t *testing.T, r *http.Request)
	}{
		{
			name:     "bearer token",
			authType: "bearer",
			validateAuth: func(t *testing.T, r *http.Request) {
				auth := r.Header.Get("Authorization")
				assert.True(t, strings.HasPrefix(auth, "Bearer "))
				assert.Equal(t, "Bearer test-token", auth)
			},
		},
		{
			name:     "api key",
			authType: "api-key",
			validateAuth: func(t *testing.T, r *http.Request) {
				apiKey := r.Header.Get("X-API-Key")
				assert.Equal(t, "test-api-key", apiKey)
			},
		},
		{
			name:     "basic auth",
			authType: "basic",
			validateAuth: func(t *testing.T, r *http.Request) {
				username, password, ok := r.BasicAuth()
				assert.True(t, ok)
				assert.Equal(t, "testuser", username)
				assert.Equal(t, "testpass", password)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tt.validateAuth(t, r)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"authenticated": true}`))
			}))
			defer server.Close()

			credStore := &mockCredentialStore{}
			client, err := NewHTTPClient(nil, credStore, nil, DefaultRetryPolicy())
			require.NoError(t, err)

			req := &Request{
				Method:      "GET",
				URL:         server.URL + "/protected",
				RequireAuth: true,
				AuthType:    tt.authType,
			}

			ctx := context.Background()
			resp, err := client.Do(ctx, req)

			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Contains(t, string(resp.Body), "authenticated")
		})
	}
}

func TestHTTPClient_Do_RateLimiting(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"request": %d}`, requestCount)))
	}))
	defer server.Close()

	rateLimiter := rate.NewLimiter(rate.Limit(2), 1) // 2 requests per second, burst of 1
	client, err := NewHTTPClient(nil, nil, rateLimiter, DefaultRetryPolicy())
	require.NoError(t, err)

	req := &Request{
		Method: "GET",
		URL:    server.URL + "/test",
	}

	ctx := context.Background()

	// First request should be immediate
	start := time.Now()
	resp, err := client.Do(ctx, req)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Less(t, elapsed, 100*time.Millisecond)

	// Second request should be rate limited
	start = time.Now()
	resp, err = client.Do(ctx, req)
	elapsed = time.Since(start)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Greater(t, elapsed, 100*time.Millisecond) // Should have been delayed
}

func TestHTTPClient_Do_RetryPolicy(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error": "temporarily unavailable"}`))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true}`))
		}
	}))
	defer server.Close()

	retryPolicy := RetryPolicy{
		MaxAttempts:          3,
		BaseDelay:            10 * time.Millisecond,
		MaxDelay:             100 * time.Millisecond,
		BackoffMultiplier:    2.0,
		RetryableStatusCodes: []int{503},
		RetryableErrors:      []string{"timeout"},
	}

	client, err := NewHTTPClient(nil, nil, nil, retryPolicy)
	require.NoError(t, err)

	req := &Request{
		Method: "GET",
		URL:    server.URL + "/test",
	}

	ctx := context.Background()
	resp, err := client.Do(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 3, requestCount) // Should have retried twice
	assert.Equal(t, 3, resp.AttemptCount)
	assert.Contains(t, string(resp.Body), "success")
}

func TestHTTPClient_Do_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond) // Longer than client timeout
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"delayed": true}`))
	}))
	defer server.Close()

	config := DefaultClientConfig()
	config.RequestTimeout = 50 * time.Millisecond // Very short timeout

	client, err := NewHTTPClient(config, nil, nil, DefaultRetryPolicy())
	require.NoError(t, err)

	req := &Request{
		Method: "GET",
		URL:    server.URL + "/slow",
	}

	ctx := context.Background()
	_, err = client.Do(ctx, req)

	assert.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "timeout")
}

func TestHTTPClient_Do_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"response": "ok"}`))
	}))
	defer server.Close()

	client, err := NewHTTPClient(nil, nil, nil, DefaultRetryPolicy())
	require.NoError(t, err)

	req := &Request{
		Method: "GET",
		URL:    server.URL + "/test",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	_, err = client.Do(ctx, req)

	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()

	assert.Equal(t, 3, policy.MaxAttempts)
	assert.Equal(t, 1*time.Second, policy.BaseDelay)
	assert.Equal(t, 30*time.Second, policy.MaxDelay)
	assert.Equal(t, 2.0, policy.BackoffMultiplier)
	assert.Contains(t, policy.RetryableStatusCodes, 429)
	assert.Contains(t, policy.RetryableStatusCodes, 503)
	assert.Contains(t, policy.RetryableErrors, "timeout")
}

func TestRetryPolicy_ShouldRetry(t *testing.T) {
	policy := DefaultRetryPolicy()

	tests := []struct {
		name       string
		statusCode int
		err        error
		expected   bool
	}{
		{
			name:       "retryable status code 503",
			statusCode: 503,
			err:        nil,
			expected:   true,
		},
		{
			name:       "retryable status code 429",
			statusCode: 429,
			err:        nil,
			expected:   true,
		},
		{
			name:       "non-retryable status code 404",
			statusCode: 404,
			err:        nil,
			expected:   false,
		},
		{
			name:       "timeout error",
			statusCode: 200,
			err:        fmt.Errorf("request timeout occurred"),
			expected:   true,
		},
		{
			name:       "connection error",
			statusCode: 200,
			err:        fmt.Errorf("connection refused"),
			expected:   true,
		},
		{
			name:       "non-retryable error",
			statusCode: 200,
			err:        fmt.Errorf("invalid request format"),
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := policy.ShouldRetry(tt.statusCode, tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRetryPolicy_CalculateDelay(t *testing.T) {
	policy := RetryPolicy{
		BaseDelay:         1 * time.Second,
		MaxDelay:          10 * time.Second,
		BackoffMultiplier: 2.0,
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{attempt: 1, expected: 0},               // First attempt has no delay
		{attempt: 2, expected: 2 * time.Second}, // 1 * 2 * (2-1) = 2
		{attempt: 3, expected: 4 * time.Second}, // 1 * 2 * (3-1) = 4
		{attempt: 4, expected: 6 * time.Second}, // 1 * 2 * (4-1) = 6
		{attempt: 5, expected: 8 * time.Second}, // 1 * 2 * (5-1) = 8
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			delay := policy.CalculateDelay(tt.attempt)
			assert.Equal(t, tt.expected, delay)
		})
	}
}

// Mock credential store for testing
type mockCredentialStore struct{}

func (m *mockCredentialStore) GetCredentials(authType string) (*Credentials, error) {
	switch authType {
	case "bearer":
		return &Credentials{Token: "test-token"}, nil
	case "api-key":
		return &Credentials{APIKey: "test-api-key"}, nil
	case "basic":
		return &Credentials{Username: "testuser", Password: "testpass"}, nil
	default:
		return nil, fmt.Errorf("unsupported auth type: %s", authType)
	}
}

func (m *mockCredentialStore) SetCredentials(authType string, creds *Credentials) error {
	return nil
}

func (m *mockCredentialStore) RefreshCredentials(authType string) (*Credentials, error) {
	return m.GetCredentials(authType)
}

func BenchmarkHTTPClient_Do(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"benchmark": true}`))
	}))
	defer server.Close()

	client, err := NewHTTPClient(nil, nil, nil, DefaultRetryPolicy())
	require.NoError(b, err)

	req := &Request{
		Method: "GET",
		URL:    server.URL + "/benchmark",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Do(ctx, req)
		if err != nil {
			b.Fatalf("Request failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			b.Fatalf("Unexpected status code: %d", resp.StatusCode)
		}
	}
}

func BenchmarkRetryPolicy_CalculateDelay(b *testing.B) {
	policy := DefaultRetryPolicy()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = policy.CalculateDelay(3) // Test with attempt 3
	}
}
