package llm

import (
	"testing"

	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/stretchr/testify/assert"
)

func TestDefaultLLMConfig(t *testing.T) {
	config := DefaultLLMConfig(ProviderOllama, "test-model")

	assert.Equal(t, ProviderOllama, config.Provider)
	assert.Equal(t, "test-model", config.Model)
	assert.Equal(t, 0.3, config.Temperature)
	assert.Equal(t, 4096, config.MaxTokens)
	assert.NotEmpty(t, config.SystemPrompt)
	assert.True(t, config.EnablePIIFilter)
	assert.True(t, config.AuditLogging)
	assert.NotNil(t, config.CustomSettings)
}

func TestGetDefaultSystemPrompt(t *testing.T) {
	prompt := GetDefaultSystemPrompt()

	assert.NotEmpty(t, prompt)
	assert.Contains(t, prompt, "Kubernetes")
	assert.Contains(t, prompt, "troubleshooting")
	assert.Contains(t, prompt, "remediation")
	assert.Contains(t, prompt, "JSON")
}

func TestGetDomainSpecificPrompts(t *testing.T) {
	prompts := GetDomainSpecificPrompts()

	// Check that all expected domains are present
	expectedDomains := []string{"kubernetes", "storage", "network", "security", "performance", "compliance"}
	for _, domain := range expectedDomains {
		assert.Contains(t, prompts, domain)
		assert.NotEmpty(t, prompts[domain])
	}

	// Verify specific domain content
	assert.Contains(t, prompts["kubernetes"], "pod scheduling")
	assert.Contains(t, prompts["storage"], "PVC binding")
	assert.Contains(t, prompts["network"], "DNS resolution")
	assert.Contains(t, prompts["security"], "RBAC")
	assert.Contains(t, prompts["performance"], "resource utilization")
	assert.Contains(t, prompts["compliance"], "governance")
}

func TestNewPIIFilter(t *testing.T) {
	// Test enabled filter
	filter := NewPIIFilter(true)
	assert.NotNil(t, filter)
	assert.True(t, filter.enabled)
	assert.NotEmpty(t, filter.patterns)

	// Test disabled filter
	filter = NewPIIFilter(false)
	assert.NotNil(t, filter)
	assert.False(t, filter.enabled)
}

