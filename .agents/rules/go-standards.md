# Go Engineering Standards - lazyomo

## Overview

This document defines Go coding standards specific to the lazyomo project.

## Package Design

### Package Responsibilities

| Package | Responsibility | I/O | Dependencies |
|---------|---------------|-----|--------------|
| omodit | JSONC engine, atomic save plus backup | Filesystem (one file) | hujson only |
| editor | Editable surface, validation, dirty set | Via omodit | omodit |
| tui | Bubble Tea editor UI | stdin/stdout | editor types (consumer-side interface) |

### Package Boundaries

```
cmd/lazyomo -> internal/tui -> internal/editor -> internal/omodit
editor is the only package that imports omodit
tui declares the Editor interface it needs in model.go, never imports omodit
```

### Package Naming

- Use lowercase, single-word names
- Avoid `utils`, `helpers`, `common`
- Package name matches directory name

## Error Handling

### Error Creation

```go
// Always wrap with context
fmt.Errorf("saving config: %w", err)

// Pattern: "verb-ing noun: %w"
fmt.Errorf("loading config %s: %w", path, err)
fmt.Errorf("setting %q: %w", pointer, err)
fmt.Errorf("backup failed: %w", err)
```

### Error Propagation

```
omodit → editor → tui / cmd
  ↓         ↓          ↓
wrap system validate   user-facing
errors   before mutate message or status line
```

### User-Facing Errors

```go
// cmd/lazyomo
fmt.Fprintf(os.Stderr, "Error: %v\n", err)
os.Exit(1)

// TUI: inline in Detail or popup, short note in status; dirty marks stay on failure
```

### Graceful Degradation

```go
// Use os.IsNotExist for optional resources
if os.IsNotExist(err) {
    return emptyResult, nil
}
```

## Logging

### Logging Library

Use `log/slog` (standard library):

```go
slog.Error("save failed", "path", path, "error", err)
```

### Logging Levels

- `slog.Error` - Errors that need attention
- `slog.Warn` - Potential issues
- `slog.Info` - Important operations
- `slog.Debug` - Detailed debugging

### Logging Context

```go
slog.Error("operation failed",
    "operation", "save",
    "error", err,
)
```

## Configuration

### Config Paths

```go
// Single live file plus sibling backups
home, _ := os.UserHomeDir()
path := filepath.Join(home, ".omo", "omo.jsonc")
// backup: <config>.bak.<UTC timestamp>, for example
// omo.jsonc.bak.2026-10-05T12-34-56-789Z
```

Credentials under `~/.omo/agent/` are never read or written.

### JSON Pointers

```go
// RFC 6901 pointers into the live config
doc.Get("/models/k3/reasoning")
doc.Set("/agents/sisyphus/model", "acme/code-large")
```

## Struct Design

### Constructor Pattern

```go
type Foo struct {
    field1 string
    field2 int
}

func NewFoo(field1 string, field2 int) *Foo {
    return &Foo{
        field1: field1,
        field2: field2,
    }
}
```

### Validate Before Mutate

```go
// editor rejects bad path or value before touching the document:
// invalid input returns an inline-displayable error, file untouched
func (e *Editor) SetScalar(path, value string) error
func (e *Editor) ToggleBool(path string) error
```

## Interface Design

### Interface Location

Define interfaces in the package that USES them:

```go
// internal/tui/model.go declares what the UI needs
type Editor interface {
    Path() string
    Sections() []editor.Section
    SetScalar(path, value string) error
    ToggleBool(path string) error
    AddEntry(section editor.SectionID, key string) error
    RemoveEntry(section editor.SectionID, key string) error
    DirtyPaths() []string
    Save() (string, error)
    Reload() error
}
```

### Compile-Time Checks

```go
var _ Editor = (*editor.Editor)(nil)
```

## Naming Conventions

### Variables

- Use camelCase for local variables
- Use descriptive names (not single letters except loops)
- Avoid abbreviations unless well-known

### Functions

- Use CamelCase for exported functions
- Use camelCase for unexported functions
- Prefix launcher functions with `cmd` plus `run` in cmd/lazyomo only

### Constants

- Use CamelCase for exported constants
- Use camelCase for unexported constants

### Packages

- Use lowercase, single-word names
- Avoid underscores in package names

## Code Organization

### File Structure

```go
package foo

// 1. Imports
import (
    "standard"
    "third-party"
    "internal"
)

// 2. Constants
const (
    // ...
)

// 3. Types
type Foo struct {
    // ...
}

// 4. Constructor
func NewFoo() *Foo {
    // ...
}

// 5. Methods
func (f *Foo) Bar() {
    // ...
}
```

### Import Grouping

```go
import (
    // Standard library
    "fmt"
    "os"

    // Third-party
    "github.com/charmbracelet/bubbletea"

    // Internal
    "github.com/itokun99/lazyomo/internal/editor"
)
```

## Testing

### Test File Naming

- Same directory as source: `foo_test.go`
- Same package for unexported access
- External package (`_test` suffix) for black-box testing

### Test Function Naming

```go
func TestFoo(t *testing.T)           // Simple
func TestFoo_Bar(t *testing.T)       // Method
func TestFoo_Bar_Scenario(t *testing.T) // Specific case
```

### Table-Driven Tests

```go
func TestFoo(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {name: "valid", input: "a", want: "b", wantErr: false},
        {name: "invalid", input: "", want: "", wantErr: true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Foo(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Foo() error = %v, wantErr %v", err, tt.wantErr)
            }
            if got != tt.want {
                t.Errorf("Foo() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Build and Run

### Build

```bash
go build -o lazyomo ./cmd/lazyomo
```

### Run Tests

```bash
go test ./...
go test -cover ./...
go test -v ./internal/editor/...
```

### Lint

```bash
go vet ./...
```

## Dependencies

### Adding Dependencies

```bash
go get github.com/package/name
go mod tidy
```

### Dependency Rules

- Prefer standard library when possible
- Use well-maintained packages
- Avoid dependencies with many transitive deps
- Document why each dependency is needed
