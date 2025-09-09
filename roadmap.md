### Cross-cutting engineering charters

1) Contract/stability policy (one pager, checked into repo)
	•	SemVer & windows: major.minor.patch; flags/commands stable for ≥2 minors; deprecations carry --explain-deprecations.
	•	Breaking-change gate: PR must include contracts/CHANGE_PROPOSAL.md + updated goldens + migration notes.
	•	Determinism: Same inputs ⇒ byte-identical outputs (normalized map ordering, sorted slices, stable timestamps with SOURCE_DATE_EPOCH).

2) Observability & diagnostics
	•	Structured logs (zerolog/zap): --log-format {text,json}, --log-level {info,debug,trace}.
	•	Exit code taxonomy: 0 ok, 1 generic, 2 usage, 3 network, 4 schema, 5 incompatible-api, 6 update-failed, 7 permission, 8 partial-success.
	•	OTel hooks (behind TROUBLESHOOT_OTEL_ENDPOINT): span “loadSpec”, “mergeSpec”, “runPreflight”, “uploadPortal”.

3) Reproducible, signed, attestable releases
	•	SBOM (cyclonedx/spdx) emitted by GoReleaser.
	•	cosign: sign archives + checksums.txt; produce SLSA provenance attestation.
	•	SOURCE_DATE_EPOCH set in CI to pin archive mtimes.

CLI contracts & packaging (more depth)

A) Machine-readable CLI spec
	•	Generate docs/cli-contracts.json from Cobra tree (name, synopsis, flags, defaults, env aliases, deprecation).
	•	Validate at runtime when TROUBLESHOOT_DEBUG_CONTRACT=1 to catch drift in dev builds.
	•	Use that JSON to:
	•	Autogenerate shell completions for bash/zsh/fish/pwsh.
	•	Render the --help text (single source of truth).

B) UX hardening
	•	TTY detection: progress bars only on TTY; --no-progress to force off.
	•	Color policy: --color {auto,always,never} + NO_COLOR env respected.
	•	Output mode: --output {human,json,yaml} for all read commands. For json, include a top-level "schemaVersion": "cli.v1".

C) Update system (secure + rollback)
	•	Channel support: --channel {stable,rc,nightly} (maps to tags: vX.Y.Z, vX.Y.Z-rc.N, nightly-YYYYMMDD).
	•	Rollback: keep N=2 previous binaries under ~/.troubleshoot/bin/versions/…; preflight update --rollback.
	•	Tamper defense: verify cosign sig for checksums.txt; verify SHA256 of selected asset; fail closed with error code 6.
	•	Delta updates (optional later): if asset .patch exists and base version matches, apply bsdiff; fallback to full.

D) Packaging matrix validation (CI)
	•	Matrix test on ubuntu-latest, macos-latest, windows-latest:
	•	Install via brew, scoop, deb/rpm, curl|bash; then run preflight --version and a sample command.
	•	Gatekeeper: spctl -a -v on macOS; print notarization ticket.

E) Config precedence & env aliases
	•	Per-binary config paths (defaults):
	•	macOS/Linux:
	•	preflight: ~/.config/preflight/config.yaml
	•	support-bundle: ~/.config/support-bundle/config.yaml
	•	Windows:
	•	preflight: %APPDATA%\Troubleshoot\Preflight\config.yaml
	•	support-bundle: %APPDATA%\Troubleshoot\SupportBundle\config.yaml
	•	Optional global fallback (lower precedence): ~/.config/troubleshoot/config.yaml
	•	Precedence: flag > binary env > global env > binary config > global config > default
	•	--config <path> overrides discovery; respects XDG_CONFIG_HOME (Unix) and APPDATA (Windows)
	•	Env aliases:
	•	Global: TROUBLESHOOT_PORTAL_URL, TROUBLESHOOT_API_TOKEN
	•	Binary-scoped: PREFLIGHT_* and SUPPORT_BUNDLE_* (take precedence over TROUBLESHOOT_*)

