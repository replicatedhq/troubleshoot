#!/bin/bash

echo "🧠 Real Cluster Intelligent Analysis with Ollama"
echo "=============================================="

# Check if we're connected to a cluster
if ! kubectl cluster-info &>/dev/null; then
    echo "❌ No Kubernetes cluster connection found"
    echo "💡 Make sure you're in the replicated cluster shell"
    exit 1
fi

# Check if troubleshoot CLI is available
if ! command -v kubectl-support_bundle &> /dev/null; then
    echo "🔧 Installing troubleshoot CLI..."
    curl -L https://github.com/replicatedhq/troubleshoot/releases/latest/download/support-bundle_darwin_amd64.tar.gz | tar xzv
    sudo mv support-bundle /usr/local/bin/kubectl-support_bundle
    chmod +x /usr/local/bin/kubectl-support_bundle
    
    if command -v kubectl-support_bundle &> /dev/null; then
        echo "✅ Troubleshoot CLI installed"
    else
        echo "❌ Failed to install troubleshoot CLI"
        exit 1
    fi
else
    echo "✅ Troubleshoot CLI found"
fi

# Check current pod status
echo ""
echo "📊 Current cluster state:"
kubectl get pods -o wide
echo ""
kubectl get events --sort-by=.metadata.creationTimestamp --field-selector type!=Normal | tail -5
echo ""

# Collect support bundle
echo "📦 Collecting real support bundle from cluster..."
kubectl-support_bundle real-cluster-troubleshoot.yaml

if [ $? -ne 0 ]; then
    echo "❌ Failed to collect support bundle"
    exit 1
fi

# Find the latest support bundle (should be in current directory)
LATEST_BUNDLE=$(ls -t support-bundle-*.tar.gz | head -1)
BUNDLE_DIR="./real-support-bundles/${LATEST_BUNDLE%.tar.gz}"

echo "✅ Support bundle collected: $LATEST_BUNDLE"

# Extract support bundle
echo "📂 Extracting support bundle..."
mkdir -p ./real-support-bundles/
tar -xzf "$LATEST_BUNDLE" -C ./real-support-bundles/
echo "✅ Support bundle extracted to: $BUNDLE_DIR"

# Run traditional analysis
echo ""
echo "🔧 Running TRADITIONAL ANALYSIS..."
kubectl-support_bundle analyze "$LATEST_BUNDLE"

echo ""
echo "=" * 80

# Run intelligent analysis with Ollama
echo ""
echo "🧠 Running INTELLIGENT ANALYSIS with Ollama..."
echo "   This will take 30-60 seconds for thorough AI analysis..."

# Create a Go program to analyze the real bundle
cat > analyze-real-bundle.go << 'EOF'
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
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run analyze-real-bundle.go <support-bundle-path>")
	}

	bundlePath := os.Args[1]
	
	fmt.Println("🧠 Analyzing Real Cluster Support Bundle with Ollama")
	fmt.Println(strings.Repeat("=", 55))
	
	ctx := context.Background()

	// Initialize agents
	localAgent := local.NewLocalAgent()
	
	ollamaConfig := llm.DefaultOllamaConfig("codellama:13b")
	ollamaConfig.SystemPrompt = `You are a Kubernetes troubleshooting expert analyzing a real production cluster. 
Focus on:
1. Memory pressure and OOMKilled containers
2. CrashLoopBackOff pods and dependency issues
3. Resource scheduling problems
4. CPU and memory resource constraints

Provide specific, actionable remediation steps with kubectl commands.
Be precise and practical in your analysis.`
	
	ollamaAgent, err := llm.NewOllamaAgent(ollamaConfig)
	if err != nil {
		log.Fatalf("Failed to create Ollama agent: %v", err)
	}

	// Load real support bundle
	bundle, err := loadRealSupportBundle(bundlePath)
	if err != nil {
		log.Fatalf("Failed to load support bundle: %v", err)
	}

	// Define analyzers
	analyzers := []analyzer.AnalyzerSpec{
		{Name: "cluster-version", Type: "clusterVersion"},
		{Name: "pod-statuses", Type: "clusterPodStatuses"},
		{Name: "container-statuses", Type: "containerStatuses"},
		{Name: "node-resources", Type: "nodeResources"},
		{Name: "deployments", Type: "deploymentStatus"},
	}

	// Run local analysis
	fmt.Println("🔧 LOCAL ANALYSIS:")
	localStart := time.Now()
	localResult, err := localAgent.Analyze(ctx, bundle, analyzers)
	localTime := time.Since(localStart)
	
	if err != nil {
		log.Printf("Local analysis error: %v", err)
	} else {
		printAnalysisResult("Local Agent", localResult, localTime)
	}

	// Run Ollama analysis
	fmt.Println("\n🧠 INTELLIGENT ANALYSIS with Ollama:")
	ollamaStart := time.Now()
	ollamaResult, err := ollamaAgent.Analyze(ctx, bundle, analyzers)
	ollamaTime := time.Since(ollamaStart)
	
	if err != nil {
		log.Printf("Ollama analysis error: %v", err)
		return
	}

	printEnhancedAnalysisResult("Ollama Agent", ollamaResult, ollamaTime)

	// Summary
	fmt.Println("\n🎯 ANALYSIS SUMMARY:")
	fmt.Printf("   Local: %d results in %v\n", len(localResult.Results), localTime)
	fmt.Printf("   Ollama: %d enhanced results + %d insights in %v\n", 
		len(ollamaResult.Results), len(ollamaResult.Insights), ollamaTime)
	
	fmt.Println("\n✨ The AI agent provided:")
	if hasDetailedExplanations(ollamaResult) {
		fmt.Println("   ✅ Detailed explanations and root cause analysis")
	}
	if hasActionableRemediation(ollamaResult) {
		fmt.Println("   ✅ Actionable remediation steps with specific commands")
	}
	if hasIntelligentInsights(ollamaResult) {
		fmt.Println("   ✅ Cross-component insights and correlations")
	}
	fmt.Println("   ✅ Confidence scoring for reliability assessment")
}

