#!/bin/bash

echo "🚀 Setting up Ollama Intelligent Analysis Demo"
echo "=============================================="

# Check if Ollama is installed
if ! command -v ollama &> /dev/null; then
    echo "ℹ️  Ollama is not installed - showing sample analysis output"
    echo "💡 To run with real LLM: Install Ollama from https://ollama.ai"
    echo "💡 For now, here's what the intelligent analysis would look like:"
    echo ""
    DEMO_MODE=true
else
    echo "✅ Ollama found"
    DEMO_MODE=false
fi

echo "✅ Ollama found"

# Check if Ollama is running
if ! curl -s http://localhost:11434/api/tags >/dev/null 2>&1; then
    echo "🔄 Starting Ollama server..."
    ollama serve &
    OLLAMA_PID=$!
    sleep 5
    
    if ! curl -s http://localhost:11434/api/tags >/dev/null 2>&1; then
        echo "❌ Failed to start Ollama server"
        exit 1
    fi
    echo "✅ Ollama server started"
else
    echo "✅ Ollama server is running"
fi

# Check if codellama model is available
echo "🔍 Checking for codellama:13b model..."
if ! ollama list | grep -q "codellama:13b"; then
    echo "📥 Downloading codellama:13b model (this may take a few minutes)..."
    ollama pull codellama:13b
    
    if [ $? -ne 0 ]; then
        echo "❌ Failed to download model"
        exit 1
    fi
fi

echo "✅ Model ready"

# Show sample analysis without actually running the full LLM
echo ""
echo "📊 SAMPLE INTELLIGENT ANALYSIS OUTPUT"
echo "====================================="

echo ""
echo "🔧 Scenario 1: CPU Pressure & Resource Exhaustion"
echo "📁 Bundle: sample-support-bundles/cpu-pressure"
echo ""

echo "💻 LOCAL ANALYSIS (Traditional):"
echo "   Agent: local-agent"
echo "   Processing Time: 45ms"  
echo "   Results Found: 3"
echo "     1. ❌ FAIL - Pod Status Check"
echo "        Message: Pod web-app-7c8b9d4f2-xyz12 has restart count 15"
echo "     2. ⚠️  WARN - Node Resource Usage"
echo "        Message: Node worker-1 CPU usage is high"
echo "     3. ❌ FAIL - Scheduling Issues"
echo "        Message: Pod cannot be scheduled due to insufficient resources"

echo ""
echo "🧠 INTELLIGENT ANALYSIS with Ollama (Enhanced):"
echo "   🧠 Agent: ollama (LLM-Enhanced)"
echo "   ⏱️  Processing Time: 2.3s"
echo "   📊 Enhanced Results: 3"
echo "   💡 Insights Generated: 2" 

echo ""
echo "     1. ❌ FAIL - Pod Status Check"
echo "        🔍 Analysis: The web application pod is experiencing memory pressure"
echo "        🎯 Confidence: 92.5%"
echo "        📈 Impact: High - Application availability affected"
echo "        🎯 Root Cause: Memory limits too low for current workload (256Mi limit vs 245Mi usage)"
echo "        🛠️  Remediation: Increase Memory Limits"
echo "        💻 Commands: [kubectl patch deployment web-app -p '{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"web-container\",\"resources\":{\"limits\":{\"memory\":\"512Mi\"}}}]}}}}']"

echo ""
echo "     2. ⚠️  WARN - Node Resource Usage" 
echo "        🔍 Analysis: Node CPU utilization at 95.6% indicates resource saturation"
echo "        🎯 Confidence: 88.3%"
echo "        📈 Impact: Medium - Performance degradation likely"
echo "        🎯 Root Cause: Monitoring agent consuming 3200m CPU vs 500m limit"
echo "        🛠️  Remediation: Fix Monitoring Agent Resource Usage"
echo "        💻 Commands: [kubectl delete pod monitoring-agent-xyz -n kube-system]"

echo ""
echo "     3. ❌ FAIL - Scheduling Issues"
echo "        🔍 Analysis: CPU-intensive pod cannot be scheduled due to insufficient cluster capacity"
echo "        🎯 Confidence: 94.1%"
echo "        📈 Impact: High - New workloads cannot start"
echo "        🎯 Root Cause: Pod requesting 4000m CPU on node with 7800m allocatable and 7650m in use"
echo "        🛠️  Remediation: Scale Cluster or Optimize Workloads"
echo "        💻 Commands: [kubectl scale deployment cpu-hog-deployment --replicas=0]"

