# Code Review Guidelines

## Basic Review

- **Breaking API changes** — Check for removed/renamed fields in `pkg/apis/`, changed function signatures in public packages, or modified CLI flags and output formats.
- **Pattern violations** — New code should follow existing patterns in the codebase (e.g., collector/analyzer structure, error handling conventions, interface usage).
- **Security** — Watch for command injection in exec-based collectors, path traversal in file operations, unsanitized user input in specs, and leaked credentials in collected data.
- **Go standards** — Issues that linters like `go vet`, `staticcheck`, and `modernize` would catch: deprecated API usage, unnecessary allocations, error shadowing, unchecked errors.
- **Test coverage** — New functionality should have tests. Changes to existing code should not reduce coverage compared to the last test run on `main`.
- **Error handling** — Errors should wrap context (`fmt.Errorf("... : %w", err)`), not be silently swallowed, and provide actionable messages for operator-facing output.
- **Concurrency safety** — Collectors run concurrently. Shared state must be protected. `CollectorResult` map writes from goroutines need synchronization.
- **Bundle storage** — Collectors must save data using `CollectorResult.SaveResult` and related methods (`SaveResults`, `SymLinkResult`). Never write files directly — `CollectorResult` handles dual-mode storage (in-memory for preflights, on-disk for support bundles). See `pkg/collect/result.go`.

## Advanced Review

- **Cross-feature impact** — Consider whether a change to one collector/analyzer could affect the broader collection pipeline, redaction, or output archive structure.
- **CLI vs SDK consumers** — This project is consumed both as CLI tools and as Go packages (SDK). Changes targeting a CLI use case must not break SDK consumers who import `pkg/collect`, `pkg/analyze`, or API types directly.
- **Documentation** — Does the change add, modify, or remove user-facing behavior? Check whether https://troubleshoot.sh needs updates. Use https://troubleshoot.sh/llms.txt or https://troubleshoot.sh/llms-full.txt to review current docs.
- **Dedicated documentation needs** — For large or complex changes, consider whether CLI users or SDK consumers need standalone documentation (migration guides, new feature walkthroughs, updated examples).
- **Backwards compatibility** — Spec changes must consider existing specs in the wild. New fields should have sensible zero-value defaults. Removed fields should not cause parse failures.
- **Downstream impact on sbctl** — Changes to public Go packages (`pkg/collect`, `pkg/analyze`, API types, etc.) may require follow-up changes in [replicatedhq/sbctl](https://github.com/replicatedhq/sbctl), which imports this project as a dependency. Flag any breaking or behavioral changes that could affect sbctl.
- **Repository docs** — If your changes affect build commands, architecture, project conventions, or review guidelines, update `CLAUDE.md`, `README.md`, and `REVIEW.md` accordingly.
