# Design: Shared Bundle Contract for sbctl and Troubleshoot

**Date:** 2026-01-22
**Status:** Draft
**Author:** Design session with Claude Code

## Problem Statement

sbctl is a companion tool to troubleshoot that creates a local Kubernetes API server from support bundles, enabling users to run `kubectl` commands against captured cluster state. Currently:

1. **Maintenance burden**: sbctl requires constant updates to stay in sync with troubleshoot's collector changes
2. **No formal contract**: No schema defines how troubleshoot writes resources or how sbctl reads them
3. **Type-specific code**: sbctl has ~1800 lines of switch statements handling each resource type individually
4. **Drift risk**: As troubleshoot adds capabilities, sbctl may not reflect them, causing inconsistent behavior

## Goals

1. **Eliminate maintenance burden** - sbctl should not require updates when troubleshoot adds collectors or changes file structure
2. **Enable dynamic resource support** - sbctl should serve any Kubernetes resource troubleshoot collects, without code changes
3. **Perfect fidelity** - sbctl output must match what a real Kubernetes API server would return; debuggers must trust the output
4. **Backwards compatibility** - New and old versions of bundles and sbctl must interoperate

## Non-Goals

- Modifying the sbctl CLI interface
- Supporting write operations
- Connecting to live clusters

## Solution Overview

A new `pkg/bundle` package in troubleshoot defines the contract between collection and serving. sbctl remains a separate binary but imports shared libraries from troubleshoot. Key changes:

1. **Pre-compute table representations** at collection time, when cluster access exists
2. **Store API discovery metadata** alongside resources
3. **Discover field selectors** at runtime with static fallback
4. **Serve resources generically** using unstructured objects with metadata-driven behavior
5. **Collect failsafe** - table and metadata failures never block raw resource collection

## Detailed Design

### New Bundle Structure

The support bundle structure gains a `_meta/` directory for metadata. Existing structure remains unchanged for backwards compatibility.

```
cluster-resources/
├── _meta/                              # NEW: Metadata directory
│   ├── discovery.json                  # API groups and resources
│   └── selectable-fields/              # Field selector definitions (optional)
│       ├── v1.pods.json
│       ├── v1.events.json
│       └── {group}.{version}.{resource}.json
│
├── pods/                               # EXISTING: Namespaced resources
│   ├── default.json                    # Raw resource list
│   ├── default.table.json              # NEW: Pre-computed table
│   ├── kube-system.json
│   └── kube-system.table.json
│
├── nodes.json                          # EXISTING: Cluster-scoped resources
├── nodes.table.json                    # NEW: Pre-computed table
│
└── custom-resources/                   # EXISTING: CRDs
    └── mycrd.example.com/
        ├── default.json
        └── default.table.json          # NEW: Pre-computed table
```

### Pre-computed Table Format

When troubleshoot collects resources, it also captures the table representation by requesting with the Table Accept header:

```http
GET /api/v1/namespaces/default/pods
Accept: application/json;as=Table;v=v1;g=meta.k8s.io
```

The response includes `columnDefinitions` and pre-computed `cells` for each row:

```json
{
  "kind": "Table",
  "apiVersion": "meta.k8s.io/v1",
  "columnDefinitions": [
    {"name": "Name", "type": "string", "format": "name", "priority": 0},
    {"name": "Ready", "type": "string", "priority": 0},
    {"name": "Status", "type": "string", "priority": 0},
    {"name": "Restarts", "type": "string", "priority": 0},
    {"name": "Age", "type": "string", "priority": 0}
  ],
  "rows": [
    {
      "cells": ["nginx-7d4b8d6b8-x2j4k", "1/1", "Running", "0", "5d"],
      "object": { "apiVersion": "v1", "kind": "Pod", ... }
    }
  ]
}
```

This is stored as `{resource}/{namespace}.table.json` alongside the raw `{namespace}.json`.

### Discovery Metadata

`_meta/discovery.json` captures API server discovery information and bundle schema version:

