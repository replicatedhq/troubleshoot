# Proposal: Capture All Collector Errors

**Status**: Draft
**Author**: Analysis Team
**Date**: 2025-10-03
**Related Issues**: Collectors fail silently, errors are lost

---

## Executive Summary

Collectors currently lose error information because:
1. **stderr is discarded** when commands succeed (exit code 0)
2. **stderr is sometimes not saved** even when commands fail
3. **Exit codes and error messages** aren't consistently saved

This proposal ensures **all error information is captured and saved** in the support bundle.

---

## Problem Statement

When collectors run commands, errors can occur that users never see:

### Example 1: sysctl with permissions errors
```bash
$ sysctl -a
sysctl: permission denied on key 'fs.protected_fifos'  # ← Written to stderr
sysctl: permission denied on key 'fs.protected_hardlinks'  # ← Written to stderr
kernel.ostype = Linux  # ← Written to stdout
```
**Current behavior**: Only stdout is saved. Users never see the permission errors.

### Example 2: journalctl returns empty
```bash
$ journalctl -u kubelet
-- No entries --
```
**Current behavior**: No way to know if this is because:
- Service doesn't exist
- No permissions to read logs
- Service genuinely has no logs

### Example 3: find with permission errors
```bash
$ find /lib/modules -name "*.ko"
find: '/lib/modules/5.15.0/kernel/crypto': Permission denied  # ← stderr
/lib/modules/5.15.0/kernel/fs/ext4.ko  # ← stdout
```
**Current behavior**: Only stdout is saved. Users don't know directories were skipped.

---

## Root Cause

Collectors use `cmd.Output()` which **only captures stdout**:

```go
// Current code in host_sysctl.go
cmd := exec.Command("sysctl", "-a")
out, err := cmd.Output()  // ← Only captures stdout, stderr is lost!
```

Even when collectors capture stderr, they often don't save it:

```go
// Current code in host_journald.go
var stdout, stderr bytes.Buffer
cmd.Stdout = &stdout
cmd.Stderr = &stderr

if err := cmd.Run(); err != nil {
    // stderr only saved in error case
    cmdInfo.Error = stderr.String()
}
// If exit code is 0, stderr is never saved even if it contains warnings
```

---

## Proposed Solution

**Principle**: Capture and save ALL error information - stderr, exit codes, and error messages.

### Approach 1: Save stderr to separate files (Preferred)

For collectors that need clean stdout for parsing:

```go
// Example: host_sysctl.go
var stdout, stderr bytes.Buffer
cmd.Stdout = &stdout
cmd.Stderr = &stderr
err := cmd.Run()

// Save stdout as normal
values := parseSysctlParameters(stdout.Bytes())
payload, _ := json.Marshal(values)
output.SaveResult(c.BundlePath, "host-collectors/system/sysctl.json", bytes.NewBuffer(payload))

// NEW: Always save stderr if it exists
if stderr.Len() > 0 {
    output.SaveResult(c.BundlePath, "host-collectors/system/sysctl-stderr.txt", bytes.NewBuffer(stderr.Bytes()))
}

// NEW: Save error info if command failed
if err != nil {
    errorInfo := map[string]string{
        "error": err.Error(),
        "stderr": stderr.String(),
    }
    errorJSON, _ := json.Marshal(errorInfo)
    output.SaveResult(c.BundlePath, "host-collectors/system/sysctl-error.json", bytes.NewBuffer(errorJSON))
}
```

### Approach 2: Always save command metadata

For all collectors using exec commands:

```go
// Save metadata about every command execution
type CommandInfo struct {
    Command  string `json:"command"`
    ExitCode int    `json:"exitCode"`
    Error    string `json:"error,omitempty"`
    Stderr   string `json:"stderr,omitempty"`
    Duration int64  `json:"duration,omitempty"`
}

// After running any command:
cmdInfo := CommandInfo{
    Command: cmd.String(),
    ExitCode: 0,
}

if exitErr, ok := err.(*exec.ExitError); ok {
    cmdInfo.ExitCode = exitErr.ExitCode()
}

if stderr.Len() > 0 {
    cmdInfo.Stderr = stderr.String()
}

if err != nil {
    cmdInfo.Error = err.Error()
}

// Always save the metadata
infoJSON, _ := json.Marshal(cmdInfo)
output.SaveResult(c.BundlePath, "host-collectors/system/sysctl-info.json", bytes.NewBuffer(infoJSON))
```

