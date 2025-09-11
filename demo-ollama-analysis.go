package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/analyze/agents/llm"
	"github.com/replicatedhq/troubleshoot/pkg/analyze/agents/local"
)

func main() {
	fmt.Println("🚀 Troubleshoot Intelligent Analysis Demo with Ollama")
	fmt.Println(strings.Repeat("=", 60))

	// Demo scenarios
	scenarios := []DemoScenario{
		{
			Name:        "CPU Pressure & Resource Exhaustion",
			BundlePath:  "sample-support-bundles/cpu-pressure",
			Description: "High CPU usage causing OOMKilled pods and scheduling failures",
			ExpectedIssues: []string{
				"Resource exhaustion",
				"Pod scheduling issues",
				"Memory pressure leading to OOMKill",
			},
		},
		{
			Name:        "Application CrashLoop BackOff",
			BundlePath:  "sample-support-bundles/crashloop-issue",
			Description: "Backend API failing to start due to dependency issues",
			ExpectedIssues: []string{
				"Service discovery problems",
				"Database connectivity issues",
				"Missing or misconfigured dependencies",
			},
		},
	}

	ctx := context.Background()

	// Initialize agents
	localAgent := local.NewLocalAgent()
	ollamaAgent, err := createOllamaAgent()
	if err != nil {
		log.Printf("⚠️  Could not initialize Ollama agent: %v", err)
		log.Println("💡 Make sure Ollama is running with: ollama serve")
		log.Println("💡 And pull a model with: ollama pull codellama:13b")
		return
	}

	// Test Ollama health
	if err := ollamaAgent.HealthCheck(ctx); err != nil {
		log.Printf("❌ Ollama health check failed: %v", err)
		log.Println("💡 Start Ollama and ensure codellama:13b model is available")
		return
	}

	fmt.Println("✅ Ollama agent ready!")
	fmt.Println()

	// Run analysis on each scenario
	for i, scenario := range scenarios {
		fmt.Printf("📊 Scenario %d: %s\n", i+1, scenario.Name)
		fmt.Printf("📁 Bundle: %s\n", scenario.BundlePath)
		fmt.Printf("📖 Description: %s\n", scenario.Description)
		fmt.Println()

		// Load support bundle
		bundle, err := loadSupportBundle(scenario.BundlePath)
		if err != nil {
			log.Printf("❌ Failed to load bundle: %v", err)
			continue
		}

		// Define analyzers to run
		analyzers := []analyzer.AnalyzerSpec{
			{Name: "cluster-resources", Type: "clusterPodStatuses"},
			{Name: "node-resources", Type: "nodeResources"},
			{Name: "pod-status", Type: "containerStatuses"},
		}

		// Run basic local analysis
		fmt.Println("🔧 Running LOCAL ANALYSIS...")
		localResult, err := localAgent.Analyze(ctx, bundle, analyzers)
		if err != nil {
			log.Printf("❌ Local analysis failed: %v", err)
			continue
		}

		printBasicAnalysis(localResult)

		// Run enhanced Ollama analysis
		fmt.Println("🧠 Running INTELLIGENT ANALYSIS with Ollama...")
		startTime := time.Now()
		ollamaResult, err := ollamaAgent.Analyze(ctx, bundle, analyzers)
		analysisTime := time.Since(startTime)

		if err != nil {
			log.Printf("❌ Ollama analysis failed: %v", err)
			continue
		}

		printEnhancedAnalysis(ollamaResult, analysisTime)

		// Compare insights
		fmt.Println("🔍 COMPARISON & INSIGHTS:")
		compareResults(localResult, ollamaResult, scenario.ExpectedIssues)

		fmt.Println("\n" + strings.Repeat("=", 80) + "\n")
	}

	fmt.Println("✨ Demo completed! The intelligent analysis provides:")
	fmt.Println("   • 🎯 Root cause identification")
	fmt.Println("   • 🛠️  Detailed remediation steps")
	fmt.Println("   • 🔗 Correlation between related issues")
	fmt.Println("   • 📚 Contextual explanations")
	fmt.Println("   • 🎯 Confidence scoring")
}

type DemoScenario struct {
	Name           string
	BundlePath     string
	Description    string
	ExpectedIssues []string
}

func createOllamaAgent() (*llm.OllamaAgent, error) {
	config := llm.DefaultOllamaConfig("codellama:13b")
	config.SystemPrompt = `You are a Kubernetes troubleshooting expert. Analyze the provided support bundle data and provide:
1. Clear root cause analysis
2. Specific remediation steps
3. Confidence score (0-1)  
4. Impact assessment
5. Evidence from the data
6. Correlation with related issues

Be practical and focus on actionable insights. Format your response as structured JSON.`

	config.Temperature = 0.2 // Lower temperature for more focused analysis
	config.MaxTokens = 2048  // Enough for detailed analysis

	return llm.NewOllamaAgent(config)
}

func loadSupportBundle(bundlePath string) (*analyzer.SupportBundle, error) {
	absPath, err := filepath.Abs(bundlePath)
	if err != nil {
		return nil, err
	}

	// Check if bundle directory exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("support bundle not found: %s", absPath)
	}

	bundle := &analyzer.SupportBundle{
		Path:    absPath,
		RootDir: absPath,
		Metadata: map[string]interface{}{
			"bundlePath":  absPath,
			"collectedAt": time.Now(),
		},
		GetFile: func(filename string) ([]byte, error) {
			fullPath := filepath.Join(absPath, filename)
			return os.ReadFile(fullPath)
		},
		FindFiles: func(pattern string, dirs []string) (map[string][]byte, error) {
			files := make(map[string][]byte)

			for _, dir := range dirs {
				dirPath := filepath.Join(absPath, dir)
				matches, err := filepath.Glob(filepath.Join(dirPath, pattern))
				if err != nil {
					continue
				}

				for _, match := range matches {
					relPath, _ := filepath.Rel(absPath, match)
					content, err := os.ReadFile(match)
					if err == nil {
						files[relPath] = content
					}
				}
			}
			return files, nil
		},
	}

	return bundle, nil
}

