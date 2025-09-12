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
  {{- if .Values.storage.enabled }}
  - docString: |
      Title: Default StorageClass Requirements
      Requirement:
        - A StorageClass named "{{ .Values.storage.className }}" must exist
      ...
    storageClass:
      checkName: Default StorageClass
      storageClassName: '{{ .Values.storage.className }}'
      outcomes:
        - fail:
            message: Default StorageClass not found
        - pass:
            message: Default StorageClass present
  {{- end }}
  ```

- **Values**: template expressions directly use values from your values files.
  ```yaml
  {{ .Values.kubernetes.minVersion }}
  ```

- **Nested conditionals**: further constrain checks (e.g., only when a specific ingress type is used).
  ```yaml
  {{- if .Values.ingress.enabled }}
  {{- if eq .Values.ingress.type "Contour" }}
  - docString: |
      Title: Required CRDs and Ingress Capabilities
      ...
    customResourceDefinition:
      checkName: Contour IngressRoute CRD
      customResourceDefinitionName: ingressroutes.contour.heptio.com
      outcomes:
        - fail:
            message: Contour IngressRoute CRD not found; required for ingress routing
        - pass:
            message: Contour IngressRoute CRD present
  {{- end }}
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
          when: '< {{ .Values.kubernetes.minVersion }}'
          message: This application requires at least Kubernetes {{ .Values.kubernetes.minVersion }}.
      - warn:
          when: '< {{ .Values.kubernetes.recommendedVersion }}'
          message: Recommended version is {{ .Values.kubernetes.recommendedVersion }} or later.
      - pass:
          when: '>= {{ .Values.kubernetes.recommendedVersion }}'
          message: Your cluster meets the recommended and required versions of Kubernetes.
  ```

- **customResourceDefinition**: ensure a CRD exists
  ```yaml
  customResourceDefinition:
    checkName: Contour IngressRoute CRD
    customResourceDefinitionName: ingressroutes.contour.heptio.com
    outcomes:
      - fail:
          message: Contour IngressRoute CRD not found; required for ingress routing
      - pass:
          message: Contour IngressRoute CRD present
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
    storageClassName: '{{ .Values.storage.className }}'
    outcomes:
      - fail:
          message: Default StorageClass not found
      - pass:
          message: Default StorageClass present
  ```

- **distribution**: whitelist/blacklist distributions
  ```yaml
  distribution:
    outcomes:
      - fail:
          when: '== docker-desktop'
          message: The application does not support Docker Desktop Clusters
      - pass:
          when: '== eks'
          message: EKS is a supported distribution
      - warn:
          message: Unable to determine the distribution of Kubernetes
  ```

- **nodeResources**: aggregate across nodes; common patterns include count, CPU, memory, and ephemeral storage
  ```yaml
  # Node count requirement
  nodeResources:
    checkName: Node count
    outcomes:
      - fail:
          when: 'count() < {{ .Values.cluster.minNodes }}'
          message: This application requires at least {{ .Values.cluster.minNodes }} nodes.
      - warn:
          when: 'count() < {{ .Values.cluster.recommendedNodes }}'
          message: This application recommends at least {{ .Values.cluster.recommendedNodes }} nodes.
      - pass:
          message: This cluster has enough nodes.

  # Cluster CPU total
  nodeResources:
    checkName: Cluster CPU total
    outcomes:
      - fail:
          when: 'sum(cpuCapacity) < {{ .Values.cluster.minCPU }}'
          message: The cluster must contain at least {{ .Values.cluster.minCPU }} cores
      - pass:
          message: There are at least {{ .Values.cluster.minCPU }} cores in the cluster

  # Per-node memory (Gi)
  nodeResources:
    checkName: Per-node memory requirement
    outcomes:
      - fail:
          when: 'min(memoryCapacity) < {{ .Values.node.minMemoryGi }}Gi'
          message: All nodes must have at least {{ .Values.node.minMemoryGi }} GiB of memory.
      - warn:
          when: 'min(memoryCapacity) < {{ .Values.node.recommendedMemoryGi }}Gi'
          message: All nodes are recommended to have at least {{ .Values.node.recommendedMemoryGi }} GiB of memory.
      - pass:
          message: All nodes have at least {{ .Values.node.recommendedMemoryGi }} GiB of memory.

  # Per-node ephemeral storage (Gi)
  nodeResources:
    checkName: Per-node ephemeral storage requirement
    outcomes:
      - fail:
          when: 'min(ephemeralStorageCapacity) < {{ .Values.node.minEphemeralGi }}Gi'
          message: All nodes must have at least {{ .Values.node.minEphemeralGi }} GiB of ephemeral storage.
      - warn:
          when: 'min(ephemeralStorageCapacity) < {{ .Values.node.recommendedEphemeralGi }}Gi'
          message: All nodes are recommended to have at least {{ .Values.node.recommendedEphemeralGi }} GiB of ephemeral storage.
      - pass:
          message: All nodes have at least {{ .Values.node.recommendedEphemeralGi }} GiB of ephemeral storage.
  ```


### Design conventions for maintainability

- **Guard every optional analyzer** with a values toggle, so consumers can enable only what they need.
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
kubernetes:
  enabled: false
  minVersion: "1.22.0"
  recommendedVersion: "1.29.0"

storage:
  enabled: true
  className: "default"

cluster:
  minNodes: 3
  recommendedNodes: 5
  minCPU: 4

node:
  minMemoryGi: 8
  recommendedMemoryGi: 32
  minEphemeralGi: 40
  recommendedEphemeralGi: 100

ingress:
  enabled: true
  type: "Contour"

runtime:
  enabled: true

distribution:
  enabled: true

nodeChecks:
  enabled: true
  count:
    enabled: true
  cpu:
    enabled: true
  memory:
    enabled: true
  ephemeral:
    enabled: true
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
- Gate optional analyzers with `{{- if .Values.<feature>.enabled }}`.
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
  analyzers:
    {{- if .Values.kubernetes.enabled }}
    - docString: |
        Title: Kubernetes Control Plane Requirements
        Requirement:
          - Version:
            - Minimum: {{ .Values.kubernetes.minVersion }}
            - Recommended: {{ .Values.kubernetes.recommendedVersion }}
        Running below minimum may remove GA APIs and critical fixes.
      clusterVersion:
        checkName: Kubernetes version
        outcomes:
          - fail:
              when: '< {{ .Values.kubernetes.minVersion }}'
              message: Requires Kubernetes >= {{ .Values.kubernetes.minVersion }}.
          - warn:
              when: '< {{ .Values.kubernetes.recommendedVersion }}'
              message: Recommended {{ .Values.kubernetes.recommendedVersion }} or later.
          - pass:
              when: '>= {{ .Values.kubernetes.recommendedVersion }}'
              message: Meets recommended and required versions.
    {{- end }}

    {{- if .Values.storage.enabled }}
    - docString: |
        Title: Default StorageClass Requirements
        Requirement:
          - A StorageClass named "{{ .Values.storage.className }}" must exist
      storageClass:
        checkName: Default StorageClass
        storageClassName: '{{ .Values.storage.className }}'
        outcomes:
          - fail:
              message: Default StorageClass not found
          - pass:
              message: Default StorageClass present
    {{- end }}
```


### References

- Example template in this repo: `v1beta3.yaml`
- Values examples: `values-v1beta3-full.yaml`, `values-v1beta3-minimal.yaml`


