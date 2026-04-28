# SPDY Executor Removal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace all `remotecommand.NewSPDYExecutor` call sites with a `k8sutil.NewFallbackExecutor` helper (WebSocket primary, SPDY fallback), and delete the unused `pkg/k8sutil/portforward.go`.

**Architecture:** A new `pkg/k8sutil/exec.go` provides an exported `NewFallbackExecutor` convenience function that wraps `NewWebSocketExecutor` + `NewSPDYExecutor` + `NewFallbackExecutor` from client-go. Each of the 7 existing call sites is updated in-place to call `k8sutil.NewFallbackExecutor` instead of `remotecommand.NewSPDYExecutor`. The dead `portforward.go` is deleted.

**Tech Stack:** Go, `k8s.io/client-go/tools/remotecommand`, `k8s.io/apimachinery/pkg/util/httpstream`

---

## File Map

| Action | File | Notes |
|--------|------|-------|
| Create | `pkg/k8sutil/exec.go` | New helper |
| Create | `pkg/k8sutil/exec_test.go` | Unit test for helper |
| Delete | `pkg/k8sutil/portforward.go` | Dead code, no callers |
| Modify | `pkg/collect/exec.go` | Add k8sutil import, swap line 140 |
| Modify | `pkg/collect/copy.go` | Add k8sutil import, swap line 110, update error string |
| Modify | `pkg/collect/copy_from_host.go` | k8sutil already imported; swap line 312, update error string |
| Modify | `pkg/collect/sonobuoy_results.go` | Add k8sutil import, swap line 150 |
| Modify | `pkg/collect/etcd.go` | Add k8sutil import, swap line 328 |
| Modify | `pkg/collect/longhorn.go` | Add k8sutil import, swap line 394 |
| Modify | `pkg/supportbundle/collect.go` | k8sutil already imported; swap line 370 |

---

## Task 1: Create `pkg/k8sutil/exec.go` and its test

**Files:**
- Create: `pkg/k8sutil/exec.go`
- Create: `pkg/k8sutil/exec_test.go`

- [ ] **Step 1: Write the failing test**

Create `pkg/k8sutil/exec_test.go`:

```go
package k8sutil

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	restclient "k8s.io/client-go/rest"
)

func TestNewFallbackExecutor(t *testing.T) {
	config := &restclient.Config{Host: "http://localhost:8080"}
	u, err := url.Parse("http://localhost:8080/api/v1/namespaces/default/pods/foo/exec")
	require.NoError(t, err)

	exec, err := NewFallbackExecutor(config, "POST", u)
	require.NoError(t, err)
	require.NotNil(t, exec)
}
```

- [ ] **Step 2: Run the test to confirm it fails**

```bash
go test ./pkg/k8sutil/... -run TestNewFallbackExecutor -v
```

Expected: compile error — `NewFallbackExecutor` undefined.

- [ ] **Step 3: Implement `pkg/k8sutil/exec.go`**

```go
package k8sutil

import (
	"net/url"

	"k8s.io/apimachinery/pkg/util/httpstream"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// NewFallbackExecutor creates an executor that tries WebSocket first and falls
// back to SPDY if the server does not support it. Use this in place of
// remotecommand.NewSPDYExecutor everywhere.
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

- [ ] **Step 4: Run the test to confirm it passes**

```bash
go test ./pkg/k8sutil/... -run TestNewFallbackExecutor -v
```

Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add pkg/k8sutil/exec.go pkg/k8sutil/exec_test.go
git commit -m "feat(k8sutil): add NewFallbackExecutor helper"
```

---

## Task 2: Update `pkg/collect/exec.go`

**Files:**
- Modify: `pkg/collect/exec.go`

- [ ] **Step 1: Add the k8sutil import**

In `pkg/collect/exec.go`, the import block currently ends with:

```go
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
```

