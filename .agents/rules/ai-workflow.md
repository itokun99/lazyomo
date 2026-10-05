# AI Workflow - Development Guidelines for lazyomo

## Overview

This document defines the AI-assisted development workflow for the lazyomo project. It ensures consistent, high-quality contributions from any AI agent.

## Pre-Development Checklist

Before writing ANY code:

1. **Read AGENTS.md** - Understand architecture boundaries
2. **Read this file** - Understand workflow requirements
3. **Find similar code** - Copy existing patterns
4. **Run tests** - `go test ./...` to establish baseline

## Development Workflow

### Phase 1: Understanding

```
1. Identify the task type:
   - Launcher verb (if any) → Read COMMAND_GUIDELINES.md
   - New TUI pane or dialog → Read CLI_ARCHITECTURE.md
   - Editable field or rule → Read PROVIDER_INTEGRATION_GUIDE.md plus docs/spec-editor-v1.md
   - Bug fix → Read BUGFIX_WORKFLOW.md
   - Refactor → Read REFACTOR_WORKFLOW.md
   - New test → Read TESTING_GUIDE.md

2. Find reference implementation:
   - Search for similar existing code
   - Understand the pattern used
   - Note the package boundaries

3. Identify affected files:
   - Which packages need changes?
   - What interfaces are involved?
   - What tests need updating?
```

### Phase 2: Implementation

```
1. Create todo list with specific steps
2. Implement one step at a time
3. Run tests after each step
4. Verify package boundaries not violated
```

### Phase 3: Verification

```
1. Run `go test ./...` - all tests must pass
2. Run `go vet ./...` - no warnings
3. Run `go build ./cmd/lazyomo` - compiles successfully
4. Manual verification if applicable
```

## Task-Specific Workflows

### Adding a Launcher Verb

v1 ships no switcher verbs. The launcher opens the editor or prints help. If a verb returns:

```
1. Read COMMAND_GUIDELINES.md
2. Add case in cmd/lazyomo dispatch (keep manual switch, no Cobra)
3. Route through internal/editor, never around it
4. Update usage text in main.go
5. Add tests beside the changed package
6. Run: go test ./... plus go build ./cmd/lazyomo
```

### Changing a TUI Pane or Dialog

```
1. Read CLI_ARCHITECTURE.md (TUI section)
2. Edit internal/tui/model.go, update.go, or view.go (no components/ dir exists)
3. Keep the consumer-side Editor interface in model.go accurate
4. Keep overlay handling (confirm, help, edit, add, error) exclusive: popup takes all input
5. Create or update table-driven tests in tui_test.go
6. Run: go test ./internal/tui/...
```

### Adding an Editable Field

```
1. Read docs/spec-editor-v1.md plus PROVIDER_INTEGRATION_GUIDE.md
2. Add the path rule in internal/editor/edit.go or validate.go
3. Surface it in Sections or Detail as the spec directs
4. Update tests in internal/editor/
5. Run: go test ./internal/editor/... ./internal/omodit/...
```

### Modifying Validation Logic

```
1. Edit internal/editor/validate.go
2. Keep the convention: invalid input returns an inline error, document untouched
3. Update tests beside the editor
4. Run: go test ./internal/editor/...
```

## Error Recovery

If you encounter issues:

1. **Don't guess** - Read the actual code
2. **Don't assume** - Verify with tests
3. **Don't skip** - Complete all steps
4. **Don't break** - Maintain package boundaries

## Quality Gates

Before marking task complete:

- [ ] All tests pass (`go test ./...`)
- [ ] No vet warnings (`go vet ./...`)
- [ ] Build succeeds (`go build ./cmd/lazyomo`)
- [ ] Package boundaries respected
- [ ] Existing patterns followed
- [ ] Tests added/updated for changes

## Common Mistakes to Avoid

| Mistake | Why It's Wrong | How to Avoid |
|---------|---------------|--------------|
| File writes outside omodit Save | Breaks atomic plus backup habit | Route all writes through editor Save |
| TUI importing omodit | Skips validation and dirty tracking | Talk to the Editor interface only |
| Touching ~/.omo/agent/ | Credentials stay out of scope | Edit omo.jsonc keys only |
| Using Cobra | Project keeps manual dispatch | Check cli-architecture entry section |
| External test frameworks | Inconsistent conventions | Use stdlib testing only |
| Creating new packages | Over-engineering | Use editor, omodit, or tui |
| Skipping tests | Quality regression | Always write tests |

## Reference Files

| File | Purpose |
|------|---------|
| AGENTS.md | Master rules |
| CLI_ARCHITECTURE.md | Architecture details |
| COMMAND_GUIDELINES.md | Command creation |
| GO_STANDARDS.md | Go conventions |
| TESTING_GUIDE.md | Test patterns |
| BUGFIX_WORKFLOW.md | Bug investigation |
| REFACTOR_WORKFLOW.md | Refactoring guide |
| CONTRIBUTING_GUIDE.md | Onboarding |