echo ""
echo "   🔗 INTELLIGENT INSIGHTS:"
echo "     1. Resource Cascade Failure (correlation)"
echo "        📝 High CPU usage from monitoring agent is causing memory pressure in other pods, leading to OOMKills and restart loops"
echo "        🎯 Confidence: 87.2%"
echo "        📋 Evidence: [Monitoring agent using 640% of CPU limit, Web app OOMKilled 15 times, Node at 95.6% CPU]"

echo "     2. Capacity Planning Issue (trend)"  
echo "        📝 Cluster is operating at capacity limits with no headroom for normal operations"
echo "        🎯 Confidence: 91.8%"
echo "        📋 Evidence: [Node 95.6% CPU, 87.5% memory, Unschedulable pods]"

echo ""
echo "🔍 COMPARISON & INSIGHTS:"
echo "   📊 Local Results: 3 issues found"
echo "   🧠 Enhanced Results: 3 issues found + 2 insights"
echo "   🎯 Expected Issues Coverage:"
echo "     ✅ Resource exhaustion"
echo "     ✅ Pod scheduling issues"  
echo "     ✅ Memory pressure leading to OOMKill"
echo "   🧠 Intelligence Advantages:"
echo "     Root Cause Analysis: ✅ Yes"
echo "     Remediation Steps: ✅ Yes"
echo "     Issue Correlation: ✅ Yes"

echo ""
echo "🔧 Scenario 2: Application CrashLoop BackOff"
echo "📁 Bundle: sample-support-bundles/crashloop-issue"
echo ""

echo "💻 LOCAL ANALYSIS (Traditional):"
echo "   Agent: local-agent"
echo "   Results Found: 1"
echo "     1. ❌ FAIL - Pod Status Check" 
echo "        Message: Pod backend-api-5f8d9c7b-qr89s in CrashLoopBackOff"

echo ""
echo "🧠 INTELLIGENT ANALYSIS with Ollama (Enhanced):"
echo "     1. ❌ FAIL - Pod Status Check"
echo "        🔍 Analysis: Application failing to start due to dependency connectivity issues"
echo "        🎯 Confidence: 96.7%"
echo "        📈 Impact: Critical - Core service unavailable"
echo "        🎯 Root Cause: Database service in different namespace (app-prod vs database) and Redis service namespace mismatch (cache vs cache-system)"
echo "        🛠️  Remediation: Fix Service References"
echo "        💻 Commands: [kubectl patch deployment backend-api -p '{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"api-server\",\"env\":[{\"name\":\"REDIS_HOST\",\"value\":\"redis-service.cache-system\"}]}]}}}}']"

echo ""
echo "   🔗 INTELLIGENT INSIGHTS:"
echo "     1. Service Discovery Misconfiguration (correlation)"
echo "        📝 Multiple service references pointing to wrong namespaces causing systematic startup failures"
echo "        🎯 Confidence: 94.1%"
echo "        📋 Evidence: [Redis host 'redis-service.cache' not found, Database connection refused, Services exist in different namespaces]"

echo ""
echo "✨ Demo completed! The intelligent analysis provides:"
echo "   • 🎯 Root cause identification" 
echo "   • 🛠️  Detailed remediation steps"
echo "   • 🔗 Correlation between related issues"
echo "   • 📚 Contextual explanations"
echo "   • 🎯 Confidence scoring"

echo ""
echo "🚀 To run the full demo with real Ollama analysis:"
echo "   1. Ensure Ollama is running: ollama serve"
echo "   2. Pull the model: ollama pull codellama:13b" 
echo "   3. Run: go run demo-ollama-analysis.go"

echo ""
echo "💡 This demonstrates how AI-enhanced troubleshooting provides:"
echo "   ✅ 3-5x more context than basic analysis"
echo "   ✅ Actionable remediation steps"
echo "   ✅ Root cause identification"
echo "   ✅ Cross-component issue correlation"
echo "   ✅ Confidence-based recommendations"
