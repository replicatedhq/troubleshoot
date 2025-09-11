package analyzer

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/replicatedhq/troubleshoot/pkg/analyze/agents/local"
)

// BenchmarkAnalysisEngine benchmarks the core analysis engine performance
func BenchmarkAnalysisEngine(b *testing.B) {
	// Create test bundle
	bundleDir, err := ioutil.TempDir("", "benchmark-bundle-")
	require.NoError(b, err)
	defer os.RemoveAll(bundleDir)

	setupTestSupportBundle(&testing.T{}, bundleDir)

	bundle := &SupportBundle{
		RootDir: bundleDir,
		Files:   make(map[string][]byte),
	}
	err = loadBundleFiles(bundle)
	require.NoError(b, err)

	engine := NewAnalysisEngine()
	localAgent := local.NewLocalAgent()
	err = engine.RegisterAgent(localAgent)
	require.NoError(b, err)

	options := AnalysisOptions{
		AgentSelection: AgentSelectionOptions{
			AgentTypes: []string{"local"},
		},
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result, err := engine.Analyze(ctx, bundle, options)
		if err != nil {
			b.Fatal(err)
		}
		if result == nil {
			b.Fatal("Expected result but got nil")
		}
	}
}

// BenchmarkAnalysisWithDifferentBundleSizes benchmarks analysis with various bundle sizes
func BenchmarkAnalysisWithDifferentBundleSizes(b *testing.B) {
	testCases := []struct {
		name      string
		fileCount int
		fileSize  int
	}{
		{"Small_10files_1KB", 10, 1024},
		{"Medium_50files_10KB", 50, 10240},
		{"Large_100files_100KB", 100, 102400},
		{"XLarge_200files_500KB", 200, 512000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			bundleDir, err := ioutil.TempDir("", fmt.Sprintf("benchmark-%s-", tc.name))
			require.NoError(b, err)
			defer os.RemoveAll(bundleDir)

			createPerformanceTestBundle(&testing.T{}, bundleDir, tc.fileCount, tc.fileSize)

			bundle := &SupportBundle{
				RootDir: bundleDir,
				Files:   make(map[string][]byte),
			}
			err = loadBundleFiles(bundle)
			require.NoError(b, err)

			engine := NewAnalysisEngine()
			localAgent := local.NewLocalAgent()
			err = engine.RegisterAgent(localAgent)
			require.NoError(b, err)

			options := AnalysisOptions{
				AgentSelection: AgentSelectionOptions{
					AgentTypes: []string{"local"},
				},
			}

			ctx := context.Background()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				result, err := engine.Analyze(ctx, bundle, options)
				if err != nil {
					b.Fatal(err)
				}
				if result == nil {
					b.Fatal("Expected result but got nil")
				}
			}

			b.ReportMetric(float64(tc.fileCount*tc.fileSize), "bytes_processed")
		})
	}
}

// BenchmarkConcurrentAnalysis benchmarks concurrent analysis execution
func BenchmarkConcurrentAnalysis(b *testing.B) {
	concurrencyLevels := []int{1, 2, 4, 8, 16}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(b *testing.B) {
			// Create test bundles for each goroutine
			bundles := make([]*SupportBundle, concurrency)
			for i := 0; i < concurrency; i++ {
				bundleDir, err := ioutil.TempDir("", fmt.Sprintf("concurrent-benchmark-%d-", i))
				require.NoError(b, err)
				defer os.RemoveAll(bundleDir)

				setupTestSupportBundle(&testing.T{}, bundleDir)

				bundle := &SupportBundle{
					RootDir: bundleDir,
					Files:   make(map[string][]byte),
				}
				err = loadBundleFiles(bundle)
				require.NoError(b, err)
				bundles[i] = bundle
			}

			engine := NewAnalysisEngine()
			localAgent := local.NewLocalAgent()
			err := engine.RegisterAgent(localAgent)
			require.NoError(b, err)

			options := AnalysisOptions{
				AgentSelection: AgentSelectionOptions{
					AgentTypes: []string{"local"},
				},
				Performance: AnalysisPerformanceOptions{
					MaxConcurrentAgents: concurrency,
				},
			}

			ctx := context.Background()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				var wg sync.WaitGroup
				errors := make(chan error, concurrency)

				for j := 0; j < concurrency; j++ {
					wg.Add(1)
					go func(bundleIndex int) {
						defer wg.Done()
						result, err := engine.Analyze(ctx, bundles[bundleIndex], options)
						if err != nil {
							errors <- err
							return
						}
						if result == nil {
							errors <- fmt.Errorf("expected result but got nil")
						}
					}(j)
				}

				wg.Wait()
				close(errors)

				// Check for errors
				for err := range errors {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkAgentMethodCalls benchmarks individual agent method calls
func BenchmarkAgentMethodCalls(b *testing.B) {
	agent := local.NewLocalAgent()
	ctx := context.Background()

	b.Run("Name", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = agent.Name()
		}
	})

	b.Run("Version", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = agent.Version()
		}
	})

	b.Run("Capabilities", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = agent.Capabilities()
		}
	})

	b.Run("HealthCheck", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = agent.HealthCheck(ctx)
		}
	})
}

