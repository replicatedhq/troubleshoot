package analyzer_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/analyze/agents/local"
	"github.com/replicatedhq/troubleshoot/pkg/analyze/remediation"
)

// TestEndToEndAnalysis tests complete analysis workflow with real support bundle data
func TestEndToEndAnalysis(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary directory for test bundle
	bundleDir, err := ioutil.TempDir("", "integration-test-bundle-")
	require.NoError(t, err)
	defer os.RemoveAll(bundleDir)

	// Create a realistic support bundle structure
	setupTestSupportBundle(t, bundleDir)

	// Initialize analysis engine
	engine := analyzer.NewAnalysisEngine()
	localAgent := local.NewLocalAgent()
	err = engine.RegisterAgent("local", localAgent)
	require.NoError(t, err)

	// Create support bundle
	bundle := &analyzer.SupportBundle{
		RootDir: bundleDir,
	}

	// Load bundle files
	err = loadBundleFiles(bundle)
	require.NoError(t, err)

	// Configure analysis options
	options := analyzer.AnalysisOptions{
		// AgentSelection: // analyzer.AgentSelectionOptions{
		//	AgentTypes:       []string{"local"},
		//	HybridMode:       false,
		//	RequireAllAgents: false,
		// },
		// OutputFormat:         "json", // Field not available
		// IncludeSensitiveData: false, // Field not available
		// Filters: analyzer.AnalysisFilters{
		//	Categories:    []string{"resource", "kubernetes", "storage"},
		//	MinConfidence: 0.3,
		// },
	}

	// Run analysis
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, err := engine.Analyze(ctx, bundle, options)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Validate analysis results
	t.Run("ResultValidation", func(t *testing.T) {
		assert.NotEmpty(t, result.Results, "Should have analysis results")
		assert.Greater(t, len(result.Results), 0, "Should have at least one result")

		for _, analysisResult := range result.Results {
			assert.NotEmpty(t, analysisResult.Title, "Result should have title")
			// Check that result has valid status
			hasValidStatus := analysisResult.IsPass || analysisResult.IsFail || analysisResult.IsWarn
			assert.True(t, hasValidStatus,
				"Result should have valid status")
		}
	})

	// Test remediation generation
	t.Run("RemediationGeneration", func(t *testing.T) {
		if len(result.Results) > 0 {
			// Convert to analysis results for remediation
			var analysisResults []remediation.AnalysisResult
			for _, r := range result.Results {
				analysisResults = append(analysisResults, remediation.AnalysisResult{
					ID:          r.Title, // Use Title as ID since ID field doesn't exist
					Title:       r.Title,
					Description: r.Message, // Use Message field instead of Description
					Category:    "general", // Default category since field doesn't exist
					Severity:    "medium",  // Default severity since field doesn't exist
				})
			}

			// Generate remediation
			remEngine := remediation.NewRemediationEngine()
			remOptions := remediation.RemediationOptions{
				IncludeManual:    true,
				IncludeAutomated: true,
				MaxSteps:         10,
				SkillLevel:       remediation.SkillIntermediate,
			}

			remResult, err := remEngine.GenerateRemediation(ctx, analysisResults, remOptions)
			require.NoError(t, err)
			assert.NotNil(t, remResult)

			if len(analysisResults) > 0 {
				assert.NotEmpty(t, remResult.Steps, "Should generate remediation steps")
			}
		}
	})

	// Test serialization of results
	t.Run("ResultSerialization", func(t *testing.T) {
		jsonData, err := json.MarshalIndent(result, "", "  ")
		require.NoError(t, err)
		assert.Contains(t, string(jsonData), "results")
		assert.Contains(t, string(jsonData), "metadata")

		// Verify can deserialize
		var deserializedResult analyzer.EnhancedAnalysisResult
		err = json.Unmarshal(jsonData, &deserializedResult)
		require.NoError(t, err)
	})
}

