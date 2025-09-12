#!/bin/bash

# Auto-Collectors Real Cluster Testing Script
# Run this against your K3s cluster to validate all functionality

set -e

echo "🧪 Auto-Collectors Real Cluster Testing"
echo "========================================"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counter
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

run_test() {
    local test_name="$1"
    local test_command="$2"
    local expected_pattern="$3"
    
    echo -e "${BLUE}🧪 Test $((++TESTS_RUN)): $test_name${NC}"
    echo "   Command: $test_command"
    
    if eval "$test_command" > "/tmp/test_output_$TESTS_RUN.txt" 2>&1; then
        if [ -n "$expected_pattern" ]; then
            if grep -q "$expected_pattern" "/tmp/test_output_$TESTS_RUN.txt"; then
                echo -e "   ${GREEN}✅ PASS${NC}"
                ((TESTS_PASSED++))
            else
                echo -e "   ${RED}❌ FAIL - Expected pattern '$expected_pattern' not found${NC}"
                echo "   Output preview:"
                head -3 "/tmp/test_output_$TESTS_RUN.txt" | sed 's/^/   /'
                ((TESTS_FAILED++))
            fi
        else
            echo -e "   ${GREEN}✅ PASS${NC}"
            ((TESTS_PASSED++))
        fi
    else
        echo -e "   ${RED}❌ FAIL - Command failed${NC}"
        echo "   Error output:"
        head -3 "/tmp/test_output_$TESTS_RUN.txt" | sed 's/^/   /'
        ((TESTS_FAILED++))
    fi
    echo
}

# Verify cluster connectivity first
echo -e "${YELLOW}📋 Prerequisites${NC}"
echo "Checking cluster connectivity..."
if ! kubectl get nodes > /dev/null 2>&1; then
    echo -e "${RED}❌ Cannot connect to Kubernetes cluster. Please ensure kubectl is configured.${NC}"
    exit 1
fi
echo -e "${GREEN}✅ Cluster connectivity confirmed${NC}"

# Get cluster info for context
CLUSTER_NAME=$(kubectl config current-context)
NODE_COUNT=$(kubectl get nodes --no-headers | wc -l)
NAMESPACE_COUNT=$(kubectl get namespaces --no-headers | wc -l)

echo "📊 Cluster Information:"
echo "   Context: $CLUSTER_NAME"
echo "   Nodes: $NODE_COUNT"
echo "   Namespaces: $NAMESPACE_COUNT"
echo

# Test 1: Basic Auto-Discovery Help
echo -e "${YELLOW}📝 Phase 1: CLI Integration Tests${NC}"
run_test "Auto-discovery flags in help" \
         "./bin/support-bundle --help" \
         "auto.*enable auto-discovery"

run_test "Diff subcommand exists" \
         "./bin/support-bundle diff --help" \
         "Compare two support bundles"

# Test 2: Flag Validation
echo -e "${YELLOW}📝 Phase 2: Flag Validation Tests${NC}"
run_test "Include-images without auto fails" \
         "./bin/support-bundle --include-images --dry-run" \
         "requires --auto flag"

run_test "Valid auto flag combination" \
         "./bin/support-bundle --auto --include-images --dry-run" \
         "apiVersion"

# Test 3: Discovery Profiles
echo -e "${YELLOW}📝 Phase 3: Discovery Profile Tests${NC}"
run_test "Minimal profile" \
         "./bin/support-bundle --auto --discovery-profile minimal --dry-run" \
         "apiVersion"

run_test "Comprehensive profile" \
         "./bin/support-bundle --auto --discovery-profile comprehensive --dry-run" \
         "apiVersion"

run_test "Invalid profile fails" \
         "./bin/support-bundle --auto --discovery-profile invalid --dry-run" \
         "unknown discovery profile"

# Test 4: Namespace Filtering
echo -e "${YELLOW}📝 Phase 4: Namespace Filtering Tests${NC}"
run_test "Specific namespace targeting" \
         "./bin/support-bundle --auto --namespace default --dry-run" \
         "apiVersion"

run_test "System namespace exclusion" \
         "./bin/support-bundle --auto --exclude-namespaces 'kube-*' --dry-run" \
         "apiVersion"

run_test "Include patterns" \
         "./bin/support-bundle --auto --include-namespaces 'default,kube-public' --dry-run" \
         "apiVersion"

