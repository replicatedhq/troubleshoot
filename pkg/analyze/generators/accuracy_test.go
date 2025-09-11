package generators

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnalyzerGenerationAccuracy tests the accuracy of analyzer generation from requirements
func TestAnalyzerGenerationAccuracy(t *testing.T) {
	generator := NewAnalyzerGenerator()

	t.Run("Basic Generation", func(t *testing.T) {
		// Test basic requirements parsing and generation
		requirements := RequirementSpec{
			APIVersion: "troubleshoot.io/v1beta2",
			Kind:       "AnalyzerRequirements",
			Metadata: RequirementMetadata{
				Name:        "test-requirements",
				Description: "Basic test requirements",
			},
			Spec: RequirementSpecDetails{
				Kubernetes: KubernetesRequirements{
					MinVersion: "1.20.0",
					MaxVersion: "1.28.0",
				},
				Resources: ResourceRequirements{
					CPU: CPURequirement{
						MinCores:       1.0,
						MaxUtilization: 80.0,
					},
				},
			},
		}

		// Convert to categorized requirements
		categorizer := NewRequirementCategorizer()
		categorized, err := categorizer.CategorizeSpec(&requirements)
		require.NoError(t, err, "Should categorize requirements")

		// Test generation
		opts := GenerationOptions{
			PackageName:   "test",
			GenerateTests: true,
			FormatCode:    true,
		}
		result, err := generator.GenerateAnalyzers(categorized, opts)
		require.NoError(t, err, "Should generate analyzers without error")
		require.NotNil(t, result, "Should return a result")

		// Basic validation
		assert.True(t, len(result.GeneratedAnalyzers) > 0, "Should generate at least one analyzer")

		// Check that generated analyzers have required fields
		for _, analyzer := range result.GeneratedAnalyzers {
			assert.NotEmpty(t, analyzer.Name, "Analyzer should have a name")
			assert.NotEmpty(t, analyzer.Description, "Analyzer should have a description")
			assert.NotEmpty(t, analyzer.Type, "Analyzer should have a type")
		}
	})

	t.Run("Kubernetes Requirements", func(t *testing.T) {
		requirements := RequirementSpec{
			APIVersion: "troubleshoot.io/v1beta2",
			Kind:       "AnalyzerRequirements",
			Metadata: RequirementMetadata{
				Name:        "k8s-requirements",
				Description: "Kubernetes requirements",
			},
			Spec: RequirementSpecDetails{
				Kubernetes: KubernetesRequirements{
					MinVersion: "1.20.0",
					MaxVersion: "1.28.0",
				},
			},
		}

		// Convert to categorized requirements
		categorizer := NewRequirementCategorizer()
		categorized, err := categorizer.CategorizeSpec(&requirements)
		require.NoError(t, err, "Should categorize requirements")

		opts := GenerationOptions{
			PackageName: "test",
		}
		result, err := generator.GenerateAnalyzers(categorized, opts)
		require.NoError(t, err, "Should generate analyzers for Kubernetes requirements")
		require.NotNil(t, result, "Should return a result")

		// Should generate at least one analyzer
		assert.GreaterOrEqual(t, len(result.GeneratedAnalyzers), 1, "Should generate at least one analyzer")

		// Check that we have Kubernetes category analyzer
		hasK8sAnalyzer := false
		for _, analyzer := range result.GeneratedAnalyzers {
			if analyzer.Type == AnalyzerType(CategoryKubernetes) {
				hasK8sAnalyzer = true
				break
			}
		}
		assert.True(t, hasK8sAnalyzer, "Should generate Kubernetes category analyzer")
	})

	t.Run("Resource Requirements", func(t *testing.T) {
		requirements := RequirementSpec{
			APIVersion: "troubleshoot.io/v1beta2",
			Kind:       "AnalyzerRequirements",
			Metadata: RequirementMetadata{
				Name:        "resource-requirements",
				Description: "Resource requirements",
			},
			Spec: RequirementSpecDetails{
				Resources: ResourceRequirements{
					CPU: CPURequirement{
						MinCores:       2.0,
						MaxUtilization: 75.0,
					},
					Memory: MemoryRequirement{
						MinBytes:       4 * 1024 * 1024 * 1024, // 4GB
						MaxUtilization: 80.0,
					},
				},
			},
		}

		categorizer := NewRequirementCategorizer()
		categorized, err := categorizer.CategorizeSpec(&requirements)
		require.NoError(t, err, "Should categorize requirements")

		opts := GenerationOptions{PackageName: "test"}
		result, err := generator.GenerateAnalyzers(categorized, opts)
		require.NoError(t, err, "Should generate analyzers for resource requirements")
		require.NotNil(t, result, "Should return a result")

		// Should generate at least one analyzer
		assert.GreaterOrEqual(t, len(result.GeneratedAnalyzers), 1, "Should generate at least one analyzer")

		// Check that we have Resources category analyzer
		hasResourceAnalyzer := false
		for _, analyzer := range result.GeneratedAnalyzers {
			if analyzer.Type == AnalyzerType(CategoryResources) {
				hasResourceAnalyzer = true
				break
			}
		}
		assert.True(t, hasResourceAnalyzer, "Should generate resource category analyzer")
	})

	t.Run("Storage Requirements", func(t *testing.T) {
		requirements := RequirementSpec{
			APIVersion: "troubleshoot.io/v1beta2",
			Kind:       "AnalyzerRequirements",
			Metadata: RequirementMetadata{
				Name:        "storage-requirements",
				Description: "Storage requirements",
			},
			Spec: RequirementSpecDetails{
				Storage: StorageRequirements{
					MinCapacity: 100 * 1024 * 1024 * 1024, // 100GB
					StorageClasses: []StorageClassRequirement{
						{
							Name:        "fast-ssd",
							Provisioner: "kubernetes.io/gce-pd",
							Required:    true,
						},
					},
				},
			},
		}

		categorizer := NewRequirementCategorizer()
		categorized, err := categorizer.CategorizeSpec(&requirements)
		require.NoError(t, err, "Should categorize requirements")

		opts := GenerationOptions{PackageName: "test"}
		result, err := generator.GenerateAnalyzers(categorized, opts)
		require.NoError(t, err, "Should generate analyzers for storage requirements")
		require.NotNil(t, result, "Should return a result")

		// Should generate at least one analyzer
		assert.GreaterOrEqual(t, len(result.GeneratedAnalyzers), 1, "Should generate at least one analyzer")

		// Check that we have Storage category analyzer
		hasStorageAnalyzer := false
		for _, analyzer := range result.GeneratedAnalyzers {
			if analyzer.Type == AnalyzerType(CategoryStorage) {
				hasStorageAnalyzer = true
				break
			}
		}
		assert.True(t, hasStorageAnalyzer, "Should generate storage category analyzer")
	})
}

