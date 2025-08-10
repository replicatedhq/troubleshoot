#!/bin/bash
# Test if environment variables are being passed correctly

echo "Testing environment variable passing..."

# Source the .env file
source .env

# Check if API key is set
if [ -z "$OPENAI_API_KEY" ]; then
    echo "❌ API key not loaded from .env"
    exit 1
fi

echo "✅ API key loaded (length: ${#OPENAI_API_KEY})"

# Set problem description
export PROBLEM_DESCRIPTION="Test problem"

# Create a minimal test bundle
mkdir -p test-bundle/cluster-resources
mkdir -p test-bundle/pod-logs/default
echo "version: '1'" > test-bundle/version.yaml
cp test-bundle/version.yaml test-bundle/cluster-resources/
echo "ERROR: Test error message" > test-bundle/pod-logs/default/test.log
tar -czf test-bundle.tar.gz -C test-bundle . 2>/dev/null
rm -rf test-bundle

# Create minimal analyzer
cat > test-analyzer.yaml << 'EOF'
apiVersion: troubleshoot.sh/v1beta2
kind: Analyzer
metadata:
  name: test
spec:
  analyzers:
    - llm:
        checkName: "LLM Test"
        collectorName: "pod-logs/default"
        fileName: "*.log"
        model: "gpt-4o-mini"
        outcomes:
          - fail:
              when: "issue_found"
              message: "Found: {{.Summary}}"
          - pass:
              message: "No issues"
EOF

echo "Running analyzer with API key..."
OPENAI_API_KEY="$OPENAI_API_KEY" PROBLEM_DESCRIPTION="$PROBLEM_DESCRIPTION" ./bin/analyze test-bundle.tar.gz --analyzers test-analyzer.yaml

# Cleanup
rm -f test-bundle.tar.gz test-analyzer.yaml