#!/bin/bash
set -e

# Helper script to update regression test baselines
# Usage: ./scripts/update_baselines.sh [run-id]

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}===========================================${NC}"
echo -e "${BLUE}Regression Test Baseline Update Script${NC}"
echo -e "${BLUE}===========================================${NC}\n"

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    echo -e "${RED}Error: GitHub CLI (gh) not found${NC}"
    echo "Install from: https://cli.github.com/"
    exit 1
fi

# Get run ID from argument or prompt
if [ -n "$1" ]; then
    RUN_ID="$1"
else
    echo -e "${YELLOW}Enter GitHub Actions run ID (or leave empty for latest):${NC}"
    read -r RUN_ID
fi

# If no run ID provided, get the latest regression-test workflow run
if [ -z "$RUN_ID" ]; then
    echo "Fetching latest regression-test workflow run..."
    RUN_ID=$(gh run list --workflow=regression-test.yaml --limit 1 --json databaseId --jq '.[0].databaseId')

    if [ -z "$RUN_ID" ]; then
        echo -e "${RED}Error: No workflow runs found${NC}"
        exit 1
    fi

    echo -e "Using latest run: ${GREEN}${RUN_ID}${NC}"
fi

# Create temp directory
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

echo -e "\n${BLUE}Step 1: Downloading artifacts...${NC}"

# Download artifacts from the run
if ! gh run download "$RUN_ID" --name "regression-test-results-${RUN_ID}-1" --dir "$TEMP_DIR" 2>/dev/null; then
    # Try without attempt suffix
    if ! gh run download "$RUN_ID" --dir "$TEMP_DIR" 2>/dev/null; then
        echo -e "${RED}Error: Failed to download artifacts from run ${RUN_ID}${NC}"
        gh run download "$RUN_ID" --dir "$TEMP_DIR"
        exit 1
    fi
fi

echo -e "${GREEN}✓ Artifacts downloaded${NC}"

# Check which bundles are present
echo -e "\n${BLUE}Step 2: Checking available bundles...${NC}"

V1BETA3_BUNDLE=""
V1BETA2_BUNDLE=""
SUPPORTBUNDLE=""

if [ -f "$TEMP_DIR/preflight-v1beta3-bundle.tar.gz" ] || [ -f "$TEMP_DIR/test/output/preflight-v1beta3-bundle.tar.gz" ]; then
    V1BETA3_BUNDLE=$(find "$TEMP_DIR" -name "preflight-v1beta3-bundle.tar.gz" | head -1)
    echo -e "${GREEN}✓${NC} Found v1beta3 preflight bundle"
fi

if [ -f "$TEMP_DIR/preflight-v1beta2-bundle.tar.gz" ] || [ -f "$TEMP_DIR/test/output/preflight-v1beta2-bundle.tar.gz" ]; then
    V1BETA2_BUNDLE=$(find "$TEMP_DIR" -name "preflight-v1beta2-bundle.tar.gz" | head -1)
    echo -e "${GREEN}✓${NC} Found v1beta2 preflight bundle"
fi

if [ -f "$TEMP_DIR/supportbundle.tar.gz" ] || [ -f "$TEMP_DIR/test/output/supportbundle.tar.gz" ]; then
    SUPPORTBUNDLE=$(find "$TEMP_DIR" -name "supportbundle.tar.gz" | head -1)
    echo -e "${GREEN}✓${NC} Found support bundle"
fi

if [ -z "$V1BETA3_BUNDLE" ] && [ -z "$V1BETA2_BUNDLE" ] && [ -z "$SUPPORTBUNDLE" ]; then
    echo -e "${RED}Error: No bundles found in artifacts${NC}"
    exit 1
fi

# Confirm update
echo -e "\n${YELLOW}This will update the following baselines:${NC}"
[ -n "$V1BETA3_BUNDLE" ] && echo "  - test/baselines/preflight-v1beta3/baseline.tar.gz"
[ -n "$V1BETA2_BUNDLE" ] && echo "  - test/baselines/preflight-v1beta2/baseline.tar.gz"
[ -n "$SUPPORTBUNDLE" ] && echo "  - test/baselines/supportbundle/baseline.tar.gz"

echo -e "\n${YELLOW}Continue? (y/N):${NC} "
read -r CONFIRM

if [ "$CONFIRM" != "y" ] && [ "$CONFIRM" != "Y" ]; then
    echo "Aborted."
    exit 0
fi

echo -e "\n${BLUE}Step 3: Updating baselines...${NC}"

# Update v1beta3 baseline
if [ -n "$V1BETA3_BUNDLE" ]; then
    mkdir -p test/baselines/preflight-v1beta3
    cp "$V1BETA3_BUNDLE" test/baselines/preflight-v1beta3/baseline.tar.gz
    echo -e "${GREEN}✓${NC} Updated preflight-v1beta3 baseline"
fi

# Update v1beta2 baseline
if [ -n "$V1BETA2_BUNDLE" ]; then
    mkdir -p test/baselines/preflight-v1beta2
    cp "$V1BETA2_BUNDLE" test/baselines/preflight-v1beta2/baseline.tar.gz
    echo -e "${GREEN}✓${NC} Updated preflight-v1beta2 baseline"
fi

# Update support bundle baseline
if [ -n "$SUPPORTBUNDLE" ]; then
    mkdir -p test/baselines/supportbundle
    cp "$SUPPORTBUNDLE" test/baselines/supportbundle/baseline.tar.gz
    echo -e "${GREEN}✓${NC} Updated supportbundle baseline"
fi

# Create metadata file
echo -e "\n${BLUE}Step 4: Creating metadata...${NC}"

GIT_SHA=$(git rev-parse HEAD)
CURRENT_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)

cat > test/baselines/metadata.json <<EOF
{
  "updated_at": "$CURRENT_DATE",
  "git_sha": "$GIT_SHA",
  "workflow_run_id": "$RUN_ID",
  "k8s_version": "v1.28.3",
  "updated_by": "$(git config user.name) <$(git config user.email)>"
}
EOF

echo -e "${GREEN}✓${NC} Created metadata.json"

# Show git status
echo -e "\n${BLUE}Step 5: Git status${NC}"
git status test/baselines/

echo -e "\n${YELLOW}Review the changes above. To commit:${NC}"
echo -e "  ${BLUE}git add test/baselines/${NC}"
echo -e "  ${BLUE}git commit -m 'chore: update regression baselines from run ${RUN_ID}'${NC}"
echo -e "  ${BLUE}git push${NC}"

echo -e "\n${GREEN}Done!${NC}"