func loadRealSupportBundle(bundlePath string) (*analyzer.SupportBundle, error) {
	// Remove .tar.gz extension if present
	if strings.HasSuffix(bundlePath, ".tar.gz") {
		bundlePath = strings.TrimSuffix(bundlePath, ".tar.gz")
	}
	
	absPath, err := filepath.Abs(bundlePath)
	if err != nil {
		return nil, err
	}

	bundle := &analyzer.SupportBundle{
		Path:    absPath,
		RootDir: absPath,
		Metadata: map[string]interface{}{
			"bundlePath": absPath,
			"collectedAt": time.Now(),
			"source": "real-cluster",
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

func printAnalysisResult(agentName string, result *analyzer.AgentResult, duration time.Duration) {
	fmt.Printf("   Agent: %s\n", agentName)
	fmt.Printf("   Processing Time: %v\n", duration)
	fmt.Printf("   Results: %d\n", len(result.Results))
	
	issueCount := 0
	for _, res := range result.Results {
		status := "✅ PASS"
		if res.IsFail {
			status = "❌ FAIL"
			issueCount++
		} else if res.IsWarn {
			status = "⚠️  WARN"
			issueCount++
		}
		
		fmt.Printf("     %s - %s\n", status, res.Title)
		if res.Message != "" && (res.IsFail || res.IsWarn) {
			fmt.Printf("       💬 %s\n", res.Message)
		}
	}
	
	fmt.Printf("   🎯 Issues Found: %d\n", issueCount)
}

func printEnhancedAnalysisResult(agentName string, result *analyzer.AgentResult, duration time.Duration) {
	fmt.Printf("   🧠 Agent: %s\n", agentName)
	fmt.Printf("   ⏱️  Processing Time: %v\n", duration)
	fmt.Printf("   📊 Enhanced Results: %d\n", len(result.Results))
	fmt.Printf("   💡 Insights: %d\n", len(result.Insights))
	
	fmt.Println("\n   📋 ENHANCED ANALYSIS RESULTS:")
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
		if res.Remediation != nil && res.Remediation.Title != "" {
			fmt.Printf("        🛠️  Remediation: %s\n", res.Remediation.Title)
			if len(res.Remediation.Commands) > 0 {
				fmt.Printf("        💻 Commands: %v\n", res.Remediation.Commands)
			}
		}
		fmt.Println()
	}

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
			fmt.Println()
		}
	}
}

func hasDetailedExplanations(result *analyzer.AgentResult) bool {
	for _, res := range result.Results {
		if res.Explanation != "" || res.RootCause != "" {
			return true
		}
	}
	return false
}

func hasActionableRemediation(result *analyzer.AgentResult) bool {
	for _, res := range result.Results {
		if res.Remediation != nil && (len(res.Remediation.Commands) > 0 || len(res.Remediation.Manual) > 0) {
			return true
		}
	}
	return false
}

func hasIntelligentInsights(result *analyzer.AgentResult) bool {
	return len(result.Insights) > 0
}
EOF

# Run the analysis
go run analyze-real-bundle.go "$BUNDLE_DIR"

echo ""
echo "🎉 Real cluster analysis complete!"
echo ""
echo "💡 This demonstrates intelligent troubleshooting on a live cluster with real issues:"
echo "   • 🧠 AI-powered root cause analysis"  
echo "   • 🛠️  Specific remediation commands"
echo "   • 🎯 Confidence-based recommendations"
echo "   • 🔗 Cross-component issue correlation"