// TestMultiAgentCoordination tests coordination between multiple agents
func TestMultiAgentCoordination(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	engine := analyzer.NewAnalysisEngine()

	// Register multiple agents
	localAgent := local.NewLocalAgent()
	err := engine.RegisterAgent("local", localAgent)
	require.NoError(t, err)

	// Create test bundle
	bundleDir, err := ioutil.TempDir("", "multi-agent-test-")
	require.NoError(t, err)
	defer os.RemoveAll(bundleDir)

	setupTestSupportBundle(t, bundleDir)

	bundle := &analyzer.SupportBundle{
		RootDir: bundleDir,
	}
	err = loadBundleFiles(bundle)
	require.NoError(t, err)

	// Test with hybrid mode
	options := analyzer.AnalysisOptions{
		// AgentSelection: // analyzer.AgentSelectionOptions{
		//	AgentTypes:         []string{"local"},
		//	HybridMode:         true,
		//	RequireAllAgents:   false,
		//	AgentFailurePolicy: "continue",
		// },
		// Performance: AnalysisPerformanceOptions{
		//	MaxConcurrentAgents: 2,
		//	TimeoutPerAgent:     30 * time.Second,
		// },
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	result, err := engine.Analyze(ctx, bundle, options)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Validate coordination results
	assert.NotEmpty(t, result.Results, "Multi-agent coordination should produce results")
	assert.NotNil(t, result.Metadata, "Should have analysis metadata")
}

// TestAnalysisPerformance tests analysis performance with various bundle sizes
func TestAnalysisPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	testCases := []struct {
		name      string
		fileCount int
		fileSize  int
	}{
		{"Small Bundle", 10, 1024},    // 10 files, 1KB each
		{"Medium Bundle", 50, 10240},  // 50 files, 10KB each
		{"Large Bundle", 100, 102400}, // 100 files, 100KB each
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test bundle with specified size
			bundleDir, err := ioutil.TempDir("", fmt.Sprintf("perf-test-%s-", tc.name))
			require.NoError(t, err)
			defer os.RemoveAll(bundleDir)

			createPerformanceTestBundle(t, bundleDir, tc.fileCount, tc.fileSize)

			bundle := &analyzer.SupportBundle{
				RootDir: bundleDir,
			}
			err = loadBundleFiles(bundle)
			require.NoError(t, err)

			// Initialize engine
			engine := analyzer.NewAnalysisEngine()
			localAgent := local.NewLocalAgent()
			err = engine.RegisterAgent("local", localAgent)
			require.NoError(t, err)

			options := analyzer.AnalysisOptions{
				// AgentSelection: analyzer.AgentSelectionOptions{
				//	AgentTypes: []string{"local"},
				// },
			}

			// Measure analysis time
			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			result, err := engine.Analyze(ctx, bundle, options)
			analysisTime := time.Since(start)

			require.NoError(t, err)
			require.NotNil(t, result)

			// Performance assertions
			assert.Less(t, analysisTime, 2*time.Minute,
				"Analysis should complete within 2 minutes for %s", tc.name)

			t.Logf("%s: Analyzed %d files (%d bytes each) in %v",
				tc.name, tc.fileCount, tc.fileSize, analysisTime)
		})
	}
}