F) Make targets

make contracts          # regen CLI JSON + goldens
make sbom               # build SBOMs
make release-dryrun     # goreleaser --skip-publish
make e2e-install        # spins a container farm to test deb/rpm


API v1beta3 & schema work (deeper)

A) JSON Schema strategy
	•	Give every schema an $id and $schema; publish at schemas.troubleshoot.sh/v1beta3/*.json.
	•	Use $defs for shared primitives (Quantity, Duration, CPUSet, Selector).
	•	Add x-kubernetes-validations parity constraints where applicable (even if not applying as CRD).

B) Defaulting & validation library
	•	pkg/validation/validate.go: returns []FieldError with JSONPointer paths and machine codes.
	•	pkg/defaults/defaults.go: idempotent defaulting; fuzz tests prove no oscillation (fuzz: in -> default -> default == default).

C) Converters robustness
	•	Fuzzers (go1.20+): generate random v1beta1/2 structs, convert→internal→v1beta3→internal and assert invariants (lossless roundtrips where representable).
	•	Report downgrade loss: if v1beta3→v1beta2 drops info, print warning list to stderr and annotate output with x-downgrade-warnings.

D) Performance budget
	•	Load+validate 1MB spec ≤ 150ms p95, 10MB ≤ 800ms p95 on GOARCH=amd64 GitHub runner.
	•	Benchmarks in pkg/apis/bench_test.go enforce budgets.

Preflight docs & portal flow (hardening)

A) Merge engine details
	•	Stable key = GroupKind/Name[/Namespace] (e.g., NodeResource/CPU, FilePermission//etc/hosts).
	•	Conflict detection emits a list with reasons: “same key, differing fields: thresholds.min, description”.
	•	Provenance captured on each merged node:
	•	troubleshoot.sh/provenance: vendor|replicated|merged
	•	troubleshoot.sh/merge-conflict: "thresholds.min, description"

B) Docs generator upgrades
	•	Template slots: why, riskLevel {low,med,high}, owner, runbookURL, estimatedTime.
	•	i18n hooks: template lookup by locale --locale es-ES falls back to en-US.
	•	Output MD + self-contained HTML (inline CSS) when --html. --toc adds a nav sidebar.

C) Portal client contract
	•	Auth: Bearer <token>; optional mTLS later.
	•	Idempotency: Idempotency-Key header derived from spec SHA256.
	•	Backoff: exponential jitter (100ms → 3s, 6 tries) on 429/5xx; code 3 on exhaustion.
	•	Response model:

{
  "requestId": "r_abc123",
  "decision": "pass|override|fail",
  "reason": "text",
  "policyVersion": "2025-09-01"
}

	•	CLI prints requestId on error for support.

D) E2E tests (httptest.Server)
	•	Scenarios: pass, fail, override, 429 with retry-after, 5xx flake, invalid JSON.
	•	Golden transcripts of HTTP exchanges under testdata/e2e/portal.


Public packages & ecosystem

A) Package boundaries

pkg/
  cli/contract       # cobra->json exporter (no cobra import cycles)
  update/            # channel, verify, rollback
  schema/            # embed.FS of JSON Schemas + helpers
  specs/loader       # version sniffing, load any -> internal
  specs/convert      # converters
  specs/validate     # validation library
  docs/render        # md/html generation
  portal/client      # http client + types

	•	No logging in libs; return structured errors with codes; callers log.

B) SARIF export (nice-to-have)
	•	--output sarif for preflight results so CI systems ingest findings.

C) Back-compat façade
	•	For integrators, add tiny shim: pkg/legacy/v1beta2loader that calls new loader + converter; mark with Deprecated: GoDoc but stable for a window.

CI/CD reinforcement

Pipelines
	1.	verify: lint, unit, fuzz (short), contracts, schemas → required.
	2.	matrix-install: brew/scoop/deb/rpm/curl on 3 OSes.
	3.	bench: enforce perf budgets.
	4.	supply-chain: build SBOM, cosign sign/verify, slsa attestation.
	5.	release (tagged): goreleaser publish, notarize, bump brew/scoop, attach SBOM, cosign attest.

Static checks
	•	revive/golangci-lint with a rule to forbid time.Now() in pure functions; must use injected clock.
	•	api-diff: compare exported pkg/** against last tag; fails on breaking changes without contracts/CHANGE_PROPOSAL.md.

1) Error codes (centralized)

package xerr
type Code int
const (
  OK Code = iota
  Usage
  Network
  Schema
  IncompatibleAPI
  UpdateFailed
  Permission
  Partial
)
type E struct { Code Code; Op, Msg string; Err error }
func (e *E) Error() string { return e.Msg }
func CodeOf(err error) Code { /* unwrap */ }

