#!/bin/bash

# Quick demo of LLM analyzer functionality
set -e

echo "🤖 Troubleshoot LLM Analyzer Quick Demo"
echo "========================================"
echo

# Check API key
if [ -z "$OPENAI_API_KEY" ]; then
    echo "❌ Please set OPENAI_API_KEY environment variable"
    exit 1
fi

echo "✅ API Key configured"
echo

# Create a simple test spec
cat > test-llm.yaml << 'EOF'
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: quick-test
spec:
  collectors:
    - clusterInfo: {}
    - logs:
        name: all-logs
        namespace: ""
        limits:
          maxLines: 500
  analyzers:
    - llm:
        checkName: "AI Analysis"
        collectorName: "all-logs"
        fileName: "*.log"
        model: "gpt-4o-mini"
        maxFiles: 5
        outcomes:
          - fail:
              when: "issue_found"
              message: "Issue: {{.Summary}}"
          - warn:
              when: "potential_issue"
              message: "Warning: {{.Summary}}"
          - pass:
              message: "No issues found"
EOF

# Run the analysis
echo "Running LLM analysis..."
echo "Problem: 'Pods are crashing with OOM errors'"
echo

./bin/support-bundle test-llm.yaml \
    --problem-description "Pods are crashing with OOM errors" \
    --interactive=false

echo
echo "✅ Demo complete! The LLM analyzer has analyzed your cluster."
echo

# Cleanup
rm -f test-llm.yaml