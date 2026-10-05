# Bug Fix Workflow - lazyomo

## Overview

This document defines the systematic approach to investigating and fixing bugs in lazyomo.

## Bug Fix Process

### Phase 1: Reproduce

```
1. Understand the bug report
   - What happened?
   - What was expected?
   - Steps to reproduce?

2. Create minimal reproduction
   - Use editor or omodit tests for easier debugging
   - Copy ~/.omo/omo.jsonc to a temp file, never the live one
   - Isolate the issue to one op: Get, Set, ToggleBool, Add, Remove, Save

3. Verify the bug exists
   - Run reproduction steps
   - Confirm unexpected behavior
```

### Phase 2: Investigate

```
1. Trace execution path
   - Start from entry point (main.go)
   - Follow the call chain
   - Identify where behavior diverges

2. Check package boundaries
   - Is the issue in editor validation or scope rules?
   - Is it in omodit parsing, pointers, save, or backup?
   - Is it in TUI focus, overlay, or rendering?
   - Is it in the cmd/lazyomo launcher?

3. Identify root cause
   - Don't just find symptoms
   - Find the actual source of the problem
```

### Phase 3: Fix

```
1. Create fix proposal
   - What needs to change?
   - What files are affected?
   - What are the side effects?

2. Implement minimal fix
   - Change only what's necessary
   - Don't refactor while fixing
   - Follow existing patterns

3. Write regression test
   - Test that reproduces the bug
   - Verify fix prevents recurrence
```

### Phase 4: Verify

```
1. Run affected tests
   go test ./internal/affected_package/...

2. Run all tests
   go test ./...

3. Manual verification
   - Test the original reproduction steps
   - Verify fix works as expected
```

## Debugging Techniques

### Launcher Debugging

```bash
# Help text
./lazyomo --help

# Check exit code
echo $?
```

v1 has no other flags. Reproduce editor behavior in tests, not through CLI verbs (none exist).

### TUI Debugging

The TUI needs a terminal, so isolate first: reproduce the state change through `internal/editor` on a temp copy, then check focus plus overlay handling in `internal/tui` (`model.go`, `update.go`, `view.go`).

### Adding Debug Output

Temporary debug output (remove before commit):

```go
fmt.Fprintf(os.Stderr, "DEBUG: variable = %v\n", variable)
```

### Using Tests for Debugging

```go
func TestDebugIssue(t *testing.T) {
    // Setup: copy a fixture to a temp file, load through the editor
    dir := t.TempDir()
    // write fixture, then:
    // ed, err := editor.Load(filepath.Join(dir, "omo.jsonc"))
    // Test one op, for example ed.SetScalar("/models/k3/reasoning", "high")
    // and log the returned inline error
}
```

## Common Bug Categories

### Category 1: Load and Validation Issues

**Symptoms**: Launch fails, edit rejected, save refuses to write

**Investigation**:
```go
// Load a temp copy through the editor and try one op
// ed, _ := editor.Load(tmpPath)
// err := ed.SetScalar("/models/k3/reasoning", "high")
// fmt.Printf("Err: %v\n", err)
```

**Common causes**:
- Missing ~/.omo/omo.jsonc
- Invalid JSONC syntax
- Alias or `provider/model-id` shape wrong
- Unknown category name or missing profile for `model_profile`

### Category 2: Launcher Issues

**Symptoms**: Wrong output, missing usage, wrong exit code

**Common causes**:
- Unknown args not echoing usage
- Help text out of date with main.go

### Category 3: TUI Issues

**Symptoms**: UI not rendering, wrong focus, stuck popup, lost dirty marks

**Investigation**:
- Reproduce the edit in `internal/editor` first to rule out validation
- Check pane focus plus overlay exclusivity in update.go
- Check dirty rendering and status text in view.go

**Common causes**:
- Popup not taking all input
- Focus order wrong across Sections, Entries, Detail
- Status line not reflecting DirtyPaths

### Category 4: Save and Backup Issues

**Symptoms**: No backup sibling, partial write, permissions error

**Investigation**:
```go
// Check the live path and its sibling backups (read-only inspection)
// _, err := os.Stat(filepath.Join(home, ".omo", "omo.jsonc"))
```

**Common causes**:
- Saving while clean (by design: no backup, no write)
- Directory not writable
- Temp file plus rename interrupted

## Bug Fix Checklist

Before submitting fix:

- [ ] Bug reproduced and understood
- [ ] Root cause identified (not just symptom)
- [ ] Minimal fix implemented
- [ ] Regression test written
- [ ] All tests pass (`go test ./...`)
- [ ] No package boundary violations
- [ ] Existing patterns followed
- [ ] No unnecessary refactoring

## Example Bug Fix

### Bug Report
"Toggling telemetry.enabled reports `not a bool` on a bool row"

### Investigation

```go
// 1. Reproduce through the editor on a temp copy
// err := ed.ToggleBool("/telemetry/enabled")
// 2. Check: is the pointer right, is the stored value a JSON bool,
//    does Detail classify the row as KindBool?
// 3. Fix at the layer at fault: classification in editor,
//    key handling in tui/update.go, or display in tui/view.go.
```

### Fix

```go
// Keep the fix minimal: correct the single misclassified path or key branch.
// Validation stays before mutation, so bad input still leaves the doc untouched.
```

### Regression Test

```go
func TestToggleTelemetryEnabled(t *testing.T) {
    // Load fixture with telemetry.enabled=false, ToggleBool, expect true
    // plus the path in DirtyPaths; Save then Reload round-trips it.
}
```

## Debugging Tools

### go vet

```bash
go vet ./...
```

### Race Detector

```bash
go test -race ./...
```

### Verbose Tests

```bash
go test -v ./internal/editor/...
```

### Specific Test

```bash
go test -run TestToggleTelemetry ./internal/editor/...
```
