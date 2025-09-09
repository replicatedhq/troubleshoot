### Phased execution plan (actionable)

1) Foundation & policy (cross-cutting)
	• Goal: Establish non-negotiable engineering charters, error taxonomy, deterministic I/O, and output envelope.
	• Do:
		• Adopt items under “Cross-cutting engineering charters”.
		• Implement centralized error codes (see “1) Error codes (centralized)”).
		• Implement JSON output envelope (see “2) Output envelope (JSON mode)”).
		• Add idempotency key helper (see “3) Idempotency key”).
		• Ensure deterministic marshaling patterns (see “4) Deterministic marshaling”).
		• Define config precedence and env aliases (see section E) Config precedence & env aliases).
		• Add Make targets (see section F) Make targets).
	• Acceptance:
		• “Measurable add-on success criteria” items related to CLI output and determinism are satisfied.

2) Distribution & updates (installers, signing, updater)
	• Goal: Stop krew; ship Homebrew and curl|bash installers; add secure update with rollback.
	• Do:
		• Remove/retire krew guidance; add Homebrew formulas and curl|bash script(s).
		• Implement “C) Update system (secure + rollback)” including channels, rollback, tamper defense, delta updates (optional later).
		• Implement “Reproducible, signed, attestable releases” (SBOM, cosign, SLSA, SOURCE_DATE_EPOCH).
		• Add minimal packaging matrix validation for brew and curl|bash; expand later (see D) Packaging matrix validation (CI)).
	• Acceptance:
		• Users can install preflight and support-bundle via brew and curl|bash.
		• Updater supports --channel, verify, rollback; signatures verified per roadmap details.

