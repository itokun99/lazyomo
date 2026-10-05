# Refactoring Workflow - lazyomo

## Overview

This document defines the safe refactoring approach for lazyomo.

## Refactoring Principles

1. **Behavior Preservation**: Refactoring must not change external behavior
2. **Incremental Steps**: Small, verifiable changes
3. **Test Coverage**: Tests must pass before and after each step
4. **Pattern Consistency**: Follow existing codebase patterns

## Refactoring Process

### Phase 1: Analyze

```
1. Understand current implementation
   - Read the code thoroughly
   - Understand all dependencies
   - Note all callers/users

2. Identify refactoring goal
   - What's wrong with current design?
   - What should it look like after?
   - Is refactoring necessary?

3. Assess risk
   - How many files affected?
   - How many tests affected?
   - What could break?
```

### Phase 2: Plan

```
1. Create step-by-step plan
   - Each step should be verifiable
   - Each step should leave code working
   - Order steps to minimize risk

2. Identify test requirements
   - What tests need updating?
   - What new tests needed?
   - What tests verify behavior preservation?

3. Estimate effort
   - Simple refactor: 1-2 files
   - Medium refactor: 3-5 files
   - Large refactor: 6+ files (consider breaking up)
```

### Phase 3: Execute

```
1. Ensure tests pass initially
   go test ./...

2. Make one small change
   - Extract function
   - Rename variable
   - Move code
   - etc.

3. Run tests after each change
   go test ./...

4. Repeat until complete
```

### Phase 4: Verify

```
1. Run all tests
   go test ./...

2. Run vet
   go vet ./...

3. Manual verification
   - Test affected features
   - Verify no behavior changes
```

## Safe Refactoring Patterns

### Pattern 1: Extract Function

**Before**:
```go
func (e *Editor) Sections() []Section {
    secs := e.sectionOrder()
    out := make([]Section, 0, len(secs))
    for _, id := range secs {
        entries, ro := e.entriesFor(id)
        out = append(out, Section{ID: id, Title: string(id), Entries: entries, ReadOnly: ro})
    }
    return out
}
```

**After**:
```go
func (e *Editor) Sections() []Section {
    out := make([]Section, 0, len(e.sectionOrder()))
    for _, id := range e.sectionOrder() {
        out = append(out, e.buildSection(id))
    }
    return out
}

func (e *Editor) buildSection(id SectionID) Section {
    entries, ro := e.entriesFor(id)
    return Section{ID: id, Title: string(id), Entries: entries, ReadOnly: ro}
}
```

### Pattern 2: Extract Interface

The TUI already uses this: `internal/tui/model.go` declares the `Editor` interface it needs, and `cmd/lazyomo` wires the real `*editor.Editor`. Tests pass a fake. When new UI needs surface, extend that consumer-side interface first, then implement on `*editor.Editor`.

```go
// in internal/tui/model.go
type Editor interface {
    DirtyPaths() []string
    Save() (string, error)
    Reload() error
    // ...
}
```

### Pattern 3: Move Function to the Right Package

File writes belong in `omodit`, scope rules in `editor`, rendering in `tui`. If save or backup logic drifts into `editor`, move it down into `omodit.Document.Save`. If a validation rule drifts into the TUI, move it up into `internal/editor/validate.go` so bad input still leaves the document untouched.

### Pattern 4: Rename for Clarity

**Before**:
```go
func (e *Editor) Set(path, value string) error {
```

**After**:
```go
func (e *Editor) SetScalar(path, value string) error {
```

## Refactoring Checklist

Before starting:

- [ ] All tests pass
- [ ] Understand all callers/users
- [ ] Have step-by-step plan
- [ ] Know what tests verify behavior

During refactoring:

- [ ] One small change at a time
- [ ] Run tests after each change
- [ ] Don't change behavior
- [ ] Follow existing patterns

After refactoring:

- [ ] All tests pass
- [ ] No vet warnings
- [ ] Build succeeds
- [ ] Behavior preserved
- [ ] Code is cleaner

## Common Refactoring Scenarios

### Scenario 1: Duplicate Code

**Problem**: Same logic in multiple places

**Solution**: Extract to shared function

**Risk**: Low (if tests exist for both locations)

**Example**:
```go
// Before: pointer parsing copied in two editor methods
func (e *Editor) SetScalar(path, value string) error {
    tokens, err := parsePointer(path)
    // ...
}

// After: one shared helper beside the editor
func parsePointer(pointer string) ([]string, error)
```

### Scenario 2: Long Function

**Problem**: Function does too many things

**Solution**: Extract sub-functions

**Risk**: Medium (need to verify extracted functions work correctly)

**Example**:
```go
// Before: 50-line Sections builder
func (e *Editor) Sections() []Section {
    // ... 50 lines
}

// After: composed of smaller functions
func (e *Editor) Sections() []Section {
    out := make([]Section, 0, len(e.sectionOrder()))
    for _, id := range e.sectionOrder() {
        out = append(out, e.buildSection(id))
    }
    return out
}
```

### Scenario 3: Interface Extraction

**Problem**: Concrete type used where interface would be better

**Solution**: Define interface, change parameter type

**Risk**: Low (compile-time checks catch errors)

**Example**:
```go
// Before: TUI holds a concrete *editor.Editor, tests can't fake it
// After: TUI declares Editor interface in model.go, cmd wires the real one
func New(ed Editor) *Model
```

### Scenario 4: Move Code to Better Package

**Problem**: Code in wrong package

**Solution**: Move to correct package, update imports

**Risk**: Medium (need to update all callers)

**Example**:
```go
// Before: chain validation inline in internal/tui/update.go
// After: in internal/editor/validate.go, TUI renders the returned error
```

## Refactoring Anti-Patterns

### Anti-Pattern 1: Refactor + Feature

**Wrong**: Add new feature while refactoring

**Right**: Refactor first, then add feature

### Anti-Pattern 2: Big Bang Refactor

**Wrong**: Change everything at once

**Right**: Small, incremental changes

### Anti-Pattern 3: Skip Tests

**Wrong**: Refactor without running tests

**Right**: Run tests after every change

### Anti-Pattern 4: Change Behavior

**Wrong**: "Improve" behavior while refactoring

**Right**: Preserve exact behavior

## Large Refactoring Strategy

For refactoring that touches 6+ files:

1. **Break into smaller refactorings**
   - Each should be independently valuable
   - Each should be independently verifiable

2. **Create feature branch**
   - Isolate refactoring from other work
   - Easy to abandon if problems arise

3. **Commit frequently**
   - Each commit should be working state
   - Easy to revert specific changes

4. **Review carefully**
   - Have someone review the refactoring
   - Or use AI review tools

## Refactoring Tools

### go vet

```bash
go vet ./...
```

### Tests

```bash
go test ./...
go test -race ./...
```

### Build

```bash
go build ./cmd/lazyomo
```

### Manual Testing

Test affected features after refactoring.
