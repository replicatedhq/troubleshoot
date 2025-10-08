# Regression Test Suite

This directory contains the regression test infrastructure for validating preflight and support bundle collectors.

## Overview

The regression test suite:
1. Provisions an ephemeral k3s cluster via Replicated Actions
2. Runs multiple preflight and support bundle specs
3. Compares output bundles against known-good baselines
4. Reports regressions (missing files, changed outputs)

## Directory Structure

```
test/
├── README.md                    # This file
├── baselines/                   # Known-good baseline bundles
│   ├── preflight-v1beta3/
│   │   └── baseline.tar.gz
│   ├── preflight-v1beta2/
│   │   └── baseline.tar.gz
│   ├── supportbundle/
│   │   └── baseline.tar.gz
│   └── metadata.json            # Baseline metadata (git sha, date, k8s version)
└── output/                      # Test run outputs (gitignored)
    ├── preflight-v1beta3-bundle.tar.gz
    ├── preflight-v1beta2-bundle.tar.gz
    ├── supportbundle.tar.gz
    └── diff-report-*.json
```

## Specs Under Test

| Spec | File | Values | Description |
|------|------|--------|-------------|
| Preflight v1beta3 | `examples/preflight/complex-v1beta3.yaml` | `examples/preflight/values-complex-full.yaml` | Templated v1beta3 with ~30 analyzers |
| Preflight v1beta2 | `examples/preflight/all-analyzers-v1beta2.yaml` | N/A | Legacy v1beta2 format with all analyzer types |
| Support Bundle | `examples/collect/host/all-kubernetes-collectors.yaml` | N/A | Comprehensive collector suite |

## Running Tests

### Via GitHub Actions (Recommended)

The regression test workflow runs automatically on:
- Push to `main` or `v1beta3` branches
- Pull requests
- Manual trigger via workflow_dispatch

**Manual trigger:**
```bash
gh workflow run regression-test.yaml
```

### Locally (Manual)

```bash
# 1. Build binaries
make bin/preflight bin/support-bundle

# 2. Create k3s cluster (use your preferred method)
k3d cluster create test-cluster --wait

# 3. Run specs
./bin/preflight examples/preflight/complex-v1beta3.yaml \
  --values examples/preflight/values-complex-full.yaml \
  --interactive=false

./bin/preflight examples/preflight/all-analyzers-v1beta2.yaml \
  --interactive=false

./bin/support-bundle examples/collect/host/all-kubernetes-collectors.yaml \
  --interactive=false

# 4. Compare bundles (if baselines exist)
python3 scripts/compare_bundles.py \
  --baseline test/baselines/preflight-v1beta3/baseline.tar.gz \
  --current preflightbundle-*.tar.gz \
  --rules scripts/compare_rules.yaml \
  --report test/output/diff-report.json \
  --spec-type preflight

# 5. Clean up
k3d cluster delete test-cluster
```

## Creating Initial Baselines

If baselines don't exist yet (first time setup):

1. **Run workflow to generate bundles:**
   ```bash
   gh workflow run regression-test.yaml
   ```

2. **Download artifacts:**
   ```bash
   gh run download <run-id> --name regression-test-results-<run-id>-1
   ```

3. **Inspect bundles manually:**
   ```bash
   tar -tzf preflight-v1beta3-bundle.tar.gz | head -20
   tar -xzf preflight-v1beta3-bundle.tar.gz
   # Verify contents look correct
   ```

4. **Copy as baselines and commit:**
   ```bash
   mkdir -p test/baselines/{preflight-v1beta3,preflight-v1beta2,supportbundle}

   cp preflight-v1beta3-bundle.tar.gz test/baselines/preflight-v1beta3/baseline.tar.gz
   cp preflight-v1beta2-bundle.tar.gz test/baselines/preflight-v1beta2/baseline.tar.gz
   cp supportbundle.tar.gz test/baselines/supportbundle/baseline.tar.gz

   git add test/baselines/
   git commit -m "chore: add initial regression test baselines"
   git push
   ```

## Updating Baselines

When legitimate changes occur (new collectors, changed output format):

### Option 1: Automatic Update (Workflow Input)

```bash
gh workflow run regression-test.yaml -f update_baselines=true
```

This will:
1. Run tests
2. Copy new bundles as baselines
3. Commit and push updated baselines

** Use with caution!** Only use this after verifying changes are intentional.

### Option 2: Manual Update

```bash
# Download artifacts from a successful run
gh run download <run-id> --name regression-test-results-<run-id>-1

# Replace baselines
cp preflight-v1beta3-bundle.tar.gz test/baselines/preflight-v1beta3/baseline.tar.gz
cp preflight-v1beta2-bundle.tar.gz test/baselines/preflight-v1beta2/baseline.tar.gz
cp supportbundle.tar.gz test/baselines/supportbundle/baseline.tar.gz

# Commit
git add test/baselines/
git commit -m "chore: update regression baselines - reason for change"
git push
```

## Comparison Strategy

The comparison uses a 3-tier approach:

### 1. Exact Match (2 files)
Files compared byte-for-byte:
- `static-data.txt/static-data` - static data collector
- `version.yaml` - spec version
- Data collector files (`files/example.yaml`, `config/replicas.txt`)

### 2. Structural Comparison (8 files)
Compare specific fields only, ignore variable values:
- **Database collectors** (`postgres/*.json`, `mysql/*.json`, etc.) - Compare `isConnected` boolean
- **DNS** (`dns/debug.json`) - Verify service exists, queries succeed
- **Registry** (`registry/*.json`) - Compare `exists` per image
- **HTTP** (`http*.json`) - Compare status code only

### 3. Non-Empty Check (Everything Else)
For highly variable outputs:
- **cluster-resources** - UIDs, timestamps, resourceVersions vary
- **node-metrics** - All metric values constantly change
- **logs** - Timestamps in every line
- **run/exec collectors** - Random pod names, variable output
- And more...

Strategy: Verify file exists, is non-empty, and (for JSON) is valid JSON.

## Understanding Test Results

### Passing Test
- All expected files present
- Exact match files identical
- Structural comparison fields match
- All files non-empty and valid

### Failing Test - Regressions Detected

**Files missing:**
```
⚠ Missing in current: postgres/postgres-example.json
```
→ Collector stopped producing output (regression)

**Structural mismatch:**
```
❌ postgres/postgres-example.json: database connection status changed: true -> false
```
→ Collector behavior changed (potential regression)

**Empty file:**
```
❌ dns/debug.json: File is empty
```
→ Collector failed to collect data (regression)

### ℹ️ New Files (Not a Failure)
```
ℹ New file in current: newcollector/output.json
```
→ New collector added (expected when adding features)

## Troubleshooting

### Workflow fails: "No baseline found"
First time setup - baselines need to be created (see above).

### Many "structural mismatch" failures
Check if cluster state changed:
- Different k8s version?
- Different installed components?
- Resources created/deleted?

### Comparison fails with Python error
Ensure dependencies installed:
```bash
pip install pyyaml deepdiff
```

### Cluster creation times out
Check Replicated Actions limits:
```bash
# View cluster status
gh api /repos/replicatedhq/compatibility-actions/...
```

## Configuration Files

### `scripts/compare_rules.yaml`
Defines comparison strategy per file pattern.

**Add new rule:**
```yaml
preflight:
  structural_compare:
    "mycollector/*.json": "my_comparator_function"
```

Then implement `_compare_my_comparator_function()` in `scripts/compare_bundles.py`.

### `scripts/compare_bundles.py`
Comparison engine - implements comparison logic.

**Add new comparator:**
```python
def _compare_my_comparator_function(self, baseline: Dict, current: Dict) -> bool:
    """Compare mycollector output."""
    # Your comparison logic
    return baseline["field"] == current["field"]
```

### `.github/workflows/regression-test.yaml`
GitHub Actions workflow definition.

## Tips

- **Start simple**: Begin with baselines for v1beta2 only, add v1beta3 later
- **Iterate on rules**: Add structural comparisons as you discover false positives
- **Review diffs**: Always inspect diff reports before updating baselines
- **Document changes**: In baseline update commits, explain why output changed
- **Monitor runtime**: Workflow should complete in < 20 minutes

## Related Documentation

- [CI Regression Test Proposal](../ci-regression-test-proposal.md)
- [Collector Comparison Strategy](../collector-comparison-strategy.md)
- [Replicated Actions Docs](https://github.com/replicatedhq/replicated-actions)
