# PROJECT KNOWLEDGE BASE

**Generated:** 2026-10-05
**Commit:** ff6628e (main)
**Branch:** main

## OVERVIEW
lazyomo - CLI/TUI switcher for oh-my-openagent configs. Go 1.26.3 + Charm stack (Bubble Tea/Bubbles/Lipgloss), clean architecture (domain -> application -> infrastructure -> cli/tui), manual CLI dispatch. The npm package `@indrawandev/lazyomo` ships `bin/lazyomo.js`, a complete JS re-implementation of the same CLI.

## STRUCTURE
```
lazyomo/
├── internal/domain/           # pure types + rules (Config, groups, schema) - no I/O
├── internal/infrastructure/   # Store + BackupManager impls (hardcoded ~/.config paths)
├── internal/application/      # ConfigService orchestrator
├── internal/cli/              # Handle() manual dispatch
├── internal/tui/              # Bubble Tea app shell -> tui/AGENTS.md
│   └── components/            # 9 leaf renderers -> components/AGENTS.md
├── cmd/lazyomo/            # MISSING from tree - see NOTES
├── bin/lazyomo.js          # npm launcher + JS fallback implementation
├── scripts/                   # build.sh/.bat, install.js (npm postinstall)
└── Formula/lazyomo.rb      # Homebrew formula (downloads release binaries)
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Add CLI command | internal/cli/handler.go:16 | switch in `Handle()`; `cmdXxx(service, w) int` |
| Add TUI view / component | internal/tui/ | see tui/AGENTS.md + components/AGENTS.md |
| Config grouping | internal/domain/group.go:11 | `KnownGroups` + "Custom" fallback |
| Validation rules | internal/domain/schema.go | `SchemaValidator`; required key: `agents` |
| Config/backup paths | internal/infrastructure/filesystem.go:27 | hardcoded `~/.config/opencode/...` |
| Service logic | internal/application/service.go:17 | `ConfigService` |

## CODE MAP
| Symbol | Type | Location | Refs (LSP) | Role |
|--------|------|----------|------------|------|
| App | struct | internal/tui/app.go:54 | 34 | Bubble Tea model (largest source file, 653 lines) |
| ConfigService | struct | internal/application/service.go:17 | 20 | orchestrator injected into cli/tui |
| Config | struct | internal/domain/config.go:6 | 19 | value type; immutable `Validate()` |
| Handle | func | internal/cli/handler.go:16 | 13 | CLI dispatch (orphaned until cmd/ returns) |
| Store | interface | internal/infrastructure/filesystem.go:11 | 5 | config file ops |
| KnownGroups | map | internal/domain/group.go | - | display classification |

## CONVENTIONS (deviations from default Go)
- Package boundaries: domain -> infrastructure -> application; cli imports application + domain; tui imports domain + infrastructure + components; components are leaves. `domain/` has NO I/O and no internal imports.
- Interface-based DI: `infrastructure.Store`, `infrastructure.BackupManager`, `domain.SchemaValidator` are injected into `ConfigService` (service.go:24). Never construct concrete deps in the service layer.
- Interfaces are declared next to their implementation; the TUI declares a consumer-side `configService` interface (app.go:16-28) instead of importing `internal/application`.
- Manual CLI dispatch only: `Handle(service, args, w) int` switches on `args[0]`; handlers write to the injected `io.Writer`, never `os.Stdout`. NO Cobra / urfave.
- Errors: `fmt.Errorf("verb-ing noun: %w", err)` only - no custom error types, no sentinels, no `errors.Is/As`. The TUI never propagates: it renders "Error: ..." in the status bar (3s auto-clear).
- Graceful degradation: `os.IsNotExist` -> empty value + `nil` error (filesystem.go:50, backup.go:81).
- `Config.Validate` is immutable: returns a new `Config`, never mutates the receiver (config.go:28).
- Tests: stdlib `testing` only (no testify); table-driven + `t.Run`; hand-written mocks with compile-time checks (`var _ X = (*mockX)(nil)`); black-box `_test` packages only for application + cli.
- Test seams: `NewX()` derives `$HOME` paths; `NewXWithPath(...)` accepts `t.TempDir()` (filesystem.go:39, backup.go:46).
- TUI components: contract lives in internal/tui/components/AGENTS.md (each component owns its Styles; leaf nodes).
- Paths (no XDG): configs `~/.config/opencode/omo_configs/omo-<alias>.json`, active `~/.config/opencode/oh-my-openagent.json`, backups `~/.config/lazyomo/backups/oh-my-openagent.<ts>.json`; dirs 0o755, files 0o644.
- Group display order is the hardcoded `knownGroupNames()` slice (Mono, Optimized, Low-Cost, Custom) because `KnownGroups` is a map (service.go:33); CLI sorts Custom alphabetically.

## ANTI-PATTERNS (THIS PROJECT)
| Violation | Why it's wrong | Where it bites |
|-----------|----------------|----------------|
| I/O or internal imports in `domain/` | breaks pure business-logic isolation | domain/config.go, group.go |
| Business logic in `infrastructure/` | single responsibility | infrastructure/filesystem.go |
| External test frameworks / assertion libs | stdlib-only convention | any `*_test.go` |
| Shared `Styles` structs across components | causes circular imports | tui/components/* |
| New packages without strong reason | over-engineering (small repo) | internal/* |
| Cobra / urfave / CLI frameworks | manual dispatch is the rule | internal/cli/handler.go |
| `os.Stdout` inside CLI handlers | breaks the injected-writer contract | internal/cli/handler.go |

No TODO/FIXME/HACK/DEPRECATED markers exist in Go code - debt lives in this file and prose docs only.

## UNIQUE STYLES
- Dual implementation: the Go tree (internal/) and a complete JS fallback (bin/lazyomo.js) that duplicates paths, KNOWN_GROUPS, and schema validation.
- "Active config" detection is content comparison: target file vs every omo-*.json (service.go:78-103; same in the JS impl). Editing the active file in place breaks detection until a switch.
- `FilesystemStore.WriteConfig` exists but is never called by the service; `SwitchConfig` writes the target with a direct `os.WriteFile` (service.go:123).
- First-run gotcha: `SwitchConfig` requires an existing target file - `backup.CreateBackup` errors when the target is missing (backup.go:55), so switching with no active config fails.

## COMMANDS
```bash
go test ./...        # 6/6 packages pass - the only working quality gate
go vet ./...         # clean
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
go build -o lazyomo ./cmd/lazyomo   # FAILS: cmd/ absent from the tree
go install github.com/itokun99/lazyomo/cmd/lazyomo@latest   # same missing target
```

## NOTES
- **`cmd/lazyomo/main.go` is missing** - there is no `package main` anywhere, and no `cmd/` in git history. Yet scripts/build.sh:6, scripts/build.bat, .github/workflows/release.yml:34, README and CONTRIBUTING all reference it. Restore the wiring documented at .agents/rules/cli-architecture.md:53-80 before any build work.
- CI (`release.yml`) runs on `v*` tags only: builds the missing path, runs no tests/vet, pins Go 1.22 while go.mod requires 1.26.3.
- Version skew: scripts/install.js pins `v2.0.0` vs package.json/Formula `2.0.1`. Naming is now unified: module, repo, and npm name are all lazyomo variants.
- Debt: duplicate `targetPath` literal (filesystem.go:34, backup.go:41); `mockStore`/`mockBackupManager` duplicated between service_test.go and handler_test.go; naive line-by-line diff (components/diff.go:78-103); no XDG support (compile-time constants).
- Coverage targets (domain 100%, infra/application 90%+, cli 80%+, tui 70%+) are documented in .agents/rules/testing-guide.md but unenforced (no CI tests).

## Related Documentation

- [ai-workflow.md](.agents/rules/ai-workflow.md) - Development workflow
- [cli-architecture.md](.agents/rules/cli-architecture.md) - CLI architecture details
- [command-guidelines.md](.agents/rules/command-guidelines.md) - Command creation guide
- [go-standards.md](.agents/rules/go-standards.md) - Go engineering standards
- [testing-guide.md](.agents/rules/testing-guide.md) - Testing standards
- [bugfix-workflow.md](.agents/rules/bugfix-workflow.md) - Bug investigation
- [refactor-workflow.md](.agents/rules/refactor-workflow.md) - Refactoring guide
- [contributing-guide.md](.agents/rules/contributing-guide.md) - Developer onboarding
- [provider-integration-guide.md](.agents/rules/provider-integration-guide.md) - AI provider integration