```json
{
  "bundleSchemaVersion": "1.0",
  "collectedAt": "2026-01-22T10:30:00Z",
  "kubernetesVersion": "v1.28.4",
  "apiVersion": "v1",
  "groups": [
    {
      "name": "",
      "versions": [{"groupVersion": "v1", "version": "v1"}],
      "preferredVersion": {"groupVersion": "v1", "version": "v1"}
    },
    {
      "name": "apps",
      "versions": [{"groupVersion": "apps/v1", "version": "v1"}],
      "preferredVersion": {"groupVersion": "apps/v1", "version": "v1"}
    }
  ],
  "resources": {
    "v1": [
      {"name": "pods", "namespaced": true, "kind": "Pod", "verbs": ["get", "list"]},
      {"name": "nodes", "namespaced": false, "kind": "Node", "verbs": ["get", "list"]}
    ],
    "apps/v1": [
      {"name": "deployments", "namespaced": true, "kind": "Deployment", "verbs": ["get", "list"]}
    ]
  }
}
```

The `bundleSchemaVersion` field enables future schema evolution:
- sbctl checks version and uses appropriate parsing logic
- Unknown versions fall back to legacy behavior
- Semantic versioning: minor versions are backwards compatible

### Shared Package: `pkg/bundle`

A new package shared between collectors and sbctl defines the bundle contract.

#### `pkg/bundle/schema.go`

```go
package bundle

const (
    ClusterResourcesDir = "cluster-resources"
    MetaDir            = "_meta"
    DiscoveryFile      = "discovery.json"
    SelectableFieldsDir = "selectable-fields"
    TableFileSuffix    = ".table.json"
)

// ResourcePath returns the path for a resource file
func ResourcePath(resource, namespace string) string {
    if namespace == "" {
        return fmt.Sprintf("%s.json", resource)
    }
    return fmt.Sprintf("%s/%s.json", resource, namespace)
}

// TablePath returns the path for a table file
func TablePath(resource, namespace string) string {
    if namespace == "" {
        return fmt.Sprintf("%s.table.json", resource)
    }
    return fmt.Sprintf("%s/%s.table.json", resource, namespace)
}

// SelectableFieldsPath returns the path for discovered selectable fields
func SelectableFieldsPath(gvr schema.GroupVersionResource) string {
    group := gvr.Group
    if group == "" {
        group = "core"
    }
    return filepath.Join(MetaDir, SelectableFieldsDir,
        fmt.Sprintf("%s.%s.%s.json", group, gvr.Version, gvr.Resource))
}
```

#### `pkg/bundle/fields.go`

```go
package bundle

// SelectableFields maps GVR to list of selectable field paths
// Source: k8s.io/kubernetes/pkg/registry/{group}/{resource}/strategy.go
var SelectableFields = map[string][]string{
    // Core v1
    "core/v1/pods": {
        "metadata.name",
        "metadata.namespace",
        "spec.nodeName",
        "spec.restartPolicy",
        "spec.schedulerName",
        "spec.serviceAccountName",
        "status.phase",
        "status.podIP",
        "status.nominatedNodeName",
    },
    "core/v1/events": {
        "metadata.name",
        "metadata.namespace",
        "involvedObject.apiVersion",
        "involvedObject.fieldPath",
        "involvedObject.kind",
        "involvedObject.name",
        "involvedObject.namespace",
        "involvedObject.resourceVersion",
        "involvedObject.uid",
        "reason",
        "reportingComponent",
        "source",
        "type",
    },
    "core/v1/nodes": {
        "metadata.name",
        "spec.unschedulable",
    },
    "core/v1/namespaces": {
        "metadata.name",
        "status.phase",
    },
    "core/v1/secrets": {
        "metadata.name",
        "metadata.namespace",
        "type",
    },
    "core/v1/services": {
        "metadata.name",
        "metadata.namespace",
    },
    // Apps v1
    "apps/v1/replicasets": {
        "metadata.name",
        "metadata.namespace",
        "status.replicas",
    },
    "apps/v1/deployments": {
        "metadata.name",
        "metadata.namespace",
    },
    // Batch v1
    "batch/v1/jobs": {
        "metadata.name",
        "metadata.namespace",
        "status.successful",
    },
    // Add other resources as needed...
}

// GetSelectableFields returns selectable fields for a GVR,
// falling back to metadata-only for unknown resources
func GetSelectableFields(group, version, resource string) []string {
    key := fmt.Sprintf("%s/%s/%s", group, version, resource)
    if group == "" {
        key = fmt.Sprintf("core/%s/%s", version, resource)
    }

    if fields, ok := SelectableFields[key]; ok {
        return fields
    }

    // Default: all resources support metadata selectors
    return []string{"metadata.name", "metadata.namespace"}
}

// LoadSelectableFields loads selectable fields using hybrid approach:
// 1. Check for discovered fields in bundle (preferred)
// 2. Fall back to static SelectableFields map
// 3. Fall back to metadata-only fields
func LoadSelectableFields(bundlePath string, gvr schema.GroupVersionResource) []string {
    // Try discovered fields first
    discoveredPath := filepath.Join(bundlePath, ClusterResourcesDir, SelectableFieldsPath(gvr))
    if data, err := os.ReadFile(discoveredPath); err == nil {
        var fields []string
        if json.Unmarshal(data, &fields) == nil && len(fields) > 0 {
            return fields
        }
    }

    // Fall back to static map
    return GetSelectableFields(gvr.Group, gvr.Version, gvr.Resource)
}
```