2) Output envelope (JSON mode)

{
  "schemaVersion": "cli.v1",
  "tool": "preflight",
  "version": "1.12.0",
  "timestamp": "2025-09-09T17:02:33Z",
  "result": { /* command-specific */ },
  "warnings": [],
  "errors": []
}

3) Idempotency key

func idemKey(spec []byte) string {
  sum := sha256.Sum256(spec)
  return hex.EncodeToString(sum[:])
}

4) Deterministic marshaling

enc := json.NewEncoder(w)
enc.SetEscapeHTML(false)
enc.SetIndent("", "  ")
sort.SliceStable(obj.Items, func(i,j int) bool { return obj.Items[i].Name < obj.Items[j].Name })

Measurable add-on success criteria
	•	preflight --help --output json validates against docs/cli-contracts.schema.json.
	•	make bench passes with stated p95 budgets.
	•	cosign verify-blob succeeds for checksums.txt in CI and on dev machines (doc’d).
	•	E2E portal tests cover all decision branches and 429/5xx paths with retries observed.
	•	api-diff is green or has an attached change proposal.

Testing strategy (Dev 1 scope)

	Unit tests
	•	CLI arg parsing: Cobra ExecuteC with table-driven flag sets for both binaries.
	•	Config precedence resolver: tmp dirs + OS-specific cases (XDG_CONFIG_HOME/APPDATA).
	•	Validation/defaulting libraries: happy/edge cases; structured []FieldError assertions.
	•	Portal client: httptest.Server scenarios (pass/fail/override/429/5xx) with retry/backoff checks.
	•	Updater: mock release index; cosign verify using test keys; rollback success/failure paths.

	Contract/golden tests
	•	CLI contracts: generate docs/cli-contracts.json and compare to goldens; update via make contracts.
	•	--help rendering snapshots (normalized width/colors) for core commands.
	•	Schemas: validate example specs against v1beta3 JSON Schemas; store fixtures in testdata/schemas/.
	•	Docs generator: preflight-docs.md/HTML goldens for sample merged specs with provenance.

	Fuzz/property tests
	•	Converters: v1beta1/2→internal→v1beta3→internal round-trip fuzz; invariants enforced.
	•	Defaulting idempotence: default(default(x)) == default(x).

	Integration/matrix tests
	•	Installers: brew/scoop/deb/rpm/curl on ubuntu/macos/windows; run preflight/support-bundle --version and a smoke command.
	•	macOS notarization: spctl -a -v on built binaries.
	•	Updater E2E: start mock release server, switch channels, rollback, tamper-detection failure.

	Determinism & performance
	•	Deterministic outputs under SOURCE_DATE_EPOCH; byte-for-byte stable archives in a test harness.
	•	Benchmarks: load+validate budgets (latency + RSS) enforced via go test -bench and thresholds.

	Artifacts & layout
	•	Fixtures under testdata/: schemas/, cli/, docs/, portal/, updater/ with README explaining regeneration.
	•	Make targets: make test, make fuzz-short, make contracts, make e2e-install, make bench.