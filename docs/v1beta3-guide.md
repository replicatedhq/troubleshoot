## Writing modular, templated Preflight specs (v1beta3 style)

This guide shows how to author preflight YAML specs in a modular, values-driven style like `v1beta3.yaml`. The goal is to keep checks self-documenting, easy to toggle on/off, and customizable via values files or inline `--set` flags.


### Core structure

- **Header**
  - `apiVersion`: `troubleshoot.sh/v1beta3`
  - `kind`: `Preflight`
  - `metadata.name`: a short, stable identifier
- **Spec**
  - `spec.analyzers`: list of checks (analyzers)
  - Each analyzer is optionally guarded by templating conditionals (e.g., `{{- if .Values.kubernetes.enabled }}`)
  - A `docString` accompanies each analyzer, describing the requirement, why it matters, and any links


### Use templating and values

The examples use Go templates with the standard Sprig function set. Values can be supplied by files (`--values`) and/or inline overrides (`--set`), and accessed in templates via `.Values`.

- **Toggling sections**: wrap analyzer blocks in conditionals tied to values.
  ```yaml
  {{- if .Values.storageClass.enabled }}
  - docString: |
      Title: Default StorageClass Requirements
      Requirement:
        - A StorageClass named "{{ .Values.storageClass.className }}" must exist
      Default StorageClass enables dynamic PVC provisioning without manual intervention.
    storageClass:
      checkName: Default StorageClass
      storageClassName: '{{ .Values.storageClass.className }}'
      outcomes:
        - fail:
            message: Default StorageClass not found
        - pass:
            message: Default StorageClass present
  {{- end }}
  ```

- **Values**: template expressions directly use values from your values files.
  ```yaml
  {{ .Values.clusterVersion.minVersion }}
  ```

- **Nested conditionals**: further constrain checks (e.g., only when a specific CRD is required).
  ```yaml
  {{- if .Values.crd.enabled }}
  - docString: |
      Title: Required CRD Presence
      Requirement:
        - CRD must exist: {{ .Values.crd.name }}
      The application depends on this CRD for controllers to reconcile desired state.
    customResourceDefinition:
      checkName: Required CRD
      customResourceDefinitionName: '{{ .Values.crd.name }}'
      outcomes:
        - fail:
            message: Required CRD not found
        - pass:
            message: Required CRD present
  {{- end }}
  ```


### Author high-quality docString blocks

Every analyzer should start with a `docString` so you can extract documentation automatically:

- **Title**: a concise name for the requirement
- **Requirement**: bullet list of specific, testable criteria (e.g., versions, counts, names)
- **Rationale**: 1–3 sentences explaining why the requirement exists and the impact if unmet
- **Links**: include authoritative docs with stable URLs

Example:
```yaml
docString: |
    Title: Required CRDs and Ingress Capabilities
    Requirement:
        - Ingress Controller: Contour
        - CRD must be present:
            - Group: heptio.com
            - Kind: IngressRoute
            - Version: v1beta1 or later served version
        The ingress layer terminates TLS and routes external traffic to Services.
        Contour relies on the IngressRoute CRD to express host/path routing, TLS
        configuration, and policy. If the CRD is not installed and served by the
        API server, Contour cannot reconcile desired state, leaving routes
        unconfigured and traffic unreachable.
```


### Choose the right analyzer type and outcomes

Use the analyzer that matches the requirement, and enumerate `outcomes` with clear messages. Common analyzers in this style:

- **clusterVersion**: compare to min and recommended versions
  ```yaml
  clusterVersion:
    checkName: Kubernetes version
    outcomes:
      - fail:
          when: '< {{ .Values.clusterVersion.minVersion }}'
          message: Requires at least Kubernetes {{ .Values.clusterVersion.minVersion }}.
      - warn:
          when: '< {{ .Values.clusterVersion.recommendedVersion }}'
          message: Recommended to use Kubernetes {{ .Values.clusterVersion.recommendedVersion }} or later.
      - pass:
          when: '>= {{ .Values.clusterVersion.recommendedVersion }}'
          message: Meets recommended and required Kubernetes versions.
  ```