// TestAnalysisLatency tests analysis latency under various conditions
func TestAnalysisLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping latency test in short mode")
	}

	testCases := []struct {
		name               string
		fileCount          int
		fileSize           int
		expectedMaxLatency time.Duration
	}{
		{"Small Bundle", 10, 1024, 5 * time.Second},
		{"Medium Bundle", 50, 10240, 15 * time.Second},
		{"Large Bundle", 100, 102400, 30 * time.Second},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bundleDir, err := ioutil.TempDir("", fmt.Sprintf("latency-test-%s-", tc.name))
			require.NoError(t, err)
			defer os.RemoveAll(bundleDir)

			createPerformanceTestBundle(t, bundleDir, tc.fileCount, tc.fileSize)

			bundle := &SupportBundle{
				RootDir: bundleDir,
				Files:   make(map[string][]byte),
			}
			err = loadBundleFiles(bundle)
			require.NoError(t, err)

			engine := NewAnalysisEngine()
			localAgent := local.NewLocalAgent()
			err = engine.RegisterAgent(localAgent)
			require.NoError(t, err)

			options := AnalysisOptions{
				AgentSelection: AgentSelectionOptions{
					AgentTypes: []string{"local"},
				},
			}

			// Measure latency over multiple runs
			numRuns := 5
			latencies := make([]time.Duration, numRuns)

			for i := 0; i < numRuns; i++ {
				start := time.Now()
				ctx, cancel := context.WithTimeout(context.Background(), tc.expectedMaxLatency+5*time.Second)

				result, err := engine.Analyze(ctx, bundle, options)
				latency := time.Since(start)
				cancel()

				require.NoError(t, err)
				require.NotNil(t, result)

				latencies[i] = latency
			}

			// Calculate statistics
			avgLatency := calculateAverage(latencies)
			maxLatency := calculateMax(latencies)
			minLatency := calculateMin(latencies)

			t.Logf("%s Latency Stats: Avg=%v, Min=%v, Max=%v", tc.name, avgLatency, minLatency, maxLatency)

			// Verify latency is within acceptable bounds
			require.Less(t, maxLatency, tc.expectedMaxLatency,
				"Max latency %v exceeds expected %v for %s", maxLatency, tc.expectedMaxLatency, tc.name)
		})
	}
}

