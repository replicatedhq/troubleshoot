# Design: Replace SPDY Executor with FallbackExecutor

**Date:** 2026-04-28
**Status:** Approved

## Background

`remotecommand.NewSPDYExecutor` is no longer the recommended way to execute commands in pods. The upstream recommendation is to use WebSocket as the primary transport with SPDY as a fallback for older clusters, via `remotecommand.NewFallbackExecutor`. This is the pattern used by `kubectl exec` itself as of Kubernetes 1.29+.

The codebase has 7 call sites still using `NewSPDYExecutor` directly. Additionally, `pkg/k8sutil/portforward.go` contains a `PortForward()` function built on `transport/spdy` that is never called anywhere and can be deleted.

## Scope

Three changes, in order:

1. **New helper** — `pkg/k8sutil/exec.go`
2. **7 in-place call site updates** — `pkg/collect/` and `pkg/supportbundle/`
3. **Dead code removal** — delete `pkg/k8sutil/portforward.go`

## New Helper: `pkg/k8sutil/exec.go`

A single exported function that wraps the three-step construction into one call:

```go
package k8sutil

import (
    "net/url"

    "k8s.io/apimachinery/pkg/util/httpstream"
    restclient "k8s.io/client-go/rest"
    "k8s.io/client-go/tools/remotecommand"
)

func NewFallbackExecutor(config *restclient.Config, method string, url *url.URL) (remotecommand.Executor, error) {
    wsExec, err := remotecommand.NewWebSocketExecutor(config, method, url.String())
    if err != nil {
        return nil, err
    }
    spdyExec, err := remotecommand.NewSPDYExecutor(config, method, url)
    if err != nil {
        return nil, err
    }
    return remotecommand.NewFallbackExecutor(wsExec, spdyExec, httpstream.IsUpgradeFailure)
}
```

Exported because `pkg/k8sutil` is a shared utility package consumed by multiple packages.

## Call Site Updates

Each of the 7 call sites replaces one line with one line:

```go
// before
exec, err := remotecommand.NewSPDYExecutor(clientConfig, "POST", req.URL())

// after
exec, err := k8sutil.NewFallbackExecutor(clientConfig, "POST", req.URL())
```

Files to update:

| File | Line |
|------|------|
| `pkg/collect/exec.go` | 140 |
| `pkg/collect/copy.go` | 110 |
| `pkg/collect/copy_from_host.go` | 312 |
| `pkg/collect/sonobuoy_results.go` | 150 |
| `pkg/collect/etcd.go` | 328 |
| `pkg/collect/longhorn.go` | 394 |
| `pkg/supportbundle/collect.go` | 370 |

Each file needs the `k8sutil` import added if not already present, and the `remotecommand` import removed if it was only used for `NewSPDYExecutor`.

## Dead Code Removal

Delete `pkg/k8sutil/portforward.go` in full. The exported `PortForward()` function is never called anywhere in the codebase. The `transport/spdy` and `tools/portforward` imports go away with it.

## Out of Scope

- `copy.go` and `copy_from_host.go` use the deprecated `exec.Stream()` instead of `exec.StreamWithContext()`. This is a separate concern and not addressed here.
- No changes to port-forward behavior elsewhere — there are no active port-forward call sites using `transport/spdy`.