Add the k8sutil import (keep `remotecommand` — it's still used for `remotecommand.StreamOptions`):

```go
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
```

- [ ] **Step 2: Swap the executor call**

In `pkg/collect/exec.go`, replace:

```go
	exec, err := remotecommand.NewSPDYExecutor(clientConfig, "POST", req.URL())
	if err != nil {
		return nil, nil, []string{err.Error()}
	}
```

With:

```go
	exec, err := k8sutil.NewFallbackExecutor(clientConfig, "POST", req.URL())
	if err != nil {
		return nil, nil, []string{err.Error()}
	}
```

- [ ] **Step 3: Verify build and tests**

```bash
go build ./pkg/collect/... && go test ./pkg/collect/... -run TestExec -v
```

Expected: builds cleanly, any existing exec tests pass.

- [ ] **Step 4: Commit**

```bash
git add pkg/collect/exec.go
git commit -m "refactor(collect): use fallback executor in exec collector"
```

---

## Task 3: Update `pkg/collect/copy.go`

**Files:**
- Modify: `pkg/collect/copy.go`

- [ ] **Step 1: Add the k8sutil import**

In `pkg/collect/copy.go`, the import block currently ends with:

```go
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
```

Add the k8sutil import (keep `remotecommand` — still used for `remotecommand.StreamOptions`):

```go
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
```

- [ ] **Step 2: Swap the executor call and update the error message**

In `pkg/collect/copy.go`, replace:

```go
	exec, err := remotecommand.NewSPDYExecutor(clientConfig, "POST", req.URL())
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create SPDY executor")
	}
```

With:

```go
	exec, err := k8sutil.NewFallbackExecutor(clientConfig, "POST", req.URL())
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create executor")
	}
```

- [ ] **Step 3: Verify build**

```bash
go build ./pkg/collect/...
```

Expected: builds cleanly.

- [ ] **Step 4: Commit**

```bash
git add pkg/collect/copy.go
git commit -m "refactor(collect): use fallback executor in copy collector"
```

---

## Task 4: Update `pkg/collect/copy_from_host.go`

**Files:**
- Modify: `pkg/collect/copy_from_host.go`

Note: `k8sutil` is already imported in this file (line 17). No import change needed.

- [ ] **Step 1: Swap the executor call and update the error message**

In `pkg/collect/copy_from_host.go`, replace:

```go
	exec, err := remotecommand.NewSPDYExecutor(clientConfig, "POST", req.URL())
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create SPDY executor")
	}
```

With:

```go
	exec, err := k8sutil.NewFallbackExecutor(clientConfig, "POST", req.URL())
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create executor")
	}
```

- [ ] **Step 2: Verify build**

```bash
go build ./pkg/collect/...
```

Expected: builds cleanly.

- [ ] **Step 3: Commit**

```bash
git add pkg/collect/copy_from_host.go
git commit -m "refactor(collect): use fallback executor in copy_from_host collector"
```

---

## Task 5: Update `pkg/collect/sonobuoy_results.go`

**Files:**
- Modify: `pkg/collect/sonobuoy_results.go`

- [ ] **Step 1: Add the k8sutil import**

In `pkg/collect/sonobuoy_results.go`, the import block currently contains:

```go
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
```

Add the k8sutil import (keep `remotecommand` — still used for `remotecommand.StreamOptions`):

```go
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
```

- [ ] **Step 2: Swap the executor call**

In `pkg/collect/sonobuoy_results.go`, replace:

```go
	executor, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
	if err != nil {
		return nil, ec, err
	}
```

With:

```go
	executor, err := k8sutil.NewFallbackExecutor(restConfig, "POST", req.URL())
	if err != nil {
		return nil, ec, err
	}
```

- [ ] **Step 3: Verify build**

```bash
go build ./pkg/collect/...
```

Expected: builds cleanly.

- [ ] **Step 4: Commit**

```bash
git add pkg/collect/sonobuoy_results.go
git commit -m "refactor(collect): use fallback executor in sonobuoy_results collector"
```

---

## Task 6: Update `pkg/collect/etcd.go`

**Files:**
- Modify: `pkg/collect/etcd.go`

- [ ] **Step 1: Add the k8sutil import**

In `pkg/collect/etcd.go`, the import block currently contains:

```go
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
```

Add the k8sutil import (keep `remotecommand` — still used for `remotecommand.StreamOptions`):

```go
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
```

- [ ] **Step 2: Swap the executor call**

In `pkg/collect/etcd.go`, replace:

```go
	exec, err := remotecommand.NewSPDYExecutor(c.clientConfig, "POST", req.URL())
	if err != nil {
		return nil, nil, err
	}
```

With:

```go
	exec, err := k8sutil.NewFallbackExecutor(c.clientConfig, "POST", req.URL())
	if err != nil {
		return nil, nil, err
	}
```

- [ ] **Step 3: Verify build**

```bash
go build ./pkg/collect/...
```

Expected: builds cleanly.

- [ ] **Step 4: Commit**

```bash
git add pkg/collect/etcd.go
git commit -m "refactor(collect): use fallback executor in etcd collector"
```

---

## Task 7: Update `pkg/collect/longhorn.go`

**Files:**
- Modify: `pkg/collect/longhorn.go`

- [ ] **Step 1: Add the k8sutil import**

In `pkg/collect/longhorn.go`, the import block currently contains:

```go
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
```

Add the k8sutil import (keep `remotecommand` — still used for `remotecommand.StreamOptions`):

```go
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
```

- [ ] **Step 2: Swap the executor call**

In `pkg/collect/longhorn.go`, replace:

```go
	executor, err := remotecommand.NewSPDYExecutor(clientConfig, "POST", req.URL())
	if err != nil {
		return "", errors.Wrapf(err, "create remote exec")
	}
```

With:

```go
	executor, err := k8sutil.NewFallbackExecutor(clientConfig, "POST", req.URL())
	if err != nil {
		return "", errors.Wrapf(err, "create remote exec")
	}
```

- [ ] **Step 3: Verify build**

```bash
go build ./pkg/collect/...
```

Expected: builds cleanly.

- [ ] **Step 4: Commit**

```bash
git add pkg/collect/longhorn.go
git commit -m "refactor(collect): use fallback executor in longhorn collector"
```

---

## Task 8: Update `pkg/supportbundle/collect.go`

**Files:**
- Modify: `pkg/supportbundle/collect.go`

Note: `k8sutil` is already imported in this file (line 20). No import change needed.

- [ ] **Step 1: Swap the executor call**

In `pkg/supportbundle/collect.go`, replace:

```go
	exec, err := remotecommand.NewSPDYExecutor(clientConfig, "POST", req.URL())
	if err != nil {
		return nil, nil, err
	}
```

With:

```go
	exec, err := k8sutil.NewFallbackExecutor(clientConfig, "POST", req.URL())
	if err != nil {
		return nil, nil, err
	}
```

- [ ] **Step 2: Verify build**

```bash
go build ./pkg/supportbundle/...
```

Expected: builds cleanly.

- [ ] **Step 3: Commit**

```bash
git add pkg/supportbundle/collect.go
git commit -m "refactor(supportbundle): use fallback executor in collect"
```

---

## Task 9: Delete dead code and verify full build

**Files:**
- Delete: `pkg/k8sutil/portforward.go`

- [ ] **Step 1: Delete the file**

```bash
git rm pkg/k8sutil/portforward.go
```

- [ ] **Step 2: Verify nothing imports the deleted function**

```bash
grep -r "k8sutil\.PortForward\b" . --include="*.go"
```

Expected: no output (zero callers).

- [ ] **Step 3: Run the full build and test suite**

```bash
make build && make test
```

Expected: both complete without errors.

- [ ] **Step 4: Commit**

```bash
git commit -m "chore(k8sutil): delete unused PortForward function"
```
