# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Replicated Troubleshoot is a Kubernetes diagnostic framework providing two kubectl plugins: `preflight` (pre-installation cluster validation) and `support-bundle` (post-installation diagnostics with log collection, redaction, and analysis). Specs use the Kubernetes custom resource format (as a serialization convention, not installed in-cluster) and are defined by application vendors and executed by cluster operators.

## Build & Test Commands

```bash
make build                        # Build bin/support-bundle and bin/preflight
make test                         # Unit tests (includes generate, fmt, vet)
make test RUN=TestMyFunction      # Run a single test
make test-integration             # Integration tests (requires k8s cluster)
make e2e                          # All e2e tests
make generate                     # Regenerate types/clients after modifying pkg/apis/
```

## Architecture

The core data flow is: **Spec loading → Collection → Redaction → Analysis → Results**. The two main workflows are orchestrated by `pkg/supportbundle/` and `pkg/preflight/`.

Three API versions coexist in `pkg/apis/troubleshoot/`: v1beta1, v1beta2 (primary, all types defined here), and v1beta3 (in-progress, adds `StringOrValueFrom` for Secret/ConfigMap references, converts to v1beta2 at runtime).

Collectors live in `pkg/collect/`, analyzers in `pkg/analyze/`. When adding either, follow the pattern of existing implementations and run `make generate` after modifying API types.

## Code Review

See [.cursor/BUGBOT.md](.cursor/BUGBOT.md) for the full review checklist covering basic checks (API breaks, pattern violations, security, test coverage) and advanced checks (cross-feature impact, CLI vs SDK consumers, documentation needs).