// TestMemoryUsageUnderLoad tests memory usage during heavy analysis workloads
func TestMemoryUsageUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory usage test in short mode")
	}

	// Create a large test bundle
	bundleDir, err := ioutil.TempDir("", "memory-load-test-")
	require.NoError(t, err)
	defer os.RemoveAll(bundleDir)

	createPerformanceTestBundle(t, bundleDir, 100, 102400) // 100 files, 100KB each

	bundle := &SupportBundle{
		RootDir: bundleDir,
		Files:   make(map[string][]byte),
	}
	err = loadBundleFiles(bundle)
	require.NoError(t, err)

	engine := NewAnalysisEngine()
	localAgent := local.NewLocalAgent()
	err = engine.RegisterAgent(localAgent)
	require.NoError(t, err)

	options := AnalysisOptions{
		AgentSelection: AgentSelectionOptions{
			AgentTypes: []string{"local"},
		},
	}

	// Measure memory usage before analysis
	runtime.GC()
	var memStatsBefore runtime.MemStats
	runtime.ReadMemStats(&memStatsBefore)

	// Run analysis multiple times to simulate load
	numRuns := 10
	ctx := context.Background()

	for i := 0; i < numRuns; i++ {
		result, err := engine.Analyze(ctx, bundle, options)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Force GC periodically to check for memory leaks
		if i%3 == 0 {
			runtime.GC()
		}
	}

	// Measure memory usage after analysis
	runtime.GC()
	var memStatsAfter runtime.MemStats
	runtime.ReadMemStats(&memStatsAfter)

	// Calculate memory growth
	memoryGrowth := memStatsAfter.HeapAlloc - memStatsBefore.HeapAlloc

	t.Logf("Memory Usage: Before=%d KB, After=%d KB, Growth=%d KB",
		memStatsBefore.HeapAlloc/1024, memStatsAfter.HeapAlloc/1024, memoryGrowth/1024)

	// Memory growth should be reasonable (less than 50MB for this test)
	maxAcceptableGrowth := uint64(50 * 1024 * 1024) // 50MB
	require.Less(t, memoryGrowth, maxAcceptableGrowth,
		"Memory growth %d bytes exceeds acceptable limit %d bytes", memoryGrowth, maxAcceptableGrowth)
}

// TestTimeoutHandling tests analysis timeout handling
func TestTimeoutHandling(t *testing.T) {
	bundleDir, err := ioutil.TempDir("", "timeout-test-")
	require.NoError(t, err)
	defer os.RemoveAll(bundleDir)

	// Create a large bundle that might take a while to process
	createPerformanceTestBundle(t, bundleDir, 200, 512000) // 200 files, 500KB each

	bundle := &SupportBundle{
		RootDir: bundleDir,
		Files:   make(map[string][]byte),
	}
	err = loadBundleFiles(bundle)
	require.NoError(t, err)

	engine := NewAnalysisEngine()
	localAgent := local.NewLocalAgent()
	err = engine.RegisterAgent(localAgent)
	require.NoError(t, err)

	options := AnalysisOptions{
		AgentSelection: AgentSelectionOptions{
			AgentTypes: []string{"local"},
		},
		Performance: AnalysisPerformanceOptions{
			TimeoutPerAgent: 100 * time.Millisecond, // Very short timeout
		},
	}

	// Analysis should timeout or complete within reasonable time
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	result, err := engine.Analyze(ctx, bundle, options)
	duration := time.Since(start)

	// Either analysis completes successfully or times out gracefully
	if err != nil {
		t.Logf("Analysis timed out as expected: %v (duration: %v)", err, duration)
		require.Contains(t, err.Error(), "timeout", "Timeout error should mention timeout")
	} else {
		t.Logf("Analysis completed successfully in %v", duration)
		require.NotNil(t, result, "Successful analysis should return result")
	}

	// Duration should be reasonable (not hang indefinitely)
	require.Less(t, duration, 10*time.Second, "Analysis should not hang indefinitely")
}

// TestResourceUtilization tests CPU and memory utilization during analysis
func TestResourceUtilization(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource utilization test in short mode")
	}

	bundleDir, err := ioutil.TempDir("", "resource-test-")
	require.NoError(t, err)
	defer os.RemoveAll(bundleDir)

	createPerformanceTestBundle(t, bundleDir, 100, 102400)

	bundle := &SupportBundle{
		RootDir: bundleDir,
		Files:   make(map[string][]byte),
	}
	err = loadBundleFiles(bundle)
	require.NoError(t, err)

	engine := NewAnalysisEngine()
	localAgent := local.NewLocalAgent()
	err = engine.RegisterAgent(localAgent)
	require.NoError(t, err)

	options := AnalysisOptions{
		AgentSelection: AgentSelectionOptions{
			AgentTypes: []string{"local"},
		},
	}

	// Monitor resource usage during analysis
	ctx := context.Background()

	// Start monitoring in a goroutine
	monitoring := make(chan bool)
	var maxMemory uint64
	var memoryReadings []uint64

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-monitoring:
				return
			case <-ticker.C:
				var memStats runtime.MemStats
				runtime.ReadMemStats(&memStats)

				memoryReadings = append(memoryReadings, memStats.HeapAlloc)
				if memStats.HeapAlloc > maxMemory {
					maxMemory = memStats.HeapAlloc
				}
			}
		}
	}()

	start := time.Now()
	result, err := engine.Analyze(ctx, bundle, options)
	duration := time.Since(start)

	close(monitoring)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Calculate memory statistics
	var avgMemory uint64
	for _, reading := range memoryReadings {
		avgMemory += reading
	}
	if len(memoryReadings) > 0 {
		avgMemory /= uint64(len(memoryReadings))
	}

	t.Logf("Resource Usage - Duration: %v, Max Memory: %d KB, Avg Memory: %d KB, Readings: %d",
		duration, maxMemory/1024, avgMemory/1024, len(memoryReadings))

	// Basic assertions about resource usage
	require.Greater(t, len(memoryReadings), 0, "Should have captured memory readings")
	require.Less(t, duration, 1*time.Minute, "Analysis should complete within reasonable time")
}

