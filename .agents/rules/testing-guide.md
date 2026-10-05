# Testing Guide - lazyomo

## Overview

This document defines testing standards for the lazyomo project.

## Test Framework

**Standard library `testing` only** - no testify, gomega, or external frameworks.

```go
import "testing"
```

## Test File Organization

| Package | File | Notes |
|-------|------|-------|
| omodit | `omodit_test.go` plus `testdata/` | Engine: parse, pointers, save plus backup |
| editor | `editor_test.go` plus `testdata/` | Surface: sections, validation, dirty set |
| tui | `tui_test.go` | UI with a fake Editor (see `Editor` interface in model.go) |

## Test Naming Conventions

```go
func TestLoad(t *testing.T)                          // Constructor
func TestDocument_Set(t *testing.T)                  // Engine method
func TestEditor_SetScalar_InvalidModel(t *testing.T) // Surface rule
func TestModel_Update(t *testing.T)                  // TUI behavior
```

## Table-Driven Tests

### Pattern

```go
func TestFoo(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {
            name:    "valid input",
            input:   "test",
            want:    "result",
            wantErr: false,
        },
        {
            name:    "invalid input",
            input:   "",
            want:    "",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Foo(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Foo() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("Foo() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Fake Pattern

The TUI tests use a fake `Editor` (the interface in `internal/tui/model.go`), not the real `*editor.Editor`. Hand-write it, keep it small, and add a compile-time check:

```go
var _ Editor = (*fakeEditor)(nil)
```

Error injection means returning errors from fake methods (save failure, blocked delete) and asserting the popup or status text.

## Test Helpers

### Helper Function Pattern

```go
func writeTestFile(t *testing.T, path, content string) {
    t.Helper()
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
        t.Fatal(err)
    }
}
```

### Editor Constructor for Tests

```go
// Load a temp copy through the editor, never the live file
dir := t.TempDir()
writeTestFile(t, filepath.Join(dir, "omo.jsonc"), fixture)
ed, err := editor.Load(filepath.Join(dir, "omo.jsonc"))
```

## Fixtures

### Inline Fixtures

```go
var (
    validJSON   = []byte(`{"agents": {"sisyphus": {}}}`)
    invalidJSON = []byte(`{"invalid": true}`)
)
```

### Temporary Directories

```go
func TestEditor_SetScalar(t *testing.T) {
    dir := t.TempDir()
    writeTestFile(t, filepath.Join(dir, "omo.jsonc"), `{...}`)

    ed, err := editor.Load(filepath.Join(dir, "omo.jsonc"))
    // ...
}
```

## Coverage

### Running Coverage

```bash
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Coverage Expectations

- omodit engine: 90%+ coverage expected
- editor surface: 90%+ coverage expected
- tui: 70%+ coverage expected

## Testing Each Package

### Omodit Tests

```go
func TestDocument_Set(t *testing.T) {
    tests := []struct {
        name      string
        pointer   string
        value     any
        wantErr   bool
    }{
        {name: "scalar", pointer: "/models/k3/reasoning", value: "high"},
        {name: "missing path errors", pointer: "/models/nope/model", value: "x", wantErr: true},
    }
    // Load fixture from testdata or temp dir, apply op, compare Bytes or Get
}

## Running Tests

```bash
# All tests
go test ./...

# Specific package
go test ./internal/editor/...

# Verbose
go test -v ./...

# Specific test
go test -run TestEditor_SetScalar ./internal/editor/...

# With coverage
go test -cover ./...

# With race detector
go test -race ./...
```

## Common Patterns

### Testing Error Cases

```go
t.Run("error case", func(t *testing.T) {
    err := ed.SetScalar("/models/bad alias!/model", "x")
    if err == nil {
        t.Error("expected error, got nil")
    }
    if len(ed.DirtyPaths()) != 0 {
        t.Error("failed edit must leave the dirty set empty")
    }
})
```

### Testing with Temp Directories

```go
func TestFoo(t *testing.T) {
    dir := t.TempDir()
    // Use dir for test files
    // Auto-cleaned after test
}
```

### Testing JSONC Content

```go
var validFixture = []byte(`{
    // comment preserved by omodit
    "agents": {
        "sisyphus": {
            "model": "acme/code-large"
        }
    }
}`)
```