func TestPIIFilter_FilterText(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		input    string
		expected string
	}{
		{
			name:     "filter disabled",
			enabled:  false,
			input:    "user@example.com with password=secret123",
			expected: "user@example.com with password=secret123",
		},
		{
			name:     "filter IP addresses",
			enabled:  true,
			input:    "Server IP is 192.168.1.100 and client is 10.0.0.5",
			expected: "Server IP is XXX.XXX.XXX.XXX and client is XXX.XXX.XXX.XXX",
		},
		{
			name:     "filter email addresses",
			enabled:  true,
			input:    "Contact admin@company.com for help",
			expected: "Contact user@example.com for help",
		},
		{
			name:     "filter secrets",
			enabled:  true,
			input:    "password=mysecret123 and token=abc123def456",
			expected: "password=[REDACTED] and token=[REDACTED-TOKEN]",
		},
		{
			name:     "filter hostnames",
			enabled:  true,
			input:    "Connect to database.company.com on port 5432",
			expected: "Connect to host.example.com on port 5432",
		},
		{
			name:     "multiple filters combined",
			enabled:  true,
			input:    "Email admin@test.com from 10.1.1.1 with password=secret",
			expected: "Email user@example.com from XXX.XXX.XXX.XXX with password=[REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewPIIFilter(tt.enabled)
			result := filter.FilterText(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewPromptBuilder(t *testing.T) {
	systemPrompt := "Test system prompt"
	builder := NewPromptBuilder(systemPrompt, true)

	assert.NotNil(t, builder)
	assert.Equal(t, systemPrompt, builder.systemPrompt)
	assert.NotNil(t, builder.filter)
	assert.True(t, builder.filter.enabled)
}

func TestPromptBuilder_BuildAnalysisPrompt(t *testing.T) {
	builder := NewPromptBuilder(GetDefaultSystemPrompt(), true)

	tests := []struct {
		name         string
		request      *EnhancementRequest
		shouldError  bool
		checkContent []string
	}{
		{
			name:        "nil request",
			request:     nil,
			shouldError: true,
		},
		{
			name: "nil original result",
			request: &EnhancementRequest{
				OriginalResult: nil,
			},
			shouldError: true,
		},
		{
			name: "basic valid request",
			request: &EnhancementRequest{
				OriginalResult: &analyzer.AnalyzeResult{
					Title:   "Test Analyzer",
					IsFail:  true,
					Message: "Test failure message",
				},
				AllResults: []*analyzer.AnalyzeResult{
					{
						Title:   "Related Check",
						IsPass:  true,
						Message: "Related success",
					},
				},
				BundleContext: &BundleContext{
					ClusterVersion: "v1.24.0",
					NodeCount:      3,
					Namespaces:     []string{"default", "kube-system"},
				},
				Options: &EnhancementOptions{
					IncludeRemediation: true,
					EnableCorrelation:  true,
					DetailLevel:        "detailed",
					FocusAreas:         []string{"kubernetes", "security"},
				},
			},
			shouldError: false,
			checkContent: []string{
				"Test Analyzer",
				"Test failure message",
				"FAIL",
				"v1.24.0",
				"3",
				"default, kube-system",
				"Related Check",
				"detailed",
				"kubernetes, security",
				"JSON format",
			},
		},
		{
			name: "request with PII filtering",
			request: &EnhancementRequest{
				OriginalResult: &analyzer.AnalyzeResult{
					Title:   "Database Check",
					IsFail:  true,
					Message: "Connection failed to db.company.com with user admin@company.com",
				},
			},
			shouldError: false,
			checkContent: []string{
				"Database Check",
				"host.example.com", // PII filtered
				"user@example.com", // PII filtered
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt, err := builder.BuildAnalysisPrompt(tt.request)

			if tt.shouldError {
				assert.Error(t, err)
				assert.Empty(t, prompt)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, prompt)

				for _, content := range tt.checkContent {
					assert.Contains(t, prompt, content)
				}
			}
		})
	}
}

func TestPromptBuilder_AreResultsRelated(t *testing.T) {
	builder := NewPromptBuilder("", false)

	tests := []struct {
		name     string
		result1  *analyzer.AnalyzeResult
		result2  *analyzer.AnalyzeResult
		expected bool
	}{
		{
			name: "storage related",
			result1: &analyzer.AnalyzeResult{
				Title:   "Storage Class Check",
				Message: "No default storage class",
			},
			result2: &analyzer.AnalyzeResult{
				Title:   "Persistent Volume Status",
				Message: "Storage issues detected",
			},
			expected: true,
		},
		{
			name: "network related",
			result1: &analyzer.AnalyzeResult{
				Title:   "DNS Resolution",
				Message: "DNS query failed",
			},
			result2: &analyzer.AnalyzeResult{
				Title:   "Network Policy Check",
				Message: "Network connectivity issues",
			},
			expected: true,
		},
		{
			name: "unrelated checks",
			result1: &analyzer.AnalyzeResult{
				Title:   "Storage Class Check",
				Message: "Storage issue",
			},
			result2: &analyzer.AnalyzeResult{
				Title:   "Memory Usage",
				Message: "Memory pressure detected",
			},
			expected: false,
		},
		{
			name: "generic words filtered out",
			result1: &analyzer.AnalyzeResult{
				Title:   "Resource Check Status",
				Message: "Check failed",
			},
			result2: &analyzer.AnalyzeResult{
				Title:   "Usage Status Analyzer",
				Message: "Test result",
			},
			expected: false, // "check", "status", etc. should be filtered out
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.areResultsRelated(tt.result1, tt.result2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewAuditLogger(t *testing.T) {
	// Test enabled logger
	logger := NewAuditLogger(true)
	assert.NotNil(t, logger)
	assert.True(t, logger.enabled)
	assert.Empty(t, logger.logs)

	// Test disabled logger
	logger = NewAuditLogger(false)
	assert.NotNil(t, logger)
	assert.False(t, logger.enabled)
}

func TestAuditLogger_LogRequest(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		logs    []AuditLog
	}{
		{
			name:    "enabled logger",
			enabled: true,
			logs: []AuditLog{
				{
					AgentName:   "test-agent",
					Provider:    "test-provider",
					Model:       "test-model",
					InputSize:   100,
					OutputSize:  200,
					PIIFiltered: true,
					Success:     true,
				},
			},
		},
		{
			name:    "disabled logger",
			enabled: false,
			logs: []AuditLog{
				{
					AgentName: "test-agent",
					Success:   true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewAuditLogger(tt.enabled)

			for _, log := range tt.logs {
				logger.LogRequest(log)
			}

			retrievedLogs := logger.GetLogs()

			if tt.enabled {
				assert.Len(t, retrievedLogs, len(tt.logs))
				if len(retrievedLogs) > 0 {
					assert.Equal(t, tt.logs[0].AgentName, retrievedLogs[0].AgentName)
					assert.NotZero(t, retrievedLogs[0].Timestamp)
				}
			} else {
				assert.Empty(t, retrievedLogs)
			}
		})
	}
}

func TestParseLLMResponse(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		shouldError bool
		validate    func(t *testing.T, result *LLMAnalysisResult)
	}{
		{
			name: "valid JSON response",
			content: `{
				"confidence": 0.9,
				"impact": "HIGH",
				"explanation": "Test explanation",
				"evidence": ["evidence1", "evidence2"],
				"problem": "Test problem",
				"remediation": {
					"id": "test-fix",
					"title": "Test Fix",
					"description": "Fix description",
					"priority": 1
				},
				"correlations": ["related1"],
				"insights": [
					{
						"type": "recommendation",
						"title": "Test Insight",
						"description": "Insight description",
						"confidence": 0.8
					}
				]
			}`,
			shouldError: false,
			validate: func(t *testing.T, result *LLMAnalysisResult) {
				assert.Equal(t, 0.9, result.Confidence)
				assert.Equal(t, "HIGH", result.Impact)
				assert.Equal(t, "Test explanation", result.Explanation)
				assert.Len(t, result.Evidence, 2)
				assert.Equal(t, "Test problem", result.Problem)
				assert.NotNil(t, result.Remediation)
				assert.Equal(t, "test-fix", result.Remediation.ID)
				assert.Len(t, result.Correlations, 1)
				assert.Len(t, result.Insights, 1)
			},
		},
		{
			name: "JSON wrapped in markdown",
			content: "```json\n" + `{
				"confidence": 0.8,
				"impact": "MEDIUM",
				"explanation": "Wrapped explanation",
				"evidence": ["wrapped evidence"],
				"problem": "Wrapped problem"
			}` + "\n```",
			shouldError: false,
			validate: func(t *testing.T, result *LLMAnalysisResult) {
				assert.Equal(t, 0.8, result.Confidence)
				assert.Equal(t, "MEDIUM", result.Impact)
				assert.Equal(t, "Wrapped explanation", result.Explanation)
			},
		},
		{
			name: "invalid confidence range - gets corrected",
			content: `{
				"confidence": 1.5,
				"impact": "",
				"explanation": "Test"
			}`,
			shouldError: false,
			validate: func(t *testing.T, result *LLMAnalysisResult) {
				assert.Equal(t, 0.5, result.Confidence)  // Should be corrected to default
				assert.Equal(t, "MEDIUM", result.Impact) // Should be set to default
			},
		},
		{
			name:        "invalid JSON",
			content:     `{"confidence": invalid}`,
			shouldError: true,
		},
		{
			name:        "empty content",
			content:     "",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseLLMResponse(tt.content)

			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestBuildContextSection(t *testing.T) {
	builder := NewPromptBuilder("", false)

	tests := []struct {
		name     string
		context  *BundleContext
		expected []string
	}{
		{
			name:     "nil context",
			context:  nil,
			expected: []string{"CLUSTER CONTEXT: No additional context available."},
		},
		{
			name: "full context",
			context: &BundleContext{
				ClusterVersion: "v1.24.0",
				NodeCount:      5,
				Namespaces:     []string{"default", "kube-system", "monitoring"},
				CriticalPods: []PodInfo{
					{
						Name:      "api-server",
						Namespace: "kube-system",
						Status:    "CrashLoopBackOff",
						Restarts:  10,
					},
				},
				RecentEvents: []EventInfo{
					{
						Type:    "Warning",
						Reason:  "FailedMount",
						Message: "Unable to mount volume",
					},
				},
				Errors: []string{"DNS resolution failed", "Network timeout"},
			},
			expected: []string{
				"CLUSTER CONTEXT:",
				"Kubernetes Version: v1.24.0",
				"Node Count: 5",
				"Namespaces: default, kube-system, monitoring",
				"Critical Pods with Issues:",
				"kube-system/api-server: CrashLoopBackOff (restarts: 10)",
				"Recent Events:",
				"Warning FailedMount: Unable to mount volume",
				"System Errors: DNS resolution failed, Network timeout",
			},
		},
		{
			name: "minimal context",
			context: &BundleContext{
				ClusterVersion: "v1.25.0",
			},
			expected: []string{
				"CLUSTER CONTEXT:",
				"Kubernetes Version: v1.25.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.buildContextSection(tt.context)

			for _, expected := range tt.expected {
				assert.Contains(t, result, expected)
			}
		})
	}
}

func BenchmarkPIIFilter_FilterText(b *testing.B) {
	filter := NewPIIFilter(true)
	text := "Contact admin@company.com at 192.168.1.100 with password=secret123 and token=abcdef123456"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = filter.FilterText(text)
	}
}

func BenchmarkPromptBuilder_BuildAnalysisPrompt(b *testing.B) {
	builder := NewPromptBuilder(GetDefaultSystemPrompt(), true)

	request := &EnhancementRequest{
		OriginalResult: &analyzer.AnalyzeResult{
			Title:   "Benchmark Test",
			IsFail:  true,
			Message: "Benchmark failure message with admin@test.com and IP 10.0.0.1",
		},
		AllResults: []*analyzer.AnalyzeResult{
			{Title: "Related1", IsPass: true},
			{Title: "Related2", IsWarn: true},
		},
		BundleContext: &BundleContext{
			ClusterVersion: "v1.24.0",
			NodeCount:      3,
		},
		Options: &EnhancementOptions{
			IncludeRemediation: true,
			DetailLevel:        "detailed",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := builder.BuildAnalysisPrompt(request)
		if err != nil {
			b.Fatalf("BuildAnalysisPrompt failed: %v", err)
		}
	}
}