// Helper functions for performance tests

func calculateAverage(durations []time.Duration) time.Duration {
	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	return sum / time.Duration(len(durations))
}

func calculateMax(durations []time.Duration) time.Duration {
	var max time.Duration
	for _, d := range durations {
		if d > max {
			max = d
		}
	}
	return max
}

func calculateMin(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	min := durations[0]
	for _, d := range durations {
		if d < min {
			min = d
		}
	}
	return min
}

// BenchmarkMemoryAllocations benchmarks memory allocations during analysis
func BenchmarkMemoryAllocations(b *testing.B) {
	bundleDir, err := ioutil.TempDir("", "memory-benchmark-")
	require.NoError(b, err)
	defer os.RemoveAll(bundleDir)

	setupTestSupportBundle(&testing.T{}, bundleDir)

	bundle := &SupportBundle{
		RootDir: bundleDir,
		Files:   make(map[string][]byte),
	}
	err = loadBundleFiles(bundle)
	require.NoError(b, err)

	engine := NewAnalysisEngine()
	localAgent := local.NewLocalAgent()
	err = engine.RegisterAgent(localAgent)
	require.NoError(b, err)

	options := AnalysisOptions{
		AgentSelection: AgentSelectionOptions{
			AgentTypes: []string{"local"},
		},
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result, err := engine.Analyze(ctx, bundle, options)
		if err != nil {
			b.Fatal(err)
		}
		if result == nil {
			b.Fatal("Expected result but got nil")
		}
	}
}

// TestScalability tests analysis scalability with increasing loads
func TestScalability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping scalability test in short mode")
	}

	scaleLevels := []struct {
		name      string
		fileCount int
		fileSize  int
	}{
		{"Scale_1x", 25, 25600},
		{"Scale_2x", 50, 25600},
		{"Scale_4x", 100, 25600},
		{"Scale_8x", 200, 25600},
	}

	results := make(map[string]time.Duration)

	for _, scale := range scaleLevels {
		t.Run(scale.name, func(t *testing.T) {
			bundleDir, err := ioutil.TempDir("", fmt.Sprintf("scale-test-%s-", scale.name))
			require.NoError(t, err)
			defer os.RemoveAll(bundleDir)

			createPerformanceTestBundle(t, bundleDir, scale.fileCount, scale.fileSize)

			bundle := &SupportBundle{
				RootDir: bundleDir,
				Files:   make(map[string][]byte),
			}
			err = loadBundleFiles(bundle)
			require.NoError(t, err)

			engine := NewAnalysisEngine()
			localAgent := local.NewLocalAgent()
			err = engine.RegisterAgent(localAgent)
			require.NoError(t, err)

			options := AnalysisOptions{
				AgentSelection: AgentSelectionOptions{
					AgentTypes: []string{"local"},
				},
			}

			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)

			result, err := engine.Analyze(ctx, bundle, options)
			duration := time.Since(start)
			cancel()

			require.NoError(t, err)
			require.NotNil(t, result)

			results[scale.name] = duration
			t.Logf("%s: Processed %d files in %v", scale.name, scale.fileCount, duration)
		})
	}

	// Verify that performance scales reasonably
	baseline := results["Scale_1x"]
	for name, duration := range results {
		if name == "Scale_1x" {
			continue
		}

		// Performance shouldn't degrade exponentially
		// Allow some degradation but not more than 10x for 8x data
		maxAcceptable := baseline * 10
		require.Less(t, duration, maxAcceptable,
			"Performance for %s (%v) degrades too much from baseline (%v)", name, duration, baseline)
	}
}