// TestRequirementCategorization tests requirement categorization
func TestRequirementCategorization(t *testing.T) {
	categorizer := NewRequirementCategorizer()

	testCases := []struct {
		name     string
		spec     RequirementSpec
		expected []RequirementCategory
	}{
		{
			name: "Kubernetes Requirements",
			spec: RequirementSpec{
				Spec: RequirementSpecDetails{
					Kubernetes: KubernetesRequirements{
						MinVersion: "1.20.0",
					},
				},
			},
			expected: []RequirementCategory{CategoryKubernetes},
		},
		{
			name: "Mixed Requirements",
			spec: RequirementSpec{
				Spec: RequirementSpecDetails{
					Kubernetes: KubernetesRequirements{
						MinVersion: "1.20.0",
					},
					Resources: ResourceRequirements{
						CPU: CPURequirement{
							MinCores: 1.0,
						},
					},
					Storage: StorageRequirements{
						MinCapacity: 50 * 1024 * 1024 * 1024, // 50GB
					},
				},
			},
			expected: []RequirementCategory{CategoryKubernetes, CategoryResources, CategoryStorage},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			categorized, err := categorizer.CategorizeSpec(&tc.spec)
			require.NoError(t, err, "Should categorize without error")
			require.NotNil(t, categorized, "Should return categorized requirements")

			// Check that all expected categories are present
			foundCategories := make(map[RequirementCategory]bool)
			for _, cat := range categorized {
				foundCategories[cat.Category] = true
			}

			for _, expectedCat := range tc.expected {
				assert.True(t, foundCategories[expectedCat],
					fmt.Sprintf("Should find category %s", expectedCat))
			}
		})
	}
}

