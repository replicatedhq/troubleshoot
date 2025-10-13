#!/bin/bash
# Comprehensive test for affected test detection
# Tests various code change scenarios to ensure correct suite detection

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

TESTS_PASSED=0
TESTS_FAILED=0

echo "========================================"
echo "Affected Test Detection Validation"
echo "========================================"
echo ""

# Helper function to run test
run_test() {
    local test_name="$1"
    local test_file="$2"
    local expected_suites="$3"

    echo -e "${BLUE}Test: $test_name${NC}"
    echo "File: $test_file"
    echo "Expected: $expected_suites"

    # Get affected tests from explicit changed files (no git required); detector prints <suite>:<TestName>
    local detector_output=$(go run ./scripts/affected-packages.go -mode=suites -changed-files "$test_file" 2>/dev/null)
    # Derive suites from prefixes for comparison
    local actual_suites=$(echo "$detector_output" | cut -d':' -f1 | grep -v '^$' | sort | uniq | tr '\n' ' ' | xargs)

    # Compare results
    if [ "$actual_suites" = "$expected_suites" ]; then
        echo -e "${GREEN}✓ PASS${NC} - Got: $actual_suites"
        if [ -n "$detector_output" ]; then
            echo "Tests:" && echo "$detector_output" | sed 's/^/  - /'
        fi
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗ FAIL${NC} - Got: '$actual_suites', Expected: '$expected_suites'"
        if [ -n "$detector_output" ]; then
            echo "Tests:" && echo "$detector_output" | sed 's/^/  - /'
        fi
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    echo ""
}

# Test 1: Preflight-only package (should only trigger preflight)
run_test "Preflight-only package change" \
    "pkg/preflight/run.go" \
    "preflight"

# Test 2: Support-bundle-only package
run_test "Support-bundle-only package change" \
    "pkg/supportbundle/supportbundle.go" \
    "support-bundle"

# Test 3: Shared package - collect
run_test "Shared package (collect) change" \
    "pkg/collect/run.go" \
    "preflight support-bundle"

# Test 4: Shared package - analyze
run_test "Shared package (analyze) change" \
    "pkg/analyze/analyzer.go" \
    "preflight support-bundle"

# Test 5: Shared package - k8sutil
run_test "Shared package (k8sutil) change" \
    "pkg/k8sutil/config.go" \
    "preflight support-bundle"

# Test 6: Shared package - convert
run_test "Shared package (convert) change" \
    "pkg/convert/output.go" \
    "preflight support-bundle"

# Test 7: Shared package - redact (another shared one)
run_test "Shared package (redact) change" \
    "pkg/redact/redact.go" \
    "preflight support-bundle"

# Test 8: Preflight command (should only trigger preflight)
run_test "Preflight command change" \
    "cmd/preflight/main.go" \
    "preflight"

# Test 9: Support-bundle types (support-bundle only package)
run_test "Support-bundle types change" \
    "pkg/supportbundle/types/types.go" \
    "support-bundle"

# Test 10: Workflow file (should not trigger e2e)
echo -e "${BLUE}Test: Workflow file change (should trigger nothing)${NC}"
echo "File: .github/workflows/affected-tests.yml"
echo "Expected: (no suites)"

detector_output=$(go run ./scripts/affected-packages.go -mode=suites -changed-files ".github/workflows/affected-tests.yml" 2>/dev/null)
actual_suites=$(echo "$detector_output" | cut -d':' -f1 | grep -v '^$' | sort | uniq | tr '\n' ' ' | xargs)

if [ -z "$actual_suites" ]; then
    echo -e "${GREEN}✓ PASS${NC} - No suites affected (as expected)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}✗ FAIL${NC} - Got: '$actual_suites', Expected: (empty)"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 11: go.mod change (should trigger all)
echo -e "${BLUE}Test: go.mod change (should trigger all suites)${NC}"
echo "File: go.mod"
echo "Expected: preflight support-bundle"

detector_output=$(go run ./scripts/affected-packages.go -mode=suites -changed-files "go.mod" 2>/dev/null)
actual_suites=$(echo "$detector_output" | cut -d':' -f1 | grep -v '^$' | sort | uniq | tr '\n' ' ' | xargs)

if [ "$actual_suites" = "preflight support-bundle" ]; then
    echo -e "${GREEN}✓ PASS${NC} - Got: $actual_suites"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}✗ FAIL${NC} - Got: '$actual_suites', Expected: 'preflight support-bundle'"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 12: Multiple files across different areas
echo -e "${BLUE}Test: Multiple file changes (support-bundle + shared)${NC}"
echo "Files: pkg/supportbundle/supportbundle.go + pkg/collect/run.go"
echo "Expected: preflight support-bundle"

detector_output=$(go run ./scripts/affected-packages.go -mode=suites -changed-files "pkg/supportbundle/supportbundle.go,pkg/collect/run.go" 2>/dev/null)
actual_suites=$(echo "$detector_output" | cut -d':' -f1 | grep -v '^$' | sort | uniq | tr '\n' ' ' | xargs)

if [ "$actual_suites" = "preflight support-bundle" ]; then
    echo -e "${GREEN}✓ PASS${NC} - Got: $actual_suites"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}✗ FAIL${NC} - Got: '$actual_suites', Expected: 'preflight support-bundle'"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 13: README change (should not trigger e2e)
echo -e "${BLUE}Test: Documentation change (should trigger nothing)${NC}"
echo "File: README.md"
echo "Expected: (no suites)"

detector_output=$(go run ./scripts/affected-packages.go -mode=suites -changed-files "README.md" 2>/dev/null)
actual_suites=$(echo "$detector_output" | cut -d':' -f1 | grep -v '^$' | sort | uniq | tr '\n' ' ' | xargs)

if [ -z "$actual_suites" ]; then
    echo -e "${GREEN}✓ PASS${NC} - No suites affected (as expected)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}✗ FAIL${NC} - Got: '$actual_suites', Expected: (empty)"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Summary
echo "========================================"
echo -e "${GREEN}Tests Passed: $TESTS_PASSED${NC}"
echo -e "${RED}Tests Failed: $TESTS_FAILED${NC}"
echo "========================================"

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}✗ Some tests failed${NC}"
    exit 1
fi