3) API v1beta3 schemas and libraries
	• Goal: Define and own v1beta3 JSON Schemas and supporting defaulting/validation/conversion libraries within performance budgets.
	• Do:
		• Implement “API v1beta3 & schema work (deeper)” sections A–D (JSON Schema strategy; defaulting; validation; performance budget).
		• Add converters and fuzzers per “C) Converters robustness”.
		• Benchmarks per “D) Performance budget”.
	• Acceptance:
		• Schemas published under schemas.troubleshoot.sh/v1beta3/* with $id, $schema, $defs.
		• Validation/defaulting return structured errors; fuzz and perf budgets pass.

4) Preflight requirements disclosure command
	• Goal: Let customers preview requirements offline; render table/json/yaml/md; support templating values.
	• Do:
		• Implement “Preflight requirements disclosure (new command)” (`preflight requirements`), including flags and behaviors.
		• Implement templating from “Preflight CLI: Values and --set support (templating)”.
	• Acceptance:
		• Output validates against docs/preflight-requirements.schema.json and renders within width targets.
		• Unit and golden tests for table/json/md; fuzz tests for extractor stability.

5) Docs generator and portal gate/override
	• Goal: Generate preflight docs with rationale and support portal gate/override flow.
	• Do:
		• Implement “Preflight docs & portal flow (hardening)” sections A–D (merge engine, docs generator, portal client contract, E2E tests).
		• Ensure CLI prints requestId on error; implement backoff/idempotency per contract.
	• Acceptance:
		• E2E portal tests cover pass/fail/override/429/5xx with retries.
		• Docs generator emits MD/HTML with i18n hooks and template slots.

6) Simplified spec model: intents, presets, imports
	• Goal: Reduce authoring burden via intents for collect/analyze, redaction profiles with tokenize, and preset/import model.
	• Do:
		• Implement “Simplified spec model: intents, presets, imports”: intents.collect.auto; intents.analyze.requirements; redact.profile + tokenize; import/extends; selectors/filters; compatibility flags `--emit` and `--explain`.
		• Provide examples and downgrade warnings for v1beta2 emit.
	• Acceptance:
		• Deterministic expansion demonstrated; explain output shows generated low-level spec; downgrade warnings reported where applicable.

7) Public packages & ecosystem factoring
	• Goal: Establish stable package boundaries to support reuse and avoid logging in libs.
	• Do:
		• Create packages listed under “Public packages & ecosystem” (pkg/cli/contract, update, schema, specs/*, docs/render, portal/client). 
		• Export minimal, stable APIs; return structured errors.
	• Acceptance:
		• api-diff green or change proposal attached.

8) CI/CD reinforcement
	• Goal: End-to-end pipelines for verification, install matrix, benchmarks, supply-chain, and releases.
	• Do:
		• Implement pipeline stages listed under “CI/CD reinforcement → Pipelines 1–5”.
		• Add static checks (revive/golangci-lint, api-diff rules) per roadmap.
	• Acceptance:
		• Pipelines green; supply chain artifacts (SBOM, cosign, SLSA) produced; release flow notarizes and publishes.

9) Testing strategy, determinism and performance harness, artifacts layout
	• Goal: Comprehensive unit/contract/fuzz/integration tests, deterministic outputs, and curated fixtures.
	• Do:
		• Implement “Testing strategy (Dev 1 scope)” (unit, contract/golden, fuzz/property, integration/matrix tests).
		• Implement “Determinism & performance” harness and budgets.
		• Organize artifacts per “Artifacts & layout” and add Make targets for test/fuzz/contracts/e2e/bench.
	• Acceptance:
		• Golden tests stable; determinism harness passes under SOURCE_DATE_EPOCH; benchmarks within budgets.

10) Packaging matrix expansion (optional later)
	• Goal: Expand beyond brew/curl to scoop and deb/rpm when desired.
	• Do:
		• Extend “D) Packaging matrix validation (CI)” to include scoop and deb/rpm installers and tests across OSes.
	• Acceptance:
		• Installers validated on ubuntu/macos/windows with smoke commands; macOS notarization verified.

Notes
	• Each phase references detailed specifications below. Implement phases in order; parallelize sub-items where safe.
	• If scope for an initial milestone is narrower (e.g., brew/curl only), mark the remaining items as deferred but keep tests/docs ready to expand.

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

E) Simplified spec model: intents, presets, imports
	•	Problem: vendors handwrite verbose collector/analyzer lists. Goal: smaller, intent-driven specs that expand deterministically.
	•	Tenets:
		•	Additive, backwards-compatible; loader can expand intents into concrete v1beta2-equivalent structures.
		•	Deterministic expansion (same inputs ⇒ same expansion) with --explain to show the generated low-level spec.
		•	Shorthand over raw lists: “what” not “how”.
	•	Top-level additions (v1beta3):
		•	intents.collect.auto: namespace, profiles, includeKinds, excludeKinds, selectors, size caps.
		•	intents.analyze.requirements: high-level checks (k8sVersion, nodes.cpu/memory, podsReady, storageClass, CRDsPresent…).
		•	redact.profile + tokenize: standard|strict; optional token map emission.
		•	import: versioned presets (preset://k8s/basic@v1) with local vendoring.
		•	extends: URL or preset to inherit from, with override blocks.
	•	Selectors & filters:
		•	labelSelector, fieldSelector, name/glob filters; include/exclude precedence clarified in schema docs.
	•	Compatibility:
		•	--emit v1beta2 to produce a concrete legacy spec; downgrade warnings if some intent can’t fully map.
		•	--explain prints the expanded collectors/analyzers to aid review and vendoring.
	•	Example: Preflight with requirements + docs

```yaml
apiVersion: troubleshoot.sh/v1beta3
kind: Preflight
metadata:
  name: example
requirements:
  - name: Baseline
    docString: "Core Kubernetes and cluster requirements."
    checks:
      - clusterVersion:
          checkName: Kubernetes version
          outcomes:
            - fail:
                when: "< 1.20.0"
                message: This application requires at least Kubernetes 1.20.0, and recommends 1.22.0.
                uri: https://kubernetes.io
            - warn:
                when: "< 1.22.0"
                message: Your cluster meets the minimum version of Kubernetes, but we recommend you update to 1.22.0 or later.
                uri: https://kubernetes.io
            - pass:
                when: ">= 1.22.0"
                message: Your cluster meets the recommended and required versions of Kubernetes.
      - customResourceDefinition:
          checkName: Ingress
          customResourceDefinitionName: ingressroutes.contour.heptio.com
          outcomes:
            - fail:
                message: Contour ingress not found!
            - pass:
                message: Contour ingress found!
      - containerRuntime:
          outcomes:
            - pass:
                when: "== containerd"
                message: containerd container runtime was found.
            - fail:
                message: Did not find containerd container runtime.
      - storageClass:
          checkName: Required storage classes
          storageClassName: "default"
          outcomes:
            - fail:
                message: Could not find a storage class called default.
            - pass:
                message: All good on storage classes
      - distribution:
          outcomes:
            - fail:
                when: "== docker-desktop"
                message: The application does not support Docker Desktop Clusters
            - fail:
                when: "== microk8s"
                message: The application does not support Microk8s Clusters
            - fail:
                when: "== minikube"
                message: The application does not support Minikube Clusters
            - pass:
                when: "== eks"
                message: EKS is a supported distribution
            - pass:
                when: "== gke"
                message: GKE is a supported distribution
            - pass:
                when: "== aks"
                message: AKS is a supported distribution
            - pass:
                when: "== kurl"
                message: KURL is a supported distribution
            - pass:
                when: "== digitalocean"
                message: DigitalOcean is a supported distribution
            - pass:
                when: "== rke2"
                message: RKE2 is a supported distribution
            - pass:
                when: "== k3s"
                message: K3S is a supported distribution
            - pass:
                when: "== oke"
                message: OKE is a supported distribution
            - pass:
                when: "== kind"
                message: Kind is a supported distribution
            - warn:
                message: Unable to determine the distribution of Kubernetes
      - nodeResources:
          checkName: Must have at least 3 nodes in the cluster, with 5 recommended
          outcomes:
            - fail:
                when: "count() < 3"
                message: This application requires at least 3 nodes.
                uri: https://kurl.sh/docs/install-with-kurl/adding-nodes
            - warn:
                when: "count() < 5"
                message: This application recommends at last 5 nodes.
                uri: https://kurl.sh/docs/install-with-kurl/adding-nodes
            - pass:
                message: This cluster has enough nodes.
      - nodeResources:
          checkName: Every node in the cluster must have at least 8 GB of memory, with 32 GB recommended
          outcomes:
            - fail:
                when: "min(memoryCapacity) < 8Gi"
                message: All nodes must have at least 8 GB of memory.
                uri: https://kurl.sh/docs/install-with-kurl/system-requirements
            - warn:
                when: "min(memoryCapacity) < 32Gi"
                message: All nodes are recommended to have at least 32 GB of memory.
                uri: https://kurl.sh/docs/install-with-kurl/system-requirements
            - pass:
                message: All nodes have at least 32 GB of memory.
      - nodeResources:
          checkName: Total CPU Cores in the cluster is 4 or greater
          outcomes:
            - fail:
                when: "sum(cpuCapacity) < 4"
                message: The cluster must contain at least 4 cores
                uri: https://kurl.sh/docs/install-with-kurl/system-requirements
            - pass:
                message: There are at least 4 cores in the cluster
      - nodeResources:
          checkName: Every node in the cluster must have at least 40 GB of ephemeral storage, with 100 GB recommended
          outcomes:
            - fail:
                when: "min(ephemeralStorageCapacity) < 40Gi"
                message: All nodes must have at least 40 GB of ephemeral storage.
                uri: https://kurl.sh/docs/install-with-kurl/system-requirements
            - warn:
                when: "min(ephemeralStorageCapacity) < 100Gi"
                message: All nodes are recommended to have at least 100 GB of ephemeral storage.
                uri: https://kurl.sh/docs/install-with-kurl/system-requirements
            - pass:
                message: All nodes have at least 100 GB of ephemeral storage.

{{- if eq .Values.postgres.enabled true }}
  - name: Postgres
    docString: "Postgres needs a storage class and sufficient memory."
    checks:
      - storageClass:
          checkName: Postgres storage class
          name: "{{ .Values.postgres.storageClassName | default \"default\" }}"
          required: true
      - nodeResources:
          checkName: Postgres memory guidance
          outcomes:
            - fail:
                when: "min(memoryCapacity) < 8Gi"
                message: All nodes must have at least 8 GB of memory for Postgres.
            - warn:
                when: "min(memoryCapacity) < 32Gi"
                message: Nodes are recommended to have at least 32 GB of memory for Postgres.
            - pass:
                message: Nodes have sufficient memory for Postgres.
{{- end }}

{{- if eq .Values.redis.enabled true }}
  - name: Redis
    docString: "Redis needs a storage class and adequate ephemeral storage."
    checks:
      - storageClass:
          checkName: Redis storage class
          name: "{{ .Values.redis.storageClassName | default \"default\" }}"
          required: true
      - nodeResources:
          checkName: Redis ephemeral storage
          outcomes:
            - fail:
                when: "min(ephemeralStorageCapacity) < 40Gi"
                message: All nodes must have at least 40 GB of ephemeral storage for Redis.
            - warn:
                when: "min(ephemeralStorageCapacity) < 100Gi"
                message: Nodes are recommended to have at least 100 GB of ephemeral storage for Redis.
            - pass:
                message: Nodes have sufficient ephemeral storage for Redis.
{{- end }}
```

	•	Presets library:
		•	Versioned URIs (e.g., preset://k8s/basic@v1, preset://app/logs@v1) maintained in-repo and publishable.
		•	"troubleshoot vendor --import" downloads presets to ./vendor/troubleshoot/ for offline builds.

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

Preflight CLI: Values and --set support (templating)

• Goal: Let end customers pass Values at runtime to drive a single modular YAML with conditionals.
• Scope: `preflight` gains `--values` (repeatable) and `--set key=value` (repeatable), rendered over the input YAML before loading specs.
• Template engine: Go text/template + Sprig, with `.Values` bound. Standard delimiters `{{` `}}`.
• Precedence:
	• `--set` overrides everything (last one wins when repeated)
	• Later `--values` files override earlier ones (left-to-right deep merge)
	• Defaults embedded in the YAML are lowest precedence
• Merge:
	• Maps: deep-merge
	• Slices: replace (whole list)
• Types:
	• `true|false` parsed as bool, numbers as float/int when unquoted, everything else as string
	• Use quotes to force string: `--set image.tag="1.2.3"`

Example usage

```bash
# combine file values with inline overrides
preflight ./some-preflight-checks.yaml \
  --values ./values.yaml \
  --values ./values-prod.yaml \
  --set postgres.enabled=true \
  --set redis.enabled=false
```

Minimal Values schema (illustrative)

```yaml
postgres:
  enabled: false
  storageClassName: default
redis:
  enabled: true
  storageClassName: default
```

Single-file modular YAML authoring pattern

```yaml
apiVersion: troubleshoot.sh/v1beta3
kind: Preflight
metadata:
  name: example
requirements:
  - name: Baseline
    docString: "Core Kubernetes requirements."
    checks:
      - k8sVersion: ">=1.22"
      - distribution:
          allow: [eks, gke, aks, kurl, digitalocean, rke2, k3s, oke, kind]
          deny:  [docker-desktop, microk8s, minikube]
      - storageClass:
          name: "default"
          required: true

{{- if eq .Values.postgres.enabled true }}
  - name: Postgres
    docString: "Postgres needs a storage class and sufficient memory."
    checks:
      - storageClass:
          name: "{{ .Values.postgres.storageClassName | default \"default\" }}"
          required: true
      - nodes:
          memoryPerNode: ">=8Gi"
          recommendMemoryPerNode: ">=32Gi"
{{- end }}

{{- if eq .Values.redis.enabled true }}
  - name: Redis
    docString: "Redis needs a storage class and adequate ephemeral storage."
    checks:
      - storageClass:
          name: "{{ .Values.redis.storageClassName | default \"default\" }}"
          required: true
      - nodes:
          ephemeralPerNode: ">=40Gi"
          recommendEphemeralPerNode: ">=100Gi"
{{- end }}
```

Notes
• Keep everything in one YAML; conditionals gate entire requirement blocks.
• Authors can still drop down to raw analyzers; the renderer runs before spec parsing, so both styles work.
• Add `--dry-run` to print the rendered spec without executing checks.