#### `pkg/bundle/extract.go`

```go
package bundle

import (
    "strconv"
    "strings"

    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    "k8s.io/apimachinery/pkg/fields"
)

// ExtractSelectableFields builds a fields.Set from an unstructured object
func ExtractSelectableFields(obj *unstructured.Unstructured, fieldPaths []string) fields.Set {
    result := fields.Set{}

    for _, path := range fieldPaths {
        value := extractFieldValue(obj, path)
        result[path] = value
    }

    return result
}

func extractFieldValue(obj *unstructured.Unstructured, path string) string {
    parts := strings.Split(path, ".")

    val, found, _ := unstructured.NestedFieldNoCopy(obj.Object, parts...)
    if !found {
        return ""
    }

    switch v := val.(type) {
    case string:
        return v
    case int64:
        return strconv.FormatInt(v, 10)
    case float64:
        return strconv.FormatFloat(v, 'f', -1, 64)
    case bool:
        return strconv.FormatBool(v)
    case []interface{}:
        return formatSlice(v)
    default:
        return fmt.Sprintf("%v", v)
    }
}

func formatSlice(slice []interface{}) string {
    strs := make([]string, 0, len(slice))
    for _, item := range slice {
        strs = append(strs, fmt.Sprintf("%v", item))
    }
    return strings.Join(strs, ",")
}
```

### Collection Changes

The `clusterResources` collector gains additional collection logic. **Critical requirement: collection must be failsafe.** Failures in table or metadata collection must never prevent raw resource collection from succeeding.

```go
func (c *CollectClusterResources) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
    // ... existing collection logic ...

    // For each resource type collected (including CRDs):
    for _, gvr := range collectedResources {
        // 1. Collect raw resources (existing) - this MUST succeed
        rawResult, err := collectRawResources(client, gvr, namespace)
        if err != nil {
            // Handle error per existing behavior
            continue
        }
        output.SaveResult(bundle.ResourcePath(gvr.Resource, namespace), rawResult)

        // 2. Collect table representation (NEW) - failsafe, errors logged but not fatal
        tableResult, err := collectTableRepresentation(client, gvr, namespace)
        if err != nil {
            // Log warning but continue - some resources (especially CRDs) may not support table format
            log.Warnf("Failed to collect table for %s: %v", gvr, err)
        } else {
            output.SaveResult(bundle.TablePath(gvr.Resource, namespace), tableResult)
        }

        // 3. Discover selectable fields (NEW) - failsafe
        fields, err := discoverSelectableFields(client, gvr)
        if err != nil {
            log.Debugf("Could not discover selectable fields for %s: %v", gvr, err)
        } else if len(fields) > 0 {
            output.SaveResult(bundle.SelectableFieldsPath(gvr), fields)
        }
    }

    // Collect discovery metadata (NEW) - failsafe
    discovery, err := collectDiscoveryMetadata(client)
    if err != nil {
        log.Warnf("Failed to collect API discovery metadata: %v", err)
    } else {
        output.SaveResult(path.Join(bundle.MetaDir, bundle.DiscoveryFile), discovery)
    }

    return output, nil
}

func collectTableRepresentation(client rest.Interface, gvr schema.GroupVersionResource, namespace string) ([]byte, error) {
    req := client.Get().
        Resource(gvr.Resource).
        Namespace(namespace).
        SetHeader("Accept", "application/json;as=Table;v=v1;g=meta.k8s.io")

    return req.DoRaw(ctx)
}

// discoverSelectableFields probes the API server to discover which fields are selectable
func discoverSelectableFields(client rest.Interface, gvr schema.GroupVersionResource) ([]byte, error) {
    // Make request with invalid field selector to get error listing valid fields
    req := client.Get().
        Resource(gvr.Resource).
        Param("fieldSelector", "__invalid_field__=x")

    _, err := req.DoRaw(ctx)
    if err == nil {
        // Unexpected success - return empty
        return nil, nil
    }

    // Parse error message for field list
    // Error format: "field label not supported: __invalid_field__"
    // or lists valid fields in some K8s versions
    fields := parseSelectableFieldsFromError(err)
    if len(fields) == 0 {
        return nil, fmt.Errorf("could not parse selectable fields from error")
    }

    return json.Marshal(fields)
}
```

