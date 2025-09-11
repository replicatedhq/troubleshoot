package analyzer

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnalysisResultSerialization tests serialization of analysis results
func TestAnalysisResultSerialization(t *testing.T) {
	originalResult := EnhancedAnalyzerResult{
		// Basic analyzer result fields
		IsPass:  false,
		IsFail:  true,
		IsWarn:  false,
		Strict:  true,
		Title:   "CPU Usage Analysis",
		Message: "CPU usage is consistently above 90%",
		URI:     "/docs/cpu-analysis",
		IconKey: "warning",

		// Enhanced fields
		Confidence:  0.85,
		Impact:      "high",
		Explanation: "CPU usage is consistently above 90% which may impact application performance",
		Evidence: []string{
			"CPU utilization metrics show sustained high usage",
			"Load average is above recommended thresholds",
			"Process analysis shows resource contention",
		},
		RootCause:     "Insufficient CPU allocation for current workload",
		AgentUsed:     "local-agent",
		RelatedIssues: []string{"memory-pressure-001", "pod-restart-002"},

		Remediation: &RemediationStep{
			ID:          "remediation-001",
			Title:       "Scale CPU Resources",
			Description: "Increase CPU allocation for the workload",
			Category:    "immediate",
			Priority:    1,
			Commands: []string{
				"kubectl patch deployment app -p '{\"spec\":{\"resources\":{\"requests\":{\"cpu\":\"2\"}}}}'",
				"kubectl rollout status deployment/app",
			},
			Manual: []string{
				"1. Open the deployment YAML file",
				"2. Increase CPU requests and limits",
				"3. Apply the changes using kubectl apply",
			},
			Links: []string{
				"https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/",
			},
			Validation: &ValidationStep{
				Description: "Verify CPU usage has decreased",
				Commands: []string{
					"kubectl top pods",
					"kubectl get deployment app -o yaml | grep cpu",
				},
				Expected: "CPU usage below 70%",
			},
			Metadata: map[string]string{
				"scaling_factor": "2x",
				"estimated_time": "5 minutes",
			},
		},
	}

	t.Run("JSON Serialization", func(t *testing.T) {
		// Serialize to JSON
		jsonData, err := json.Marshal(originalResult)
		require.NoError(t, err, "Should serialize to JSON without error")
		assert.NotEmpty(t, jsonData, "JSON data should not be empty")

		// Deserialize from JSON
		var deserializedResult EnhancedAnalyzerResult
		err = json.Unmarshal(jsonData, &deserializedResult)
		require.NoError(t, err, "Should deserialize from JSON without error")

		// Verify core fields
		assert.Equal(t, originalResult.IsPass, deserializedResult.IsPass)
		assert.Equal(t, originalResult.IsFail, deserializedResult.IsFail)
		assert.Equal(t, originalResult.IsWarn, deserializedResult.IsWarn)
		assert.Equal(t, originalResult.Title, deserializedResult.Title)
		assert.Equal(t, originalResult.Message, deserializedResult.Message)
		assert.Equal(t, originalResult.URI, deserializedResult.URI)
		assert.Equal(t, originalResult.IconKey, deserializedResult.IconKey)
		assert.Equal(t, originalResult.Confidence, deserializedResult.Confidence)
		assert.Equal(t, originalResult.Impact, deserializedResult.Impact)
		assert.Equal(t, originalResult.Explanation, deserializedResult.Explanation)
		assert.Equal(t, originalResult.Evidence, deserializedResult.Evidence)
		assert.Equal(t, originalResult.RootCause, deserializedResult.RootCause)
		assert.Equal(t, originalResult.AgentUsed, deserializedResult.AgentUsed)
		assert.Equal(t, originalResult.RelatedIssues, deserializedResult.RelatedIssues)

		// Verify remediation step
		if originalResult.Remediation != nil {
			require.NotNil(t, deserializedResult.Remediation)
			originalRem := originalResult.Remediation
			deserializedRem := deserializedResult.Remediation

			assert.Equal(t, originalRem.ID, deserializedRem.ID)
			assert.Equal(t, originalRem.Title, deserializedRem.Title)
			assert.Equal(t, originalRem.Category, deserializedRem.Category)
			assert.Equal(t, originalRem.Priority, deserializedRem.Priority)
			assert.Equal(t, originalRem.Commands, deserializedRem.Commands)
			assert.Equal(t, originalRem.Manual, deserializedRem.Manual)
			assert.Equal(t, originalRem.Links, deserializedRem.Links)
		}
	})

	t.Run("JSON Pretty Print", func(t *testing.T) {
		prettyJSON, err := json.MarshalIndent(originalResult, "", "  ")
		require.NoError(t, err, "Should serialize to pretty JSON")
		assert.Contains(t, string(prettyJSON), "CPU Usage Analysis")
		assert.Contains(t, string(prettyJSON), "remediation")
		assert.Contains(t, string(prettyJSON), "explanation")
	})
}

