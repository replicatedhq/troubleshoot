# Design: Merging sbctl into Troubleshoot

**Date:** 2026-01-22
**Status:** Draft
**Author:** Design session with Claude Code

## Problem Statement

sbctl is a companion tool to troubleshoot that creates a local Kubernetes API server from support bundles, allowing `kubectl` commands to be run against captured cluster state. Currently:

1. **Maintenance burden**: sbctl requires constant updates to stay in sync with troubleshoot's collector changes
2. **No formal contract**: There's no schema defining how troubleshoot writes resources and how sbctl reads them
3. **Type-specific code**: sbctl has ~1800 lines of switch statements handling each resource type individually
4. **Drift risk**: As troubleshoot adds capabilities, sbctl may not reflect them, causing inconsistent behavior

## Goals

1. **Eliminate maintenance burden** - Stop having to update sbctl every time troubleshoot adds a new collector or changes file structure
2. **Enable dynamic resource support** - sbctl should serve any Kubernetes resource without code changes, as long as troubleshoot collected it
3. **Perfect fidelity** - sbctl output must match what a real Kubernetes API server would return; debuggers must trust the output
4. **Backwards compatibility** - New bundles work with old sbctl; new sbctl works with old bundles

## Non-Goals

- Modifying how users invoke sbctl (CLI interface unchanged)
- Supporting write operations (sbctl remains read-only)
- Real-time cluster connection (sbctl only serves static bundle data)

## Solution Overview

Merge sbctl into the troubleshoot codebase with a shared `pkg/bundle` package that defines the contract between collection and serving. Key changes:

1. **Pre-compute table representations** at collection time (when cluster access exists)
2. **Store API discovery metadata** alongside resources
3. **Use shared selectable fields mapping** for field selector support
4. **Serve resources generically** using unstructured objects with metadata-driven behavior

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

`_meta/discovery.json` captures API server discovery information:

```json
{
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

The `clusterResources` collector gains additional collection logic:

```go
func (c *CollectClusterResources) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
    // ... existing collection logic ...

    // For each resource type collected:
    for _, gvr := range collectedResources {
        // 1. Collect raw resources (existing)
        rawResult := collectRawResources(client, gvr, namespace)
        output.SaveResult(bundle.ResourcePath(gvr.Resource, namespace), rawResult)

        // 2. Collect table representation (NEW)
        tableResult := collectTableRepresentation(client, gvr, namespace)
        output.SaveResult(bundle.TablePath(gvr.Resource, namespace), tableResult)
    }

    // Collect discovery metadata (NEW)
    discovery := collectDiscoveryMetadata(client)
    output.SaveResult(path.Join(bundle.MetaDir, bundle.DiscoveryFile), discovery)

    return output, nil
}

func collectTableRepresentation(client rest.Interface, gvr schema.GroupVersionResource, namespace string) ([]byte, error) {
    req := client.Get().
        Resource(gvr.Resource).
        Namespace(namespace).
        SetHeader("Accept", "application/json;as=Table;v=v1;g=meta.k8s.io")

    return req.DoRaw(ctx)
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

| Phase | Changes |
|-------|---------|
| Phase 1 | Add `pkg/bundle` to troubleshoot with schema definitions |
| Phase 2 | Update collectors to write `_meta/` and `.table.json` files |
| Phase 3 | Move sbctl code into troubleshoot as `pkg/sbctl` or `cmd/sbctl` |
| Phase 4 | Refactor sbctl to use `pkg/bundle` with fallback to legacy |
| Phase 5 | Deprecate standalone sbctl repository |
| Phase 6 | (Later) Remove legacy fallback code from sbctl |

## Bundle Size Impact

Pre-computed table files add storage overhead:
- Approximately 2x storage for resource data (raw + table)
- Table format rows are often smaller than full objects (cells vs full spec)
- Estimated increase: 30-50% of cluster-resources directory size

This tradeoff is acceptable because:
1. Fidelity guarantee is more valuable than bundle size for debugging
2. Support bundles are typically compressed (tar.gz)
3. Table data compresses well (repetitive structure)

## Alternatives Considered

### Alternative 1: Dynamic Table Generation

Generate table output at serve time using Kubernetes' internal printers.

**Rejected because:**
- Requires typed objects for each resource (switch statements remain)
- Complex columns (e.g., Pod "Ready" = running/total containers) require type-specific logic
- Cannot achieve perfect fidelity without cluster access

### Alternative 2: Unstructured with Basic Tables

Serve all resources as unstructured, provide only NAME/NAMESPACE/AGE columns.

**Rejected because:**
- Violates fidelity requirement
- Debuggers lose valuable context from resource-specific columns
- UX degradation unacceptable for troubleshooting tool

### Alternative 3: Parse Error Messages for Selectable Fields

Discover selectable fields by making invalid requests and parsing error messages.

**Rejected because:**
- Fragile (error message format may change)
- Requires cluster access at serve time (defeats offline purpose)
- Shared mapping is more maintainable

## Open Questions

1. **CRD table columns**: Should we capture table representations for custom resources? (Recommended: yes, same approach applies)

2. **Bundle format versioning**: Should we add a version field to `_meta/` for future schema changes?

3. **Selective table collection**: Should table collection be opt-in to reduce bundle size for users who don't use sbctl?

## Success Criteria

1. sbctl can serve any resource troubleshoot collects without code changes
2. `kubectl get <resource>` output matches what a live cluster would return
3. Field selectors work for documented selectable fields
4. Old bundles continue to work with new sbctl
5. No switch statements for resource types in sbctl serving code

## References

- [sbctl repository](https://github.com/replicatedhq/sbctl)
- [Troubleshoot repository](https://github.com/replicatedhq/troubleshoot)
- [Kubernetes Table API](https://kubernetes.io/docs/reference/using-api/api-concepts/#receiving-resources-as-tables)
- [Kubernetes field selectors](https://kubernetes.io/docs/concepts/overview/working-with-objects/field-selectors/)