### sbctl Serving Changes

sbctl's API server becomes generic:

```go
type DynamicHandler struct {
    clusterData ClusterData
    hasMetadata bool  // true if _meta/ exists
}

func (h *DynamicHandler) ServeResource(w http.ResponseWriter, r *http.Request) {
    gvr := extractGVR(r)
    namespace := mux.Vars(r)["namespace"]
    name := mux.Vars(r)["name"]

    // Determine file paths
    rawPath := h.resourceFilePath(gvr, namespace)
    tablePath := h.tableFilePath(gvr, namespace)

    // Handle table format requests
    if wantsTable(r) && h.hasMetadata && fileExists(tablePath) {
        h.serveTableResponse(w, r, tablePath, name)
        return
    }

    // Handle raw format (or fallback for old bundles)
    h.serveRawResponse(w, r, rawPath, name)
}

func (h *DynamicHandler) serveTableResponse(w http.ResponseWriter, r *http.Request, tablePath, name string) {
    data, err := os.ReadFile(tablePath)
    if err != nil {
        http.Error(w, "Not found", http.StatusNotFound)
        return
    }

    // If requesting single resource, filter table rows
    if name != "" {
        data = filterTableByName(data, name)
    }

    // Apply field/label selectors if present
    if selector := r.URL.Query().Get("fieldSelector"); selector != "" {
        data = filterTableByFieldSelector(data, selector)
    }

    w.Header().Set("Content-Type", "application/json")
    w.Write(data)
}
```

## Backwards Compatibility

### New sbctl + Old bundles (no `_meta/`)

When sbctl encounters a bundle without `_meta/`:
- Falls back to current behavior (hardcoded switch statements)
- Detection: check if `_meta/` directory exists

```go
func (h *DynamicHandler) hasMetadata() bool {
    metaDir := filepath.Join(h.clusterData.ClusterResourcesDir, bundle.MetaDir)
    _, err := os.Stat(metaDir)
    return err == nil
}
```

### Old sbctl + New bundles (has `_meta/`)

Old sbctl versions:
- Ignore `_meta/` directory entirely (unknown directory)
- Continue reading resources from existing locations
- Work exactly as before

This works because:
1. `_meta/` is additive - doesn't change existing file locations
2. `.table.json` files are parallel to existing `.json` files
3. Old sbctl doesn't look for either

## Migration Path

sbctl remains a separate binary with its own repository, but imports shared libraries from troubleshoot.

| Phase | Changes |
|-------|---------|
| Phase 1 | Add `pkg/bundle` to troubleshoot with schema definitions |
| Phase 2 | Update collectors to write `_meta/` and `.table.json` files (failsafe) |
| Phase 3 | sbctl adds troubleshoot as a Go module dependency |
| Phase 4 | Refactor sbctl to use `pkg/bundle` with fallback to legacy |
| Phase 5 | (Later) Remove legacy fallback code from sbctl |

## Bundle Size Impact