// TestRemediationStepSerialization tests serialization of remediation steps
func TestRemediationStepSerialization(t *testing.T) {
	testCases := []struct {
		name string
		step RemediationStep
	}{
		{
			name: "Command Step",
			step: RemediationStep{
				ID:          "cmd-001",
				Title:       "Scale Deployment",
				Description: "Scale deployment to handle increased load",
				Category:    "immediate",
				Priority:    1,
				Commands: []string{
					"kubectl scale deployment app --replicas=3",
					"kubectl rollout status deployment/app",
				},
				Metadata: map[string]string{
					"type": "scaling",
					"tool": "kubernetes",
				},
			},
		},
		{
			name: "Manual Step",
			step: RemediationStep{
				ID:          "manual-001",
				Title:       "Update Configuration",
				Description: "Manually update application configuration",
				Category:    "short-term",
				Priority:    2,
				Manual: []string{
					"Open configuration file",
					"Update the timeout value to 30s",
					"Save and restart application",
				},
				Metadata: map[string]string{
					"type":           "manual",
					"estimated_time": "30 minutes",
				},
			},
		},
		{
			name: "Complete Step with All Fields",
			step: RemediationStep{
				ID:          "complete-001",
				Title:       "Comprehensive Fix",
				Description: "A comprehensive fix with all possible fields",
				Category:    "immediate",
				Priority:    1,
				Commands: []string{
					"kubectl apply -f security-policy.yaml",
				},
				Manual: []string{
					"Review security policy before applying",
					"Monitor for any security alerts",
				},
				Links: []string{
					"https://kubernetes.io/docs/concepts/services-networking/network-policies/",
				},
				Validation: &ValidationStep{
					Description: "Verify security policy is active",
					Commands: []string{
						"kubectl get networkpolicy",
					},
					Expected: "security-policy",
				},
				Metadata: map[string]string{
					"type":           "security",
					"estimated_time": "60 minutes",
					"skill_level":    "expert",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Serialize to JSON
			jsonData, err := json.Marshal(tc.step)
			require.NoError(t, err, "Should serialize step to JSON")

			// Deserialize from JSON
			var deserializedStep RemediationStep
			err = json.Unmarshal(jsonData, &deserializedStep)
			require.NoError(t, err, "Should deserialize step from JSON")

			// Verify basic fields
			assert.Equal(t, tc.step.ID, deserializedStep.ID)
			assert.Equal(t, tc.step.Title, deserializedStep.Title)
			assert.Equal(t, tc.step.Description, deserializedStep.Description)
			assert.Equal(t, tc.step.Category, deserializedStep.Category)
			assert.Equal(t, tc.step.Priority, deserializedStep.Priority)
			assert.Equal(t, tc.step.Commands, deserializedStep.Commands)
			assert.Equal(t, tc.step.Manual, deserializedStep.Manual)
			assert.Equal(t, tc.step.Links, deserializedStep.Links)
			assert.Equal(t, tc.step.Metadata, deserializedStep.Metadata)

			// Verify validation step if present
			if tc.step.Validation != nil {
				require.NotNil(t, deserializedStep.Validation)
				assert.Equal(t, tc.step.Validation.Description, deserializedStep.Validation.Description)
				assert.Equal(t, tc.step.Validation.Commands, deserializedStep.Validation.Commands)
				assert.Equal(t, tc.step.Validation.Expected, deserializedStep.Validation.Expected)
			}
		})
	}
}

// TestAnalysisOptionsSerialization tests serialization of analysis options
func TestAnalysisOptionsSerialization(t *testing.T) {
	originalOptions := AnalysisOptions{
		// Agent selection and fallback
		PreferredAgents:    []string{"local", "hosted"},
		FallbackAgents:     []string{"local"},
		RequireAllAgents:   false,
		HybridMode:         true,
		AgentFailurePolicy: "continue",

		// Analysis configuration
		IncludeRemediation:  true,
		GenerateInsights:    true,
		ConfidenceThreshold: 0.7,
		MaxProcessingTime:   5 * time.Minute,
		EnableCorrelation:   true,

		// Data sensitivity and privacy
		DataSensitivityLevel: "internal",
		AllowExternalAPIs:    true,
		RequireLocalOnly:     false,

		// Quality and performance
		MinConfidenceScore: 0.6,
		MaxCostPerAnalysis: 1.00,

		// Custom configuration
		CustomConfig: map[string]string{
			"feature_flags": "correlation,insights",
			"log_level":     "info",
		},
	}

	t.Run("JSON Serialization", func(t *testing.T) {
		jsonData, err := json.Marshal(originalOptions)
		require.NoError(t, err, "Should serialize options to JSON")

		var deserializedOptions AnalysisOptions
		err = json.Unmarshal(jsonData, &deserializedOptions)
		require.NoError(t, err, "Should deserialize options from JSON")

		// Verify agent selection options
		assert.Equal(t, originalOptions.PreferredAgents, deserializedOptions.PreferredAgents)
		assert.Equal(t, originalOptions.FallbackAgents, deserializedOptions.FallbackAgents)
		assert.Equal(t, originalOptions.HybridMode, deserializedOptions.HybridMode)
		assert.Equal(t, originalOptions.AgentFailurePolicy, deserializedOptions.AgentFailurePolicy)

		// Verify analysis configuration
		assert.Equal(t, originalOptions.IncludeRemediation, deserializedOptions.IncludeRemediation)
		assert.Equal(t, originalOptions.GenerateInsights, deserializedOptions.GenerateInsights)
		assert.Equal(t, originalOptions.ConfidenceThreshold, deserializedOptions.ConfidenceThreshold)

		// Verify data sensitivity options
		assert.Equal(t, originalOptions.DataSensitivityLevel, deserializedOptions.DataSensitivityLevel)
		assert.Equal(t, originalOptions.AllowExternalAPIs, deserializedOptions.AllowExternalAPIs)

		// Verify custom config
		assert.Equal(t, originalOptions.CustomConfig, deserializedOptions.CustomConfig)
	})
}

// TestAnalysisInsightSerialization tests serialization of analysis insights
func TestAnalysisInsightSerialization(t *testing.T) {
	originalInsight := AnalysisInsight{
		ID:          "insight-001",
		Title:       "Memory and CPU Correlation",
		Description: "Memory usage spikes correlate with CPU throttling events",
		Type:        "correlation",
		Confidence:  0.82,
		Evidence: []string{
			"Memory usage increased by 150% during CPU throttling periods",
			"Garbage collection events coincide with CPU spikes",
			"Container restarts occurred during resource contention",
		},
		Impact: "high",
	}

	jsonData, err := json.Marshal(originalInsight)
	require.NoError(t, err, "Should serialize insight to JSON")

	var deserializedInsight AnalysisInsight
	err = json.Unmarshal(jsonData, &deserializedInsight)
	require.NoError(t, err, "Should deserialize insight from JSON")

	assert.Equal(t, originalInsight.ID, deserializedInsight.ID)
	assert.Equal(t, originalInsight.Title, deserializedInsight.Title)
	assert.Equal(t, originalInsight.Description, deserializedInsight.Description)
	assert.Equal(t, originalInsight.Type, deserializedInsight.Type)
	assert.Equal(t, originalInsight.Confidence, deserializedInsight.Confidence)
	assert.Equal(t, originalInsight.Evidence, deserializedInsight.Evidence)
	assert.Equal(t, originalInsight.Impact, deserializedInsight.Impact)
}

// TestSerializationBackwardCompatibility tests that serialized data remains compatible
func TestSerializationBackwardCompatibility(t *testing.T) {
	// Simulate old JSON format without some newer fields
	oldFormatJSON := `{
		"isPass": false,
		"isFail": false,
		"isWarn": true,
		"title": "Legacy Analysis Result",
		"message": "Analysis result from older version"
	}`

	var result EnhancedAnalyzerResult
	err := json.Unmarshal([]byte(oldFormatJSON), &result)
	require.NoError(t, err, "Should handle legacy JSON format")

	assert.Equal(t, "Legacy Analysis Result", result.Title)
	assert.Equal(t, "Analysis result from older version", result.Message)
	assert.True(t, result.IsWarn)
	assert.Equal(t, 0.0, result.Confidence) // Should default to zero value
	assert.Empty(t, result.Evidence)        // Should default to empty slice
	assert.Nil(t, result.Remediation)       // Should default to nil
}

// TestSerializationEdgeCases tests serialization edge cases
func TestSerializationEdgeCases(t *testing.T) {
	t.Run("Empty Result", func(t *testing.T) {
		emptyResult := EnhancedAnalyzerResult{}

		jsonData, err := json.Marshal(emptyResult)
		require.NoError(t, err)

		var deserializedResult EnhancedAnalyzerResult
		err = json.Unmarshal(jsonData, &deserializedResult)
		require.NoError(t, err)

		assert.Equal(t, emptyResult, deserializedResult)
	})

	t.Run("Null Fields", func(t *testing.T) {
		jsonWithNulls := `{
			"title": "Test Result",
			"remediation": null,
			"evidence": null
		}`

		var result EnhancedAnalyzerResult
		err := json.Unmarshal([]byte(jsonWithNulls), &result)
		require.NoError(t, err)

		assert.Equal(t, "Test Result", result.Title)
		assert.Nil(t, result.Remediation)
		assert.Nil(t, result.Evidence)
	})

	t.Run("Large Data", func(t *testing.T) {
		// Create a result with large amounts of data
		largeResult := EnhancedAnalyzerResult{
			Title:   "Large Result",
			Message: "A test result with large amounts of data",
		}

		// Add many evidence items
		for i := 0; i < 1000; i++ {
			largeResult.Evidence = append(largeResult.Evidence,
				"Evidence item #"+fmt.Sprintf("%d", i)+" with some detailed information about the analysis")
		}

		jsonData, err := json.Marshal(largeResult)
		require.NoError(t, err, "Should handle large data serialization")

		var deserializedResult EnhancedAnalyzerResult
		err = json.Unmarshal(jsonData, &deserializedResult)
		require.NoError(t, err, "Should handle large data deserialization")

		assert.Equal(t, len(largeResult.Evidence), len(deserializedResult.Evidence))
		assert.Equal(t, largeResult.Title, deserializedResult.Title)
	})
}

// BenchmarkSerialization benchmarks serialization performance
func BenchmarkSerialization(b *testing.B) {
	result := EnhancedAnalyzerResult{
		Title:      "Benchmark Test Result",
		Message:    "Analysis result used for benchmarking serialization performance",
		IsFail:     true,
		Confidence: 0.85,
		Impact:     "high",
		Evidence:   []string{"evidence1", "evidence2", "evidence3"},
		Remediation: &RemediationStep{
			ID:          "remedy-001",
			Title:       "Fix Issue",
			Description: "Fix the identified issue",
			Category:    "immediate",
			Priority:    1,
		},
	}

	b.Run("Marshal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = json.Marshal(result)
		}
	})

	jsonData, _ := json.Marshal(result)

	b.Run("Unmarshal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var r EnhancedAnalyzerResult
			_ = json.Unmarshal(jsonData, &r)
		}
	})
}