- **customResourceDefinition**: ensure a CRD exists
  ```yaml
  customResourceDefinition:
    checkName: Required CRD
    customResourceDefinitionName: '{{ .Values.crd.name }}'
    outcomes:
      - fail:
          message: Required CRD not found
      - pass:
          message: Required CRD present
  ```

- **containerRuntime**: verify container runtime
  ```yaml
  containerRuntime:
    outcomes:
      - pass:
          when: '== containerd'
          message: containerd runtime detected
      - fail:
          message: Unsupported container runtime; containerd required
  ```

- **storageClass**: check for a named StorageClass (often the default)
  ```yaml
  storageClass:
    checkName: Default StorageClass
    storageClassName: '{{ .Values.analyzers.storageClass.className }}'
    outcomes:
      - fail:
          message: Default StorageClass not found
      - pass:
          message: Default StorageClass present
  ```

- **distribution**: whitelist/blacklist distributions
  ```yaml
  distribution:
    checkName: Supported distribution
    outcomes:
      {{- range $d := .Values.distribution.unsupported }}
      - fail:
          when: '== {{ $d }}'
          message: '{{ $d }} is not supported'
      {{- end }}
      {{- range $d := .Values.distribution.supported }}
      - pass:
          when: '== {{ $d }}'
          message: '{{ $d }} is a supported distribution'
      {{- end }}
      - warn:
          message: Unable to determine the distribution
  ```

- **nodeResources**: aggregate across nodes; common patterns include count, CPU, memory, and ephemeral storage
  ```yaml
  # Node count requirement
  nodeResources:
    checkName: Node count
    outcomes:
      - fail:
          when: 'count() < {{ .Values.nodeResources.count.min }}'
          message: Requires at least {{ .Values.nodeResources.count.min }} nodes
      - warn:
          when: 'count() < {{ .Values.nodeResources.count.recommended }}'
          message: Recommended at least {{ .Values.nodeResources.count.recommended }} nodes
      - pass:
          message: Cluster has sufficient nodes

  # Cluster CPU total
  nodeResources:
    checkName: Cluster CPU total
    outcomes:
      - fail:
          when: 'sum(cpuCapacity) < {{ .Values.nodeResources.cpu.min }}'
          message: Requires at least {{ .Values.nodeResources.cpu.min }} cores
      - pass:
          message: Cluster CPU capacity meets requirement

  # Per-node memory (Gi)
  nodeResources:
    checkName: Per-node memory
    outcomes:
      - fail:
          when: 'min(memoryCapacity) < {{ .Values.nodeResources.memory.minGi }}Gi'
          message: All nodes must have at least {{ .Values.nodeResources.memory.minGi }} GiB
      - warn:
          when: 'min(memoryCapacity) < {{ .Values.nodeResources.memory.recommendedGi }}Gi'
          message: Recommended {{ .Values.nodeResources.memory.recommendedGi }} GiB per node
      - pass:
          message: All nodes meet recommended memory

  # Per-node ephemeral storage (Gi)
  nodeResources:
    checkName: Per-node ephemeral storage
    outcomes:
      - fail:
          when: 'min(ephemeralStorageCapacity) < {{ .Values.nodeResources.ephemeral.minGi }}Gi'
          message: All nodes must have at least {{ .Values.nodeResources.ephemeral.minGi }} GiB
      - warn:
          when: 'min(ephemeralStorageCapacity) < {{ .Values.nodeResources.ephemeral.recommendedGi }}Gi'
          message: Recommended {{ .Values.nodeResources.ephemeral.recommendedGi }} GiB per node
      - pass:
          message: All nodes meet recommended ephemeral storage
  ```

