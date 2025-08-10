#!/bin/bash
# Demo script for the LLM Analyzer

set -e

echo "==================================="
echo "Troubleshoot LLM Analyzer Demo"
echo "==================================="
echo

# Check for API key
if [ -z "$OPENAI_API_KEY" ]; then
    if [ -f .env ]; then
        echo "Loading API key from .env file..."
        source .env
    else
        echo "Error: OPENAI_API_KEY not set and .env file not found"
        echo "Please set your OpenAI API key:"
        echo "  export OPENAI_API_KEY=your-key-here"
        exit 1
    fi
fi

echo "✓ API key loaded (length: ${#OPENAI_API_KEY})"
echo

# Step 1: Build the project
echo "Step 1: Building the project..."
make build 2>&1 | tail -2
echo "✓ Build complete"
echo

# Step 2: Create a sample support bundle
echo "Step 2: Creating sample support bundle..."
mkdir -p demo-bundle/pod-logs/default
mkdir -p demo-bundle/cluster-resources/pods
mkdir -p demo-bundle/cluster-resources/events

# Create version file (required by troubleshoot)
echo "version: '1'" > demo-bundle/version.yaml
cp demo-bundle/version.yaml demo-bundle/cluster-resources/

# Create sample pod logs with issues
cat > demo-bundle/pod-logs/default/app-pod.log << 'EOF'
2024-12-10T10:15:00Z INFO Starting application...
2024-12-10T10:15:01Z INFO Connecting to database...
2024-12-10T10:15:02Z ERROR Failed to connect to database: connection refused
2024-12-10T10:15:03Z INFO Retrying connection...
2024-12-10T10:15:04Z ERROR Failed to connect to database: connection refused
2024-12-10T10:15:05Z FATAL Application terminated due to database connection failure
2024-12-10T10:15:06Z ERROR OOMKilled: Container exceeded memory limit
EOF

cat > demo-bundle/pod-logs/default/db-pod.log << 'EOF'
2024-12-10T10:10:00Z INFO PostgreSQL starting...
2024-12-10T10:10:01Z ERROR FATAL: could not create shared memory segment: No space left on device
2024-12-10T10:10:02Z INFO Shutting down...
EOF

# Create a tar bundle
tar -czf demo-bundle.tar.gz -C demo-bundle . 2>/dev/null
rm -rf demo-bundle
echo "✓ Sample bundle created"
echo

# Step 3: Create analyzer spec
echo "Step 3: Creating analyzer spec..."
cat > demo-analyzer.yaml << 'EOF'
apiVersion: troubleshoot.sh/v1beta2
kind: Analyzer
metadata:
  name: llm-demo
spec:
  analyzers:
    - llm:
        checkName: "AI-Powered Log Analysis"
        collectorName: "pod-logs/default"
        fileName: "*.log"
        model: "gpt-4o-mini"
        maxFiles: 10
        priorityPatterns:
          - "error"
          - "fatal"
          - "OOM"
          - "crash"
        outcomes:
          - fail:
              when: "issue_found"
              message: "Critical Issue Found: {{.Summary}}"
          - pass:
              message: "No critical issues detected"
EOF
echo "✓ Analyzer spec created"
echo

# Step 4: Run the analyzer
echo "Step 4: Running LLM analyzer..."
echo "Problem description: Application is experiencing connectivity issues and crashes"
echo
echo "Output:"
echo "-------"
OPENAI_API_KEY="$OPENAI_API_KEY" \
PROBLEM_DESCRIPTION="Application is experiencing connectivity issues and crashes" \
./bin/analyze demo-bundle.tar.gz --analyzers demo-analyzer.yaml

echo
echo "==================================="
echo "Demo Complete!"
echo "==================================="
echo
echo "The LLM analyzer has successfully:"
echo "1. Analyzed the log files in the support bundle"
echo "2. Identified the root cause of the issues"
echo "3. Provided actionable recommendations"
echo
echo "To use with your own support bundles:"
echo "1. Collect a support bundle: ./bin/support-bundle your-spec.yaml"
echo "2. Analyze it: OPENAI_API_KEY=\$OPENAI_API_KEY ./bin/analyze bundle.tar.gz --analyzers llm-analyzer.yaml"
echo

# Cleanup
rm -f demo-bundle.tar.gz demo-analyzer.yaml