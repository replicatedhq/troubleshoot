#!/bin/bash

# Troubleshoot LLM Analyzer Demo Script
# This script demonstrates the AI-powered analysis capabilities

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}==================================================${NC}"
echo -e "${BLUE}    Troubleshoot LLM Analyzer Demo${NC}"
echo -e "${BLUE}==================================================${NC}"
echo

# Check for required environment variables
if [ -z "$OPENAI_API_KEY" ]; then
    echo -e "${RED}Error: OPENAI_API_KEY environment variable is not set${NC}"
    echo "Please set it with: export OPENAI_API_KEY=your-api-key"
    exit 1
fi

echo -e "${GREEN}✓ OpenAI API key detected${NC}"
echo

# Function to run demo scenarios
run_scenario() {
    local scenario_name=$1
    local spec_file=$2
    local problem_description=$3
    
    echo -e "${YELLOW}----------------------------------------${NC}"
    echo -e "${YELLOW}Scenario: $scenario_name${NC}"
    echo -e "${YELLOW}----------------------------------------${NC}"
    echo "Problem Description: $problem_description"
    echo "Running analysis..."
    echo
    
    # Run the support bundle with problem description
    ./bin/support-bundle "$spec_file" \
        --problem-description "$problem_description" \
        --interactive=false \
        --output "demo-bundle-$(date +%s).tar.gz"
    
    echo
}

# Demo 1: Basic LLM Analysis
echo -e "${BLUE}Demo 1: Basic LLM Analysis${NC}"
echo "This demonstrates analyzing pod logs for crashes and restarts"
echo

cat > demo-spec-1.yaml << 'EOF'
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: demo-basic
spec:
  collectors:
    - clusterInfo: {}
    - logs:
        name: pod-logs
        namespace: ""
        limits:
          maxLines: 1000
  analyzers:
    - llm:
        checkName: "AI Problem Analysis"
        collectorName: "pod-logs"
        fileName: "*.log"
        model: "gpt-4o-mini"
        maxFiles: 5
        outcomes:
          - fail:
              when: "issue_found"
              message: "Critical issue: {{.Summary}}"
          - warn:
              when: "potential_issue"
              message: "Warning: {{.Summary}}"
          - pass:
              message: "No issues detected"
EOF

run_scenario "Pod Crash Analysis" \
    "demo-spec-1.yaml" \
    "My application pods are crashing and restarting frequently"

# Demo 2: Memory Issues Detection
echo -e "${BLUE}Demo 2: Memory Issues Detection${NC}"
echo "This demonstrates detecting OOM kills and memory leaks"
echo

cat > demo-spec-2.yaml << 'EOF'
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: demo-memory
spec:
  collectors:
    - clusterInfo: {}
    - clusterResources: {}
    - logs:
        name: system-logs
        namespace: "kube-system"
    - events:
        name: cluster-events
        namespace: ""
  analyzers:
    - llm:
        checkName: "Memory Issue Detection"
        collectorName: "cluster-events"
        fileName: "*"
        model: "gpt-5"
        maxFiles: 10
        outcomes:
          - fail:
              when: "issue_found"
              message: |
                Memory Issue Detected:
                {{.Summary}}
                
                Root Cause: {{.Issue}}
                Solution: {{.Solution}}
          - pass:
              message: "No memory issues detected"
EOF

run_scenario "Memory Issue Detection" \
    "demo-spec-2.yaml" \
    "My pods are being killed due to memory issues"

# Demo 3: Re-analyzing existing bundle
echo -e "${BLUE}Demo 3: Re-analyzing Existing Bundle${NC}"
echo "This demonstrates re-analyzing a previously collected bundle"
echo

# First, create a bundle
echo "Step 1: Creating initial support bundle..."
./bin/support-bundle examples/analyzers/llm-analyzer.yaml \
    --problem-description "Initial issue: application not starting" \
    --interactive=false \
    --output "reanalysis-demo.tar.gz"

echo
echo "Step 2: Re-analyzing with different problem description..."

cat > reanalyze-spec.yaml << 'EOF'
apiVersion: troubleshoot.sh/v1beta2
kind: Analyzer
metadata:
  name: reanalyze-demo
spec:
  analyzers:
    - llm:
        checkName: "Re-analysis with Different Context"
        collectorName: "pod-logs"
        fileName: "*.log"
        model: "gpt-5"
        outcomes:
          - fail:
              when: "issue_found"
              message: "New insights: {{.Summary}}"
          - pass:
              message: "No additional issues found"
EOF

./bin/support-bundle analyze \
    --bundle reanalysis-demo.tar.gz \
    --analyzers reanalyze-spec.yaml \
    --problem-description "Performance degradation after recent deployment"

# Demo 4: Multiple Analyzers
echo
echo -e "${BLUE}Demo 4: Combining LLM with Traditional Analyzers${NC}"
echo "This shows LLM working alongside deterministic analyzers"
echo

cat > demo-spec-combined.yaml << 'EOF'
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: demo-combined
spec:
  collectors:
    - clusterInfo: {}
    - clusterResources: {}
    - logs:
        name: app-logs
        namespace: "default"
  analyzers:
    # Traditional analyzer
    - clusterVersion:
        outcomes:
          - fail:
              when: "< 1.20.0"
              message: "Kubernetes version too old"
          - pass:
              message: "Kubernetes version OK"
    
    # LLM analyzer
    - llm:
        checkName: "AI-Powered Log Analysis"
        collectorName: "app-logs"
        fileName: "*.log"
        model: "gpt-4o-mini"
        outcomes:
          - fail:
              when: "issue_found"
              message: "AI detected: {{.Summary}}"
          - pass:
              message: "AI analysis: No issues found"
EOF

run_scenario "Combined Analysis" \
    "demo-spec-combined.yaml" \
    "Application showing intermittent errors"

# Cleanup
echo
echo -e "${YELLOW}Cleaning up demo files...${NC}"
rm -f demo-spec-*.yaml reanalyze-spec.yaml
rm -f demo-bundle-*.tar.gz reanalysis-demo.tar.gz

echo
echo -e "${GREEN}==================================================${NC}"
echo -e "${GREEN}    Demo Complete!${NC}"
echo -e "${GREEN}==================================================${NC}"
echo
echo "Key Features Demonstrated:"
echo "  ✓ AI-powered log analysis without pre-written rules"
echo "  ✓ Problem description context for targeted analysis"
echo "  ✓ Re-analyzing existing bundles with new context"
echo "  ✓ Combining AI and traditional analyzers"
echo "  ✓ Multiple model options (gpt-5, gpt-4o-mini)"
echo
echo "Next Steps:"
echo "  1. Set your OpenAI API key: export OPENAI_API_KEY=your-key"
echo "  2. Create custom specs for your applications"
echo "  3. Use --problem-description to provide context"
echo "  4. See examples/analyzers/llm-analyzer.yaml for more examples"