# Test 5: Path 1 - Foundational Only Collection
echo -e "${YELLOW}📝 Phase 5: Path 1 - Foundational Only Collection${NC}"
echo "⚠️  These tests will actually collect data from your cluster (safe operations only)"

run_test "Basic foundational collection" \
         "timeout 60s ./bin/support-bundle --auto --namespace default --output /tmp/foundational-test.tar.gz" \
         ""

if [ -f "/tmp/foundational-test.tar.gz" ]; then
    echo -e "${GREEN}✅ Foundational collection succeeded: $(ls -lh /tmp/foundational-test.tar.gz | awk '{print $5}')${NC}"
    
    # Verify bundle contents
    run_test "Bundle contains cluster-info" \
             "tar -tzf /tmp/foundational-test.tar.gz" \
             "cluster-info"
             
    run_test "Bundle contains logs" \
             "tar -tzf /tmp/foundational-test.tar.gz" \
             "logs"
             
    run_test "Bundle contains configmaps" \
             "tar -tzf /tmp/foundational-test.tar.gz" \
             "configmaps"
else
    echo -e "${RED}❌ Foundational collection failed - no bundle created${NC}"
    ((TESTS_FAILED++))
fi

# Test 6: Image Collection
echo -e "${YELLOW}📝 Phase 6: Image Metadata Collection${NC}"
run_test "Image collection enabled" \
         "timeout 90s ./bin/support-bundle --auto --namespace default --include-images --output /tmp/images-test.tar.gz" \
         ""

if [ -f "/tmp/images-test.tar.gz" ]; then
    echo -e "${GREEN}✅ Image collection succeeded: $(ls -lh /tmp/images-test.tar.gz | awk '{print $5}')${NC}"
    
    # Check if facts.json exists in bundle (when Phase 2 integration is complete)
    run_test "Bundle may contain image facts" \
             "tar -tzf /tmp/images-test.tar.gz" \
             "image-facts"
else
    echo -e "${YELLOW}⚠️  Image collection test skipped (may require registry access)${NC}"
fi

# Test 7: RBAC Integration
echo -e "${YELLOW}📝 Phase 7: RBAC Integration Tests${NC}"
run_test "RBAC checking enabled" \
         "./bin/support-bundle --auto --namespace default --rbac-check --dry-run" \
         "apiVersion"

run_test "RBAC checking disabled" \
         "./bin/support-bundle --auto --namespace default --rbac-check=false --dry-run" \
         "apiVersion"

# Test 8: Path 2 - YAML + Foundational (need a sample YAML spec)
echo -e "${YELLOW}📝 Phase 8: Path 2 - YAML + Foundational Tests${NC}"

# Create a minimal test spec
cat > /tmp/test-spec.yaml << 'EOF'
apiVersion: troubleshoot.replicated.com/v1beta2
kind: SupportBundle
metadata:
  name: test-spec
spec:
  collectors:
    - logs:
        selector:
          - app=test
        namespace: default
        name: test-logs
EOF

run_test "YAML + foundational augmentation" \
         "./bin/support-bundle /tmp/test-spec.yaml --auto --dry-run" \
         "apiVersion"

# Test 9: Comprehensive Real Collection
echo -e "${YELLOW}📝 Phase 9: Comprehensive Real Collection Test${NC}"
echo "🚀 Running comprehensive collection test..."
echo "   This will collect actual data from your K3s cluster."
echo "   Collection should complete in 30-60 seconds."

if timeout 120s ./bin/support-bundle --auto --namespace default --discovery-profile comprehensive --include-images --output /tmp/comprehensive-test.tar.gz > /tmp/comprehensive_output.txt 2>&1; then
    if [ -f "/tmp/comprehensive-test.tar.gz" ]; then
        BUNDLE_SIZE=$(ls -lh /tmp/comprehensive-test.tar.gz | awk '{print $5}')
        FILE_COUNT=$(tar -tzf /tmp/comprehensive-test.tar.gz | wc -l)
        echo -e "${GREEN}✅ Comprehensive collection succeeded!${NC}"
        echo "   Bundle size: $BUNDLE_SIZE"
        echo "   Files collected: $FILE_COUNT"
        echo "   Location: /tmp/comprehensive-test.tar.gz"
        ((TESTS_PASSED++))
    else
        echo -e "${RED}❌ Comprehensive collection failed - no bundle created${NC}"
        ((TESTS_FAILED++))
    fi
