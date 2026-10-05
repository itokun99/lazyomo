# Contributing Guide - lazyomo

## Welcome

This guide helps new developers (human or AI) start contributing to lazyomo.

## Prerequisites

- Go 1.26.3 or later
- Git
- Terminal (for TUI testing)

## Getting Started

### 1. Clone Repository

```bash
git clone https://github.com/itokun99/lazyomo.git
cd lazyomo
```

### 2. Build

```bash
go build -o lazyomo ./cmd/lazyomo
```

### 3. Run Tests

```bash
go test ./...
```

### 4. Run Application

```bash
# TUI editor (default, edits ~/.omo/omo.jsonc)
./lazyomo

# Help
./lazyomo --help
```

## Project Structure

```
lazyomo/
├── cmd/lazyomo/          # Entry point (open editor or --help)
│   └── main.go
├── internal/
│   ├── omodit/              # JSONC engine (Load, Get, Set, Add, Remove, Save)
│   ├── editor/              # Editable surface, validation, dirty set
│   └── tui/                 # Bubble Tea editor UI (model, update, view, styles)
├── docs/                    # spec-tui-v1.md (UI), spec-editor-v1.md (scope)
├── .agents/rules/           # AI agent documentation
└── AGENTS.md                # Master AI rules
```

## Development Workflow

### 1. Pick a Task

- Check issues for bugs/features
- Or identify improvement area

### 2. Create Branch

```bash
git checkout -b feature/my-feature
# or
git checkout -b fix/my-fix
```

### 3. Implement

Follow the patterns:
- Read AGENTS.md for architecture rules
- Read relevant documentation in .agents/rules/
- Find similar existing code
- Copy patterns, don't invent new ones

### 4. Test

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./internal/editor/... ./internal/omodit/...

# Run with coverage
go test -cover ./...

# Run with race detector
go test -race ./...
```

### 5. Verify

```bash
# Lint
go vet ./...

# Build
go build -o lazyomo ./cmd/lazyomo

# Manual test
./lazyomo --help
./lazyomo   # needs ~/.omo/omo.jsonc present
```

### 6. Commit

```bash
git add .
git commit -m "feat: add new feature"
# or
git commit -m "fix: fix bug description"
```

### 7. Push and PR

```bash
git push origin feature/my-feature
```

## Code Style

### Go Standards

- Follow Go conventions (gofmt, go vet)
- Use table-driven tests
- Follow existing patterns
- Don't add dependencies without strong reason

### Naming

- Package names: lowercase, single word
- Function names: CamelCase (exported), camelCase (unexported)
- Variable names: camelCase
- Constants: CamelCase (exported), camelCase (unexported)

### Error Handling

```go
// Wrap errors with context
if err != nil {
    return fmt.Errorf("doing something: %w", err)
}

// User-facing errors
fmt.Fprintf(w, "Error: %v\n", err)
return 1
```

### Testing

```go
// Table-driven tests
func TestFoo(t *testing.T) {
    tests := []struct {
        name  string
        input string
        want  string
    }{
        {name: "case 1", input: "a", want: "b"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Foo(tt.input)
            if got != tt.want {
                t.Errorf("Foo() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Adding Features

### Launcher Verb

1. Read command-guidelines.md (v1 ships no verbs besides --help)
2. Keep manual dispatch in cmd/lazyomo, route through internal/editor
3. Update usage text
4. Add tests

### TUI Pane or Dialog

1. Read cli-architecture.md (TUI section)
2. Edit internal/tui/model.go, update.go, or view.go
3. Keep the Editor interface accurate
4. Create or update tests

### Editable Field or Rule

1. Read docs/spec-editor-v1.md plus provider-integration-guide.md
2. Add the path rule in internal/editor/
3. Update tests

## Common Issues

### Tests Failing

```bash
# Run specific test
go test -run TestName ./internal/package/...

# Verbose output
go test -v ./internal/package/...

# Check for race conditions
go test -race ./...
```

### Build Errors

```bash
# Clean and rebuild
go clean
go build ./cmd/lazyomo
```

### Import Errors

```bash
# Tidy modules
go mod tidy
```

## Architecture Rules

### DO

- Follow the cmd/lazyomo -> tui -> editor -> omodit chain
- Validate before mutating; keep writes inside editor Save
- Write table-driven tests with stdlib testing
- Handle errors with context
- Use existing patterns

### DON'T

- Write files outside omodit Save
- Let the TUI import omodit directly
- Touch ~/.omo/agent/ credentials
- Use external test frameworks
- Create new packages without strong reason
- Use Cobra or other CLI frameworks

## Getting Help

1. Read AGENTS.md
2. Read relevant .agents/rules/ documentation
3. Look at similar existing code
4. Check test files for examples

## Code Review Checklist

Before submitting PR:

- [ ] All tests pass (`go test ./...`)
- [ ] No vet warnings (`go vet ./...`)
- [ ] Build succeeds (`go build ./cmd/lazyomo`)
- [ ] Package boundaries respected
- [ ] Existing patterns followed
- [ ] Tests added for new code
- [ ] Error handling with context
- [ ] No unnecessary dependencies

## Release Process

1. Update version (if applicable)
2. Update CHANGELOG.md
3. Create git tag
4. GitHub Actions builds binaries
5. Create GitHub release

## Resources

- [Go Documentation](https://go.dev/doc/)
- [Bubble Tea Documentation](https://github.com/charmbracelet/bubbletea)
- [Lipgloss Documentation](https://github.com/charmbracelet/lipgloss)
- [AGENTS.md](../AGENTS.md) - Master AI rules