Pre-computed table files add minimal storage overhead:
- The `cluster-resources/` directory rarely exceeds 5MB
- Table JSON compresses well within the tar.gz bundle

## Alternatives Considered

### Alternative 1: Dynamic Table Generation

Generate table output at serve time using Kubernetes' internal printers.

**Rejected because:**
- Typed objects for each resource preserve switch statements
- Complex columns (e.g., Pod "Ready" = running/total containers) demand type-specific logic
- Perfect fidelity requires cluster access

### Alternative 2: Unstructured with Basic Tables

Serve all resources as unstructured, provide only NAME/NAMESPACE/AGE columns.

**Rejected because:**
- Violates the fidelity requirement
- Strips resource-specific columns that debuggers rely on
- Degrades the troubleshooting experience unacceptably

### Alternative 3: Static-Only Selectable Fields Mapping

Maintain only a static mapping of selectable fields without runtime discovery.

**Rejected because:**
- Map becomes stale as Kubernetes adds new selectable fields
- No way to know selectable fields for CRDs
- Requires manual updates to track Kubernetes releases

The chosen hybrid approach (runtime discovery at collection time + static fallback) provides better coverage while maintaining offline functionality.

## Design Decisions

### CRD Table Collection

**Decision: Yes, collect table representations for CRDs.**

Rationale:
- CRDs often provide the most valuable custom printer columns (e.g., Certificate expiry, VolumeSnapshot ready state)
- Users expect consistent behavior between core resources and CRDs
- Failsafe collection ensures raw data even when a CRD lacks table format support

### Failsafe Collection

**Decision: Table and metadata collection failures must never block raw resource collection.**

Rationale:
- The cluster-resources collector exists primarily to capture raw resource data
- Some resources, especially CRDs, lack table format support
- Degraded clusters may fail API server discovery
- Users must always receive at least the raw resource data

### Field Selector Discovery

**Decision: Hybrid approach - runtime discovery at collection time with static fallback.**

At collection time:
1. Probe API server with invalid field selector
2. Parse error message to extract valid field list
3. Store discovered fields in `_meta/selectable-fields/`

At serve time (sbctl):
1. Check for discovered fields in bundle
2. Fall back to static `SelectableFields` map
3. Fall back to metadata-only (`metadata.name`, `metadata.namespace`)

**Note:** Field discovery via error parsing is inherently fragile as error message formats vary between Kubernetes versions. The static fallback ensures functionality even when discovery fails.

### Bundle Schema Versioning

**Decision: Include `bundleSchemaVersion` in `_meta/discovery.json`.**

Rationale:
- Enables future schema changes without breaking compatibility
- sbctl can adapt behavior based on schema version
- Unknown versions trigger fallback to legacy behavior
- Semantic versioning: `1.x` versions are backwards compatible with `1.0`

## Limitations

The following features are explicitly **not supported** by sbctl and are out of scope for this design:

### Watch Operations

sbctl serves static bundle data and cannot support watch operations. Commands like `kubectl get pods -w`:
- Return the initial list
- Immediately close the watch stream

This is inherent to sbctl's design as a snapshot-based tool.

### Subresources

Subresources that require live cluster interaction are not supported:

| Subresource | Reason |
|-------------|--------|
| `pods/exec` | Requires running container |
| `pods/attach` | Requires running container |
| `pods/portforward` | Requires running container |
| `pods/log` | Log data not collected by default (separate collector) |
| `*/scale` | Write operation |
| `*/status` | Could be supported if status is in collected data |

### Write Operations

All mutating operations return `405 Method Not Allowed`:
- `kubectl create`, `kubectl apply`, `kubectl delete`
- `kubectl edit`, `kubectl patch`
- `kubectl scale`, `kubectl rollout`

### Real-time Data

sbctl serves point-in-time snapshot data. It cannot reflect:
- Current pod status
- Live metrics
- Recent events (only events collected at bundle time)
- Changes since bundle collection

### List Pagination

Kubernetes `continue` tokens for paginated lists are not implemented. sbctl returns all matching resources in a single response. This design works because:
- Bundles contain finite resources, already filtered at collection time
- Typical bundle sizes fit comfortably in single responses

### resourceVersion Semantics