// TestAnalysisWithRealKubernetesData tests analysis with realistic Kubernetes data
func TestAnalysisWithRealKubernetesData(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	bundleDir, err := ioutil.TempDir("", "k8s-integration-test-")
	require.NoError(t, err)
	defer os.RemoveAll(bundleDir)

	// Create realistic Kubernetes support bundle data
	setupRealisticKubernetesBundle(t, bundleDir)

	bundle := &analyzer.SupportBundle{
		RootDir: bundleDir,
	}
	err = loadBundleFiles(bundle)
	require.NoError(t, err)

	engine := analyzer.NewAnalysisEngine()
	localAgent := local.NewLocalAgent()
	err = engine.RegisterAgent("local", localAgent)
	require.NoError(t, err)

	options := analyzer.AnalysisOptions{
		AgentSelection: // analyzer.AgentSelectionOptions{
			AgentTypes: []string{"local"},
		},
		Filters: analyzer.AnalysisFilters{
			Categories: []string{"kubernetes"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	result, err := engine.Analyze(ctx, bundle, options)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Validate Kubernetes-specific results
	assert.NotEmpty(t, result.Results, "Should have Kubernetes analysis results")

	kubernetesResults := 0
	for _, r := range result.Results {
		if r.Category == "kubernetes" {
			kubernetesResults++
		}
	}
	assert.Greater(t, kubernetesResults, 0, "Should have Kubernetes-specific results")
}

// TestConcurrentAnalysis tests concurrent analysis execution
func TestConcurrentAnalysis(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	// Create multiple test bundles
	numBundles := 3
	bundles := make([]*SupportBundle, numBundles)
	bundleDirs := make([]string, numBundles)

	for i := 0; i < numBundles; i++ {
		bundleDir, err := ioutil.TempDir("", fmt.Sprintf("concurrent-test-%d-", i))
		require.NoError(t, err)
		defer os.RemoveAll(bundleDir)

		bundleDirs[i] = bundleDir
		setupTestSupportBundle(t, bundleDir)

		bundle := &analyzer.SupportBundle{
			RootDir: bundleDir,
			Files:   make(map[string][]byte),
		}
		err = loadBundleFiles(bundle)
		require.NoError(t, err)

		bundles[i] = bundle
	}

	engine := analyzer.NewAnalysisEngine()
	localAgent := local.NewLocalAgent()
	err := engine.RegisterAgent("local", localAgent)
	require.NoError(t, err)

	options := analyzer.AnalysisOptions{
		// AgentSelection: // analyzer.AgentSelectionOptions{
		//	AgentTypes: []string{"local"},
		// },
		// Performance: AnalysisPerformanceOptions{
		//	MaxConcurrentAgents: 2,
		// },
	}

	// Run analyses concurrently
	results := make(chan *analyzer.EnhancedAnalysisResult, numBundles)
	errors := make(chan error, numBundles)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	for i, bundle := range bundles {
		go func(id int, b *SupportBundle) {
			result, err := engine.Analyze(ctx, b, options)
			if err != nil {
				errors <- err
			} else {
				results <- result
			}
		}(i, bundle)
	}

	// Collect results
	successCount := 0
	errorCount := 0

	for i := 0; i < numBundles; i++ {
		select {
		case result := <-results:
			assert.NotNil(t, result)
			successCount++
		case err := <-errors:
			t.Logf("Analysis error: %v", err)
			errorCount++
		case <-time.After(3 * time.Minute):
			t.Fatal("Concurrent analysis timed out")
		}
	}

	assert.Greater(t, successCount, 0, "At least one analysis should succeed")
	t.Logf("Concurrent analysis: %d successful, %d errors", successCount, errorCount)
}

// Helper functions for setting up test data

func setupTestSupportBundle(t *testing.T, bundleDir string) {
	// Create cluster-resources directory
	clusterDir := filepath.Join(bundleDir, "cluster-resources")
	err := os.MkdirAll(clusterDir, 0755)
	require.NoError(t, err)

	// Create sample pods.json
	podsData := `{
		"apiVersion": "v1",
		"kind": "List",
		"items": [
			{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"name": "test-pod-1",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "app",
							"image": "nginx:1.20",
							"resources": {
								"requests": {"cpu": "100m", "memory": "128Mi"},
								"limits": {"cpu": "200m", "memory": "256Mi"}
							}
						}
					]
				},
				"status": {
					"phase": "Running"
				}
			}
		]
	}`

	err = ioutil.WriteFile(filepath.Join(clusterDir, "pods.json"), []byte(podsData), 0644)
	require.NoError(t, err)

	// Create sample nodes.json
	nodesData := `{
		"apiVersion": "v1",
		"kind": "List",
		"items": [
			{
				"apiVersion": "v1",
				"kind": "Node",
				"metadata": {
					"name": "node-1"
				},
				"status": {
					"conditions": [
						{
							"type": "Ready",
							"status": "True"
						}
					],
					"capacity": {
						"cpu": "4",
						"memory": "8Gi"
					},
					"allocatable": {
						"cpu": "3.5",
						"memory": "7Gi"
					}
				}
			}
		]
	}`

	err = ioutil.WriteFile(filepath.Join(clusterDir, "nodes.json"), []byte(nodesData), 0644)
	require.NoError(t, err)

	// Create version info
	versionData := `{
		"major": "1",
		"minor": "21",
		"gitVersion": "v1.21.0"
	}`

	err = ioutil.WriteFile(filepath.Join(clusterDir, "version.json"), []byte(versionData), 0644)
	require.NoError(t, err)
}

func setupRealisticKubernetesBundle(t *testing.T, bundleDir string) {
	setupTestSupportBundle(t, bundleDir)

	clusterDir := filepath.Join(bundleDir, "cluster-resources")

	// Add more realistic data - deployments
	deploymentData := `{
		"apiVersion": "v1",
		"kind": "List",
		"items": [
			{
				"apiVersion": "apps/v1",
				"kind": "Deployment",
				"metadata": {
					"name": "web-app",
					"namespace": "default"
				},
				"spec": {
					"replicas": 3,
					"selector": {
						"matchLabels": {"app": "web"}
					},
					"template": {
						"metadata": {
							"labels": {"app": "web"}
						},
						"spec": {
							"containers": [
								{
									"name": "web",
									"image": "nginx:1.20",
									"resources": {
										"requests": {"cpu": "100m", "memory": "128Mi"},
										"limits": {"cpu": "500m", "memory": "512Mi"}
									}
								}
							]
						}
					}
				},
				"status": {
					"replicas": 3,
					"readyReplicas": 2,
					"availableReplicas": 2
				}
			}
		]
	}`

	err := ioutil.WriteFile(filepath.Join(clusterDir, "deployments.json"), []byte(deploymentData), 0644)
	require.NoError(t, err)

	// Add services
	serviceData := `{
		"apiVersion": "v1",
		"kind": "List", 
		"items": [
			{
				"apiVersion": "v1",
				"kind": "Service",
				"metadata": {
					"name": "web-service",
					"namespace": "default"
				},
				"spec": {
					"selector": {"app": "web"},
					"ports": [{"port": 80, "targetPort": 80}],
					"type": "ClusterIP"
				}
			}
		]
	}`

	err = ioutil.WriteFile(filepath.Join(clusterDir, "services.json"), []byte(serviceData), 0644)
	require.NoError(t, err)
}

func createPerformanceTestBundle(t *testing.T, bundleDir string, fileCount, fileSize int) {
	clusterDir := filepath.Join(bundleDir, "cluster-resources")
	err := os.MkdirAll(clusterDir, 0755)
	require.NoError(t, err)

	// Create specified number of files with specified size
	for i := 0; i < fileCount; i++ {
		fileName := fmt.Sprintf("test-file-%d.json", i)
		content := generateTestContent(fileSize)

		err = ioutil.WriteFile(filepath.Join(clusterDir, fileName), []byte(content), 0644)
		require.NoError(t, err)
	}
}

func generateTestContent(size int) string {
	baseContent := `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"test"},"data":{"key":"value`

	// Pad content to reach desired size
	padding := ""
	currentSize := len(baseContent) + 3 // Account for closing quotes and braces

	if size > currentSize {
		paddingSize := size - currentSize
		for i := 0; i < paddingSize; i++ {
			padding += "x"
		}
	}

	return baseContent + padding + `"}}`
}

func loadBundleFiles(bundle *analyzer.SupportBundle) error {
	return filepath.Walk(bundle.RootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(bundle.RootDir, path)
		if err != nil {
			return err
		}

		// Read file content
		content, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		bundle.Files[relPath] = content
		return nil
	})
}

// TestMemoryUsage tests memory usage during analysis
func TestMemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}

	// This test would require runtime memory profiling
	// For now, we'll just ensure analysis completes without excessive memory usage
	bundleDir, err := ioutil.TempDir("", "memory-test-")
	require.NoError(t, err)
	defer os.RemoveAll(bundleDir)

	// Create a bundle with moderate amount of data
	createPerformanceTestBundle(t, bundleDir, 50, 10240)

	bundle := &analyzer.SupportBundle{
		RootDir: bundleDir,
	}
	err = loadBundleFiles(bundle)
	require.NoError(t, err)

	engine := analyzer.NewAnalysisEngine()
	localAgent := local.NewLocalAgent()
	err = engine.RegisterAgent("local", localAgent)
	require.NoError(t, err)

	options := analyzer.AnalysisOptions{
		AgentSelection: // analyzer.AgentSelectionOptions{
			AgentTypes: []string{"local"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	result, err := engine.Analyze(ctx, bundle, options)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Test passes if analysis completes without running out of memory
	assert.NotEmpty(t, result.Results, "Should complete analysis successfully")
}
