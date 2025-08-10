#!/bin/bash

# End-to-end test for LLM analyzer
# This script tests the full workflow without requiring a real API key

set -e

echo "=== LLM Analyzer E2E Test ==="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test directory
TEST_DIR=$(mktemp -d)
trap "rm -rf $TEST_DIR" EXIT

echo "Test directory: $TEST_DIR"

# Create a test spec
cat > $TEST_DIR/test-spec.yaml <<EOF
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: llm-test
spec:
  collectors:
    - logs:
        name: test-logs
        namespace: default
        selector:
          - app=test
  analyzers:
    - llm:
        checkName: "Test LLM Analysis"
        collectorName: "test-logs"
        fileName: "*"
        model: "gpt-4"
        outcomes:
          - fail:
              when: "issue_found"
              message: "Issue detected: {{.Summary}}"
          - pass:
              message: "No issues found"
EOF

# Test 1: Missing API key
echo -e "\n${YELLOW}Test 1: Missing API key${NC}"
unset OPENAI_API_KEY
OUTPUT=$(./bin/support-bundle $TEST_DIR/test-spec.yaml --interactive=false 2>&1 || true)
if echo "$OUTPUT" | grep -q "OPENAI_API_KEY environment variable is required"; then
    echo -e "${GREEN}✓ Correctly detected missing API key${NC}"
else
    echo -e "${RED}✗ Failed to detect missing API key${NC}"
    exit 1
fi

# Test 2: Invalid API key
echo -e "\n${YELLOW}Test 2: Invalid API key${NC}"
OPENAI_API_KEY="invalid-key-123" OUTPUT=$(./bin/support-bundle $TEST_DIR/test-spec.yaml --interactive=false --debug 2>&1 || true)
if echo "$OUTPUT" | grep -q "Incorrect API key provided"; then
    echo -e "${GREEN}✓ Correctly handled invalid API key${NC}"
else
    echo -e "${RED}✗ Failed to handle invalid API key${NC}"
    exit 1
fi

# Test 3: Problem description flag
echo -e "\n${YELLOW}Test 3: Problem description flag${NC}"
OPENAI_API_KEY="test-key" OUTPUT=$(./bin/support-bundle $TEST_DIR/test-spec.yaml \
    --problem-description "Test problem" \
    --interactive=false \
    --dry-run 2>&1)
if echo "$OUTPUT" | grep -q "llm:"; then
    echo -e "${GREEN}✓ LLM analyzer included in spec${NC}"
else
    echo -e "${RED}✗ LLM analyzer not found in spec${NC}"
    exit 1
fi

# Test 4: Check analyzer registration
echo -e "\n${YELLOW}Test 4: Analyzer registration${NC}"
if ./bin/support-bundle $TEST_DIR/test-spec.yaml --dry-run | grep -q "checkName: Test LLM Analysis"; then
    echo -e "${GREEN}✓ LLM analyzer properly registered${NC}"
else
    echo -e "${RED}✗ LLM analyzer not registered${NC}"
    exit 1
fi

# Test 5: File pattern matching
echo -e "\n${YELLOW}Test 5: File pattern configuration${NC}"
cat > $TEST_DIR/pattern-spec.yaml <<EOF
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: pattern-test
spec:
  collectors:
    - clusterInfo: {}
  analyzers:
    - llm:
        collectorName: "cluster-info"
        fileName: "*.json"
        maxFiles: 3
EOF

if ./bin/support-bundle $TEST_DIR/pattern-spec.yaml --dry-run | grep -q 'fileName: "\*.json"'; then
    echo -e "${GREEN}✓ File pattern correctly configured${NC}"
else
    echo -e "${RED}✗ File pattern not configured${NC}"
    exit 1
fi

echo -e "\n${GREEN}=== All E2E tests passed ===${NC}"