- **deploymentStatus**: verify workload deployment status
  ```yaml
  deploymentStatus:
    checkName: Deployment ready
    namespace: '{{ .Values.workloads.deployments.namespace }}'
    name: '{{ .Values.workloads.deployments.name }}'
    outcomes:
      - fail:
          when: absent
          message: Deployment not found
      - fail:
          when: '< {{ .Values.workloads.deployments.minReady }}'
          message: Deployment has insufficient ready replicas
      - pass:
          when: '>= {{ .Values.workloads.deployments.minReady }}'
          message: Deployment has sufficient ready replicas
  ```

- **postgres/mysql/redis**: database connectivity (requires collectors)
  ```yaml
  # Collector section
  - postgres:
      collectorName: '{{ .Values.databases.postgres.collectorName }}'
      uri: '{{ .Values.databases.postgres.uri }}'

  # Analyzer section
  postgres:
    checkName: Postgres checks
    collectorName: '{{ .Values.databases.postgres.collectorName }}'
    outcomes:
      - fail:
          message: Postgres checks failed
      - pass:
          message: Postgres checks passed
  ```

- **textAnalyze/yamlCompare/jsonCompare**: analyze collected data
  ```yaml
  textAnalyze:
    checkName: Text analyze
    collectorName: 'cluster-resources'
    fileName: '{{ .Values.textAnalyze.fileName }}'
    regex: '{{ .Values.textAnalyze.regex }}'
    outcomes:
      - fail:
          message: Pattern matched in files
      - pass:
          message: Pattern not found
  ```


### Design conventions for maintainability

- **Guard every optional analyzer** with a values toggle, so consumers can enable only what they need.
- **Always include collectors section** when analyzers require them (databases, http, registryImages, etc.).
- **Use `checkName`** to provide a stable, user-facing label for each check.
- **Prefer `fail` for unmet hard requirements**, `warn` for soft requirements, and `pass` with a direct, affirmative message.
- **Attach `uri`** to outcomes when helpful for remediation.
- **Keep docString in sync** with the actual checks; avoid drift by templating values into both the docs and the analyzer.
- **Ensure values files contain all required fields** since templates now directly use values without fallback defaults.


### Values files: shape and examples

Provide a values schema that mirrors your toggles and thresholds. Example full and minimal values are included in this repository:

- `values-v1beta3-full.yaml` (all features enabled, opinionated defaults)
- `values-v1beta3-minimal.yaml` (most features disabled, conservative thresholds)

Typical structure:
```yaml
clusterVersion:
  enabled: true
  minVersion: "1.24.0"
  recommendedVersion: "1.28.0"

storageClass:
  enabled: true
  className: "standard"

crd:
  enabled: true
  name: "samples.mycompany.com"

containerRuntime:
  enabled: true

distribution:
  enabled: true
  supported: ["eks", "gke", "aks", "kubeadm"]
  unsupported: []

nodeResources:
  count:
    enabled: true
    min: 1
    recommended: 3
  cpu:
    enabled: true
    min: "4"
  memory:
    enabled: true
    minGi: 8
    recommendedGi: 16
  ephemeral:
    enabled: true
    minGi: 20
    recommendedGi: 50

workloads:
  deployments:
    enabled: true
    namespace: "default"
    name: "example-deploy"
    minReady: 1

databases:
  postgres:
    enabled: true
    collectorName: "postgres"
    uri: "postgres://user:pass@postgres:5432/db?sslmode=disable"
  mysql:
    enabled: true
    collectorName: "mysql"
    uri: "mysql://user:pass@tcp(mysql:3306)/db"
```


### Render, run, and extract docs

You can render templates, run preflights with values, and extract requirement docs without running checks.

- **Render a templated preflight spec** to stdout or a file:
  ```bash
  preflight template v1beta3.yaml \
    --values values-base.yaml \
    --values values-prod.yaml \
    --set storage.className=fast-local \
    -o rendered-preflight.yaml
  ```

- **Run preflights with values** (values and sets also work with `preflight` root command):
  ```bash
  preflight run rendered-preflight.yaml
  # or run directly against the template with values
  preflight run v1beta3.yaml --values values-prod.yaml --set cluster.minNodes=5
  ```

- **Extract only documentation** from enabled analyzers in one or more templates:
  ```bash
  preflight docs v1beta3.yaml other-spec.yaml \
    --values values-prod.yaml \
    --set kubernetes.enabled=true \
    -o REQUIREMENTS.md
  ```

Notes:
- Multiple `--values` files are merged in order; later files win.
- `--set` uses Helm-style semantics for nested keys and types, applied after files.


### Authoring checklist

- Add `docString` with Title, Requirement bullets, rationale, and links.
- Gate optional analyzers with `{{- if .Values.analyzers.<feature>.enabled }}`.
- Parameterize thresholds and names with `.Values` expressions.
- Ensure all required values are present in your values files since there are no fallback defaults.
- Use precise, user-actionable `message` text for each outcome; add `uri` where helpful.
- Prefer a minimal values file with everything disabled, and a full values file enabling most checks.
- Test with `preflight template` (no values, minimal, full) and verify `preflight docs` output reads well.


### Example skeleton to start a new spec

```yaml
apiVersion: troubleshoot.sh/v1beta3
kind: Preflight
metadata:
  name: your-product-preflight
spec:
  {{- /* Determine if we need explicit collectors beyond always-on clusterResources */}}
  {{- $needExtraCollectors := or .Values.databases.postgres.enabled .Values.http.enabled }}

  collectors:
    # Always collect cluster resources to support core analyzers
    - clusterResources: {}

    {{- if .Values.databases.postgres.enabled }}
    - postgres:
        collectorName: '{{ .Values.databases.postgres.collectorName }}'
        uri: '{{ .Values.databases.postgres.uri }}'
    {{- end }}

  analyzers:
    {{- if .Values.clusterVersion.enabled }}
    - docString: |
        Title: Kubernetes Control Plane Requirements
        Requirement:
          - Version:
            - Minimum: {{ .Values.clusterVersion.minVersion }}
            - Recommended: {{ .Values.clusterVersion.recommendedVersion }}
          - Docs: https://kubernetes.io
        These version targets ensure required APIs and defaults are available and patched.
      clusterVersion:
        checkName: Kubernetes version
        outcomes:
          - fail:
              when: '< {{ .Values.clusterVersion.minVersion }}'
              message: Requires at least Kubernetes {{ .Values.clusterVersion.minVersion }}.
          - warn:
              when: '< {{ .Values.clusterVersion.recommendedVersion }}'
              message: Recommended to use Kubernetes {{ .Values.clusterVersion.recommendedVersion }} or later.
          - pass:
              when: '>= {{ .Values.clusterVersion.recommendedVersion }}'
              message: Meets recommended and required Kubernetes versions.
    {{- end }}

    {{- if .Values.storageClass.enabled }}
    - docString: |
        Title: Default StorageClass Requirements
        Requirement:
          - A StorageClass named "{{ .Values.storageClass.className }}" must exist
        A default StorageClass enables dynamic PVC provisioning without manual intervention.
      storageClass:
        checkName: Default StorageClass
        storageClassName: '{{ .Values.storageClass.className }}'
        outcomes:
          - fail:
              message: Default StorageClass not found
          - pass:
              message: Default StorageClass present
    {{- end }}

    {{- if .Values.databases.postgres.enabled }}
    - docString: |
        Title: Postgres Connectivity
        Requirement:
          - Postgres checks collected by '{{ .Values.databases.postgres.collectorName }}' must pass
      postgres:
        checkName: Postgres checks
        collectorName: '{{ .Values.databases.postgres.collectorName }}'
        outcomes:
          - fail:
              message: Postgres checks failed
          - pass:
              message: Postgres checks passed
    {{- end }}
```


### References

- Example template in this repo: `v1beta3-all-analyzers.yaml`
- Values example: `values-v1beta3-all-analyzers.yaml`