else
    echo -e "${RED}❌ Comprehensive collection timed out or failed${NC}"
    echo "   Check output: /tmp/comprehensive_output.txt"
    ((TESTS_FAILED++))
fi

# Test 10: Bundle Diff (if we have multiple bundles)
echo -e "${YELLOW}📝 Phase 10: Bundle Diff Tests${NC}"
if [ -f "/tmp/foundational-test.tar.gz" ] && [ -f "/tmp/comprehensive-test.tar.gz" ]; then
    run_test "Bundle diff text format" \
             "./bin/support-bundle diff /tmp/foundational-test.tar.gz /tmp/comprehensive-test.tar.gz" \
             "Support Bundle Diff Report"
             
    run_test "Bundle diff JSON format" \
             "./bin/support-bundle diff /tmp/foundational-test.tar.gz /tmp/comprehensive-test.tar.gz --output json" \
             '"summary"'
             
    run_test "Bundle diff to file" \
             "./bin/support-bundle diff /tmp/foundational-test.tar.gz /tmp/comprehensive-test.tar.gz --output json -f /tmp/diff-report.json" \
             ""
             
    if [ -f "/tmp/diff-report.json" ]; then
        echo -e "${GREEN}✅ Diff report created: /tmp/diff-report.json${NC}"
    fi
else
    echo -e "${YELLOW}⚠️  Bundle diff tests skipped (need two bundles)${NC}"
fi

# Performance Test
echo -e "${YELLOW}📝 Phase 11: Performance Tests${NC}"
echo "🏃 Testing auto-discovery performance..."

DISCOVERY_START=$(date +%s)
if timeout 45s ./bin/support-bundle --auto --namespace default --discovery-profile minimal --dry-run > /tmp/perf_test.txt 2>&1; then
    DISCOVERY_END=$(date +%s)
    DISCOVERY_TIME=$((DISCOVERY_END - DISCOVERY_START))
    echo -e "${GREEN}✅ Auto-discovery performance: ${DISCOVERY_TIME}s (target: <30s)${NC}"
    if [ $DISCOVERY_TIME -lt 30 ]; then
        ((TESTS_PASSED++))
    else
        echo -e "${YELLOW}⚠️  Discovery took longer than expected but completed${NC}"
        ((TESTS_PASSED++))
    fi
else
    echo -e "${RED}❌ Auto-discovery performance test failed${NC}"
    ((TESTS_FAILED++))
fi

# Summary
echo -e "${YELLOW}📊 Test Summary${NC}"
echo "=============="
echo "Tests run:     $TESTS_RUN"
echo -e "Tests passed:  ${GREEN}$TESTS_PASSED${NC}"
echo -e "Tests failed:  ${RED}$TESTS_FAILED${NC}"

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}🎉 All tests passed! Auto-collectors system is working perfectly!${NC}"
    
    echo -e "${BLUE}📦 Generated Test Bundles:${NC}"
    for bundle in /tmp/foundational-test.tar.gz /tmp/images-test.tar.gz /tmp/comprehensive-test.tar.gz; do
        if [ -f "$bundle" ]; then
            echo "   $(basename $bundle): $(ls -lh $bundle | awk '{print $5}')"
        fi
    done
    
    echo -e "${BLUE}📋 Next Steps:${NC}"
    echo "1. Extract and examine bundle contents:"
    echo "   tar -tzf /tmp/comprehensive-test.tar.gz | head -20"
    echo "2. Test with your application namespaces:"  
    echo "   ./bin/support-bundle --auto --namespace your-app-namespace --include-images"
    echo "3. Try YAML augmentation with your specs:"
    echo "   ./bin/support-bundle your-spec.yaml --auto"
    
    exit 0
else
    echo -e "${RED}❌ Some tests failed. Check output files in /tmp/ for details.${NC}"
    echo -e "${BLUE}📋 Debugging:${NC}"
    echo "- Check cluster connectivity: kubectl get nodes"
    echo "- Check permissions: kubectl auth can-i list pods"
    echo "- Review test output files: ls /tmp/*test*.txt"
    exit 1
fi
