#!/bin/bash
# Simple test script for LLM analyzer

set -e

echo "🤖 LLM Analyzer Simple Test"
echo "==========================="
echo

# Source .envrc if it exists and API key isn't already set
if [ -z "$OPENAI_API_KEY" ] && [ -f ".envrc" ]; then
    echo "📂 Loading API key from .envrc..."
    source .envrc
fi

# Check for API key
if [ -z "$OPENAI_API_KEY" ]; then
    echo "⚠️  Please set your OpenAI API key:"
    echo "   export OPENAI_API_KEY='sk-...'"
    echo "   Or add it to .envrc file"
    echo
    echo "   Get one at: https://platform.openai.com/api-keys"
    exit 1
fi

echo "✅ API Key is set"
echo

# Create a minimal test bundle
echo "Creating test support bundle..."
mkdir -p test-bundle/cluster-resources
mkdir -p test-bundle/pod-logs/default

# Add required version file
echo "version: '1'" > test-bundle/version.yaml
cp test-bundle/version.yaml test-bundle/cluster-resources/

# Create a log file with clear OOM errors
cat > test-bundle/pod-logs/default/app.log << 'EOF'
2024-01-10T10:00:00Z INFO Starting application
2024-01-10T10:00:10Z ERROR java.lang.OutOfMemoryError: Java heap space
2024-01-10T10:00:11Z FATAL Application crashed - OOMKilled
2024-01-10T10:00:12Z INFO Container restarting
2024-01-10T10:00:20Z ERROR java.lang.OutOfMemoryError: Java heap space
2024-01-10T10:00:21Z FATAL Application crashed - OOMKilled
EOF

# Create the bundle
tar -czf test-bundle.tar.gz -C test-bundle . 2>/dev/null
rm -rf test-bundle

echo "✅ Created test-bundle.tar.gz"
echo

# Create a simple analyzer spec
cat > test-analyzer.yaml << 'EOF'
apiVersion: troubleshoot.sh/v1beta2
kind: Analyzer
metadata:
  name: test-llm
spec:
  analyzers:
    - llm:
        checkName: "AI Analysis"
        collectorName: "pod-logs"
        fileName: "*"
        model: "gpt-4o-mini"
        outcomes:
          - fail:
              when: "issue_found"
              message: |
                Issue: {{.Summary}}
                Solution: {{.Solution}}
          - pass:
              message: "No issues found"
EOF

echo "Running LLM analysis..."
echo "Problem: Application OOM errors"
echo "---"

# Run the analyzer
export PROBLEM_DESCRIPTION="Application experiencing OOM errors"
./bin/analyze test-bundle.tar.gz --analyzers test-analyzer.yaml

# Cleanup
rm -f test-bundle.tar.gz test-analyzer.yaml

echo
echo "✅ Test complete!"