# AI Agent Rules - lazyomo

This directory contains documentation for AI agents working on the lazyomo project.

## Files

| File | Purpose |
|------|---------|
| [ai-workflow.md](ai-workflow.md) | Development workflow for AI-assisted development |
| [cli-architecture.md](cli-architecture.md) | TUI plus editor chain details |
| [command-guidelines.md](command-guidelines.md) | Launcher usage plus style for future verbs |
| [go-standards.md](go-standards.md) | Go engineering standards |
| [testing-guide.md](testing-guide.md) | Testing patterns and conventions |
| [bugfix-workflow.md](bugfix-workflow.md) | Bug investigation and fixing workflow |
| [refactor-workflow.md](refactor-workflow.md) | Safe refactoring approach |
| [contributing-guide.md](contributing-guide.md) | Developer onboarding guide |
| [provider-integration-guide.md](provider-integration-guide.md) | AI provider integration standards |

## Quick Start

1. Read [../AGENTS.md](../AGENTS.md) for master rules
2. Read [contributing-guide.md](contributing-guide.md) for getting started
3. Read the relevant guide for your task

## Architecture Summary

```
cmd/lazyomo/main.go          # Entry point (open editor or --help)
internal/
├── omodit/                   # JSONC engine (Load, Get, Set, Add, Remove, Save)
├── editor/                   # Editable surface, validation, dirty set
└── tui/                      # Bubble Tea editor UI (model, update, view, styles)
```

Single file edited: `~/.omo/omo.jsonc`. Backups are siblings named `<config>.bak.<UTC timestamp>`. No switching, no discovery, no `ConfigService` or `KnownGroups`.

## Key Rules

1. **Chain**: `cmd/lazyomo` -> `internal/tui` -> `internal/editor` -> `internal/omodit`; TUI never touches `omodit` directly
2. **No writes until save**: edits stay in the dirty set; `Save` backs up then writes atomically
3. **Validate before mutate**: bad path or value returns an inline error, document untouched
4. **Consumer-side interfaces**: TUI declares the `Editor` interface it needs
5. **Table-driven tests**: Standard testing package only
6. **Error wrapping**: `fmt.Errorf("context: %w", err)`
