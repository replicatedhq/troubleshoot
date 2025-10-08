# Regression Test Baselines

This directory contains known-good baseline bundles used for regression testing.

## Directory Structure

- `preflight-v1beta3/` - Baseline for complex-v1beta3.yaml spec
- `preflight-v1beta2/` - Baseline for all-analyzers-v1beta2.yaml spec
- `supportbundle/` - Baseline for all-kubernetes-collectors.yaml spec
- `metadata.json` - Metadata about when baselines were last updated

## Creating Initial Baselines

If this directory is empty, baselines need to be created:

1. Run the regression test workflow manually
2. Download the artifacts
3. Inspect bundles to verify correctness
4. Use `scripts/update_baselines.sh` to copy them here

See `test/README.md` for detailed instructions.

## Updating Baselines

Baselines should only be updated when:
- New collectors are added
- Collector output format changes intentionally
- Kubernetes version is upgraded
- Bug fixes that change collector behavior

**Never update baselines to make failing tests pass without investigation!**

Use `scripts/update_baselines.sh` to update from a workflow run.
