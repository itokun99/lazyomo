# Changelog

## Unreleased

## [3.1.0] - 2026-10-06

### Added
- Two-document editing in one TUI: user `~/.omo/omo.jsonc` + `~/.omo/agent/mcp.json`, each with atomic save and timestamped backups
- MCP servers pane: list / detail / add / edit / rename / remove / toggle, strict-JSON validation, secrets masked `•••` with explicit reveal
- Read-only connections + model-catalog panes with provenance badges and refreshed-at, built from `models.json` / `models-store.json`
- Read-only MCP tools pane per server, built from `cache/mcp-cache.json`
- Fuzzy model picker overlay (`e` on model fields): refs-only in the models section, aliases elsewhere, unchanged-selection closes without a write
- `git_master` section (commit_footer / commit_trailer toggles); `model_profile` accepts an existing profile key or a literal `provider/model` pin
- New packages: `internal/derived` (comment-preserving derived readers), `internal/fuzzy` (stdlib DP matcher), `internal/workspace` (multi-file save + stale guard), `internal/mcpfile` (strict-JSON surface)
- Spec v2 docs (`docs/spec-tui-v2.md`, `docs/spec-editor-v2.md`); v1 specs marked superseded; README + AGENTS knowledge bases refreshed

### Fixed
- First MCP server add is reachable when `mcp.json` does not exist yet
- Catalog decode tolerates numeric epoch timestamps; junk model entries are skipped, never dropped
- gofmt drift in editor and tui

