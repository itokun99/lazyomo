# Changelog

## Unreleased

- Repurpose lazyomo from config switcher to TUI editor for `~/.omo/omo.jsonc` (lazygit-style panes, section keymap, save with automatic timestamped backup).
- Add `internal/editor` plus `internal/omodit` engine: comment-preserving JSONC load, Get, Set, Add, Remove, Save with atomic write and `<config>.bak.<UTC timestamp>` backup next to the file.
- Remove old switcher packages (`internal/domain`, `internal/application`, `internal/infrastructure`, `internal/cli`): no switching, no `omo_configs` discovery, no `ConfigService` or `KnownGroups`.
- Rename product from omo-switch to lazyomo (docs only, no behavior change).

## [2.0.0] - 2026-06-04

### Added
- Interactive TUI mode (Bubble Tea) as default
- Search/filter configs with `/` key
- Config detail view with `s` key
- Help overlay with `?` key
- Config validation with `v` key
- Backup manager with `b` key
- Config diff viewer with `d` key
- Config info display with `i` key
- Reload configs with `r` key
- Cross-platform builds (macOS, Linux, Windows)
- GitHub Actions release workflow

### Changed
- Rewritten from Node.js to Go
- TUI mode is now default (no args)
- CLI mode available via `--cli` flag
- Layered architecture (domain/application/infrastructure/tui)

### Preserved
- All CLI commands (--list, --current, show, alias)
- Config discovery from ~/.config/opencode/omo_configs/
- Schema validation before switching
- Auto-backup before switching
- Grouped display (Mono, Optimized, Low-Cost, Custom)

## [1.0.0] - 2025-01-01

### Added
- Initial Node.js CLI implementation
- Config discovery and switching
- Schema validation
- Auto-backup