---

## Implementation Plan

### Files to Change

| File | Current Issue | Fix |
|------|--------------|-----|
| `host_sysctl.go` | Uses `cmd.Output()`, loses stderr | Capture stderr separately, save to file |
| `host_kernel_modules.go` | Uses `cmd.Output()`, loses stderr | Capture stderr separately, save to file |
| `host_journald.go` | Captures stderr but only saves on error | Always save stderr when non-empty |
| `host_filesystem_performance.go` | Shows stderr in error but doesn't save to bundle | Save stderr to file |
| `host_run.go` | Generic runner, inconsistent error handling | Standardize error capture |

### Example Changes

#### host_sysctl.go
```go
func (c *CollectHostSysctl) Collect() (map[string][]byte, error) {
    output := NewResult()

    // NEW: Capture both stdout and stderr
    cmd := execCommand("sysctl", "-a")
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    err := cmd.Run()

    // Parse stdout as normal
    values := parseSysctlParameters(stdout.Bytes())
    payload, _ := json.Marshal(values)
    output.SaveResult(c.BundlePath, HostSysctlPath, bytes.NewBuffer(payload))

    // NEW: Save stderr if present
    if stderr.Len() > 0 {
        stderrPath := strings.TrimSuffix(HostSysctlPath, ".json") + "-stderr.txt"
        output.SaveResult(c.BundlePath, stderrPath, bytes.NewBuffer(stderr.Bytes()))
    }

    // NEW: Save error info if command failed
    if err != nil {
        errorInfo := map[string]string{
            "command": "sysctl -a",
            "error": err.Error(),
            "stderr": stderr.String(),
        }
        errorJSON, _ := json.Marshal(errorInfo)
        errorPath := strings.TrimSuffix(HostSysctlPath, ".json") + "-error.json"
        output.SaveResult(c.BundlePath, errorPath, bytes.NewBuffer(errorJSON))
    }

    return output, nil
}
```

#### host_journald.go
```go
// After line 95 (where stdout is saved), add:
if stderr.Len() > 0 {
    // NEW: Always save stderr, not just on error
    stderrFileName := filepath.Join(HostJournaldPath, collectorName+"-stderr.txt")
    output.SaveResult(c.BundlePath, stderrFileName, bytes.NewBuffer(stderr.Bytes()))
}
```

---

## Bundle Structure After Changes

```
support-bundle-XXX/
  host-collectors/
    system/
      sysctl.json              # Normal output
      sysctl-stderr.txt        # NEW: stderr output (if any)
      sysctl-error.json        # NEW: error details (if failed)
    run-host/
      journalctl-kubelet.txt
      journalctl-kubelet-info.json    # Already exists
      journalctl-kubelet-stderr.txt   # NEW: stderr warnings
```

---

## Benefits

**Before**:
- ❌ stderr lost when exit code is 0
- ❌ No indication of partial data
- ❌ Users don't know what errors occurred

**After**:
- ✅ All stderr captured and saved
- ✅ Exit codes and error messages saved
- ✅ Users can see exactly what went wrong

---

## Testing

Run support bundle without sudo and verify error files are created:

```bash
./support-bundle collect examples/host/default.yaml

# Check that error information was captured:
ls support-bundle-*/host-collectors/system/*stderr*
ls support-bundle-*/host-collectors/system/*error*
ls support-bundle-*/host-collectors/run-host/*stderr*

# Verify content:
cat support-bundle-*/host-collectors/system/sysctl-stderr.txt
# Should show: "sysctl: permission denied on key 'fs.protected_fifos'"
```

---

## Backwards Compatibility

All changes are additive:
- Existing files remain unchanged
- New `-stderr.txt` and `-error.json` files are added
- Tools that parse existing files continue to work
- Tools can optionally use new error files for better diagnostics

---

## Summary

This proposal ensures all collector errors are captured by:
1. **Always capturing stderr** (not just stdout)
2. **Always saving stderr** when non-empty (not just on failure)
3. **Saving error metadata** (exit codes, error messages)

**Total effort**: ~30 lines of code across 5 files.

**Result**: Users can see all errors that occurred during collection.