// TestAnalyzerValidation tests analyzer validation
func TestAnalyzerValidation(t *testing.T) {
	validator := NewRequirementValidator()

	t.Run("Valid Requirement", func(t *testing.T) {
		spec := RequirementSpec{
			APIVersion: "troubleshoot.io/v1beta2",
			Kind:       "AnalyzerRequirements",
			Metadata: RequirementMetadata{
				Name:        "valid-requirements",
				Description: "Valid requirements for testing",
			},
			Spec: RequirementSpecDetails{
				Kubernetes: KubernetesRequirements{
					MinVersion: "1.20.0",
					MaxVersion: "1.28.0",
				},
			},
		}

		err := validator.Validate(&spec)
		assert.NoError(t, err, "Should be valid")
	})

	t.Run("Invalid Requirement", func(t *testing.T) {
		spec := RequirementSpec{
			APIVersion: "invalid-version",
			Kind:       "",
			// Missing required metadata
		}

		err := validator.Validate(&spec)
		assert.Error(t, err, "Should be invalid")
	})
}

// TestGenerationPerformance tests the performance of analyzer generation
func TestGenerationPerformance(t *testing.T) {
	generator := NewAnalyzerGenerator()

	// Create a complex requirement spec
	complexSpec := RequirementSpec{
		APIVersion: "troubleshoot.io/v1beta2",
		Kind:       "AnalyzerRequirements",
		Metadata: RequirementMetadata{
			Name:        "complex-requirements",
			Description: "Complex requirements for performance testing",
		},
		Spec: RequirementSpecDetails{
			Kubernetes: KubernetesRequirements{
				MinVersion: "1.20.0",
				MaxVersion: "1.28.0",
				Features:   []string{"metrics-server", "ingress"},
			},
			Resources: ResourceRequirements{
				CPU: CPURequirement{
					MinCores:       4.0,
					MaxUtilization: 80.0,
				},
				Memory: MemoryRequirement{
					MinBytes:       8 * 1024 * 1024 * 1024, // 8GB
					MaxUtilization: 85.0,
				},
			},
			Storage: StorageRequirements{
				MinCapacity: 200 * 1024 * 1024 * 1024, // 200GB
				StorageClasses: []StorageClassRequirement{
					{
						Name:        "fast-ssd",
						Provisioner: "kubernetes.io/gce-pd",
						Required:    true,
					},
					{
						Name:        "standard",
						Provisioner: "kubernetes.io/standard",
						Required:    false,
					},
				},
			},
			Security: SecurityRequirements{
				RBAC: RBACRequirement{
					Required:       true,
					ClusterRole:    true,
					ServiceAccount: true,
				},
				PodSecurity: PodSecurityRequirement{
					Standards:    []string{"restricted"},
					RunAsNonRoot: true,
					ReadOnlyRoot: true,
				},
			},
		},
	}

	categorizer := NewRequirementCategorizer()
	categorized, err := categorizer.CategorizeSpec(&complexSpec)
	require.NoError(t, err, "Should categorize requirements")

	opts := GenerationOptions{PackageName: "test"}
	start := time.Now()
	result, err := generator.GenerateAnalyzers(categorized, opts)
	duration := time.Since(start)

	require.NoError(t, err, "Should generate analyzers without error")
	require.NotNil(t, result, "Should return a result")
	assert.GreaterOrEqual(t, len(result.GeneratedAnalyzers), 1, "Should generate analyzers")

	t.Logf("Generated %d analyzers in %v", len(result.GeneratedAnalyzers), duration)

	// Performance assertion - should complete in reasonable time
	assert.Less(t, duration, 5*time.Second, "Generation should complete within 5 seconds")
}