func printBasicAnalysis(result *analyzer.AgentResult) {
	fmt.Printf("   Agent: %s\n", result.AgentName)
	fmt.Printf("   Processing Time: %v\n", result.ProcessingTime)
	fmt.Printf("   Results Found: %d\n", len(result.Results))

	for i, res := range result.Results {
		status := "✅ PASS"
		if res.IsFail {
			status = "❌ FAIL"
		} else if res.IsWarn {
			status = "⚠️  WARN"
		}

		fmt.Printf("     %d. %s - %s\n", i+1, status, res.Title)
		if res.Message != "" {
			fmt.Printf("        Message: %s\n", res.Message)
		}
	}
	fmt.Println()
}

func printEnhancedAnalysis(result *analyzer.AgentResult, analysisTime time.Duration) {
	fmt.Printf("   🧠 Agent: %s (LLM-Enhanced)\n", result.AgentName)
	fmt.Printf("   ⏱️  Processing Time: %v\n", analysisTime)
	fmt.Printf("   📊 Enhanced Results: %d\n", len(result.Results))
	fmt.Printf("   💡 Insights Generated: %d\n", len(result.Insights))

	// Show enhanced results with confidence and explanations
	for i, res := range result.Results {
		status := "✅ PASS"
		if res.IsFail {
			status = "❌ FAIL"
		} else if res.IsWarn {
			status = "⚠️  WARN"
		}

		fmt.Printf("     %d. %s - %s\n", i+1, status, res.Title)
		if res.Explanation != "" {
			fmt.Printf("        🔍 Analysis: %s\n", res.Explanation)
		}
		if res.Confidence > 0 {
			fmt.Printf("        🎯 Confidence: %.1f%%\n", res.Confidence*100)
		}
		if res.Impact != "" {
			fmt.Printf("        📈 Impact: %s\n", res.Impact)
		}
		if res.RootCause != "" {
			fmt.Printf("        🎯 Root Cause: %s\n", res.RootCause)
		}
		if res.Remediation != nil {
			fmt.Printf("        🛠️  Remediation: %s\n", res.Remediation.Title)
			if len(res.Remediation.Commands) > 0 {
				fmt.Printf("        💻 Commands: %v\n", res.Remediation.Commands)
			}
		}
		fmt.Println()
	}

	// Show intelligent insights
	if len(result.Insights) > 0 {
		fmt.Println("   🔗 INTELLIGENT INSIGHTS:")
		for i, insight := range result.Insights {
			fmt.Printf("     %d. %s (%s)\n", i+1, insight.Title, insight.Type)
			fmt.Printf("        📝 %s\n", insight.Description)
			if insight.Confidence > 0 {
				fmt.Printf("        🎯 Confidence: %.1f%%\n", insight.Confidence*100)
			}
			if len(insight.Evidence) > 0 {
				fmt.Printf("        📋 Evidence: %v\n", insight.Evidence)
			}
		}
	}
}

func compareResults(localResult, ollamaResult *analyzer.AgentResult, expectedIssues []string) {
	fmt.Printf("   📊 Local Results: %d issues found\n", countIssues(localResult))
	fmt.Printf("   🧠 Enhanced Results: %d issues found + %d insights\n",
		countIssues(ollamaResult), len(ollamaResult.Insights))

	fmt.Println("   🎯 Expected Issues Coverage:")
	for _, expected := range expectedIssues {
		covered := checkIssueCoverage(ollamaResult, expected)
		status := "❌"
		if covered {
			status = "✅"
		}
		fmt.Printf("     %s %s\n", status, expected)
	}

	// Show the intelligence advantage
	hasRootCause := hasRootCauseAnalysis(ollamaResult)
	hasRemediation := hasRemediationSteps(ollamaResult)
	hasCorrelation := len(ollamaResult.Insights) > 0

	fmt.Println("   🧠 Intelligence Advantages:")
	fmt.Printf("     Root Cause Analysis: %s\n", boolToStatus(hasRootCause))
	fmt.Printf("     Remediation Steps: %s\n", boolToStatus(hasRemediation))
	fmt.Printf("     Issue Correlation: %s\n", boolToStatus(hasCorrelation))
}

func countIssues(result *analyzer.AgentResult) int {
	count := 0
	for _, res := range result.Results {
		if res.IsFail || res.IsWarn {
			count++
		}
	}
	return count
}

func checkIssueCoverage(result *analyzer.AgentResult, expectedIssue string) bool {
	// Check if any result or insight covers the expected issue
	for _, res := range result.Results {
		if containsIgnoreCase(res.Explanation, expectedIssue) ||
			containsIgnoreCase(res.RootCause, expectedIssue) ||
			containsIgnoreCase(res.Title, expectedIssue) {
			return true
		}
	}

	for _, insight := range result.Insights {
		if containsIgnoreCase(insight.Description, expectedIssue) ||
			containsIgnoreCase(insight.Title, expectedIssue) {
			return true
		}
	}

	return false
}

func hasRootCauseAnalysis(result *analyzer.AgentResult) bool {
	for _, res := range result.Results {
		if res.RootCause != "" {
			return true
		}
	}
	return false
}

func hasRemediationSteps(result *analyzer.AgentResult) bool {
	for _, res := range result.Results {
		if res.Remediation != nil {
			return true
		}
	}
	return false
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func boolToStatus(b bool) string {
	if b {
		return "✅ Yes"
	}
	return "❌ No"
}