sbctl does not implement proper `resourceVersion` semantics:
- All resources return a static or omitted `resourceVersion`
- Optimistic concurrency checks are not meaningful
- Watch `resourceVersion` parameters are ignored

## Open Questions

None remaining. All questions have been resolved in the Design Decisions section.

## Success Criteria

1. sbctl can serve any resource troubleshoot collects without code changes
2. `kubectl get <resource>` output matches what a live cluster would return
3. Field selectors work for documented selectable fields
4. Old bundles continue to work with new sbctl
5. No switch statements for resource types in sbctl serving code
6. Table/metadata collection failures never prevent raw resource collection
7. CRD resources have table representations when the CRD supports them

## Testing Strategy

### Unit Tests

**pkg/bundle tests:**
- `ResourcePath()`, `TablePath()`, `SelectableFieldsPath()` return correct paths
- `GetSelectableFields()` returns correct fields for known GVRs
- `GetSelectableFields()` returns metadata-only fallback for unknown GVRs
- `LoadSelectableFields()` prefers discovered fields over static map
- `ExtractSelectableFields()` correctly extracts nested field values

**Collection tests:**
- Table collection failure doesn't prevent raw collection
- Discovery metadata collection failure doesn't prevent resource collection
- Field selector discovery failure logs warning and continues
- CRDs without table support are collected as raw-only

### Integration Tests

**Fidelity tests (compare sbctl vs live cluster):**
```bash
# Collect bundle from live cluster
support-bundle --spec=test-spec.yaml -o test-bundle.tar.gz

# For each resource type:
# 1. Query live cluster
kubectl get pods -o json > live-pods.json
kubectl get pods > live-pods-table.txt

# 2. Query sbctl
sbctl serve test-bundle.tar.gz &
kubectl --server=https://localhost:8443 get pods -o json > sbctl-pods.json
kubectl --server=https://localhost:8443 get pods > sbctl-pods-table.txt

# 3. Compare (ignoring dynamic fields like resourceVersion)
diff <(jq 'del(.metadata.resourceVersion)' live-pods.json) \
     <(jq 'del(.metadata.resourceVersion)' sbctl-pods.json)
diff live-pods-table.txt sbctl-pods-table.txt
```

**Backwards compatibility tests:**
```bash
# Test new sbctl with old bundle (no _meta/)
sbctl serve old-bundle-without-meta.tar.gz
kubectl --server=https://localhost:8443 get pods  # Should work via legacy path

# Test old sbctl with new bundle (has _meta/)
old-sbctl serve new-bundle-with-meta.tar.gz
kubectl --server=https://localhost:8443 get pods  # Should work, ignoring _meta/
```

**Failsafe tests:**
```bash
# Simulate table collection failure (mock API server returning errors)
# Verify raw resources still collected
# Verify bundle is valid and usable

# Simulate discovery metadata failure
# Verify resources still collected
# Verify sbctl falls back to legacy behavior
```

### Field Selector Tests

```bash
# Test field selectors work correctly
kubectl --server=https://localhost:8443 get pods --field-selector=status.phase=Running
kubectl --server=https://localhost:8443 get pods --field-selector=spec.nodeName=node-1
kubectl --server=https://localhost:8443 get events --field-selector=involvedObject.name=my-pod

# Compare results with live cluster output
```

### CRD Tests

```bash
# Collect bundle from cluster with CRDs (e.g., cert-manager Certificates)
# Verify CRD resources have table representations
# Verify kubectl get certificates shows custom columns (Ready, Secret, Age)
# Verify CRDs without additionalPrinterColumns still work (basic columns)
```

### Performance Tests

For large bundles (10k+ resources):
- Measure response time for list operations
- Measure memory usage during filtering
- Verify no timeouts or OOM conditions

## References

- [sbctl repository](https://github.com/replicatedhq/sbctl)
- [Troubleshoot repository](https://github.com/replicatedhq/troubleshoot)
- [Kubernetes Table API](https://kubernetes.io/docs/reference/using-api/api-concepts/#receiving-resources-as-tables)
- [Kubernetes field selectors](https://kubernetes.io/docs/concepts/overview/working-with-objects/field-selectors/)
