# internal/tui

Score 12/17 -> created: distinct domain; highest source-LOC package; `app.go` (653 lines) is the repo's largest source file.

## OVERVIEW
Bubble Tea app shell (`App`) that renders the interactive TUI: view modes, key dispatch, async `tea.Cmd` helpers. Renders the 9 leaf components in `components/` - contract in `components/AGENTS.md`.

## STRUCTURE
- `app.go` (653) - `App` model, `Update`/`View`, Msg types, `tea.Cmd` helpers, per-mode key handlers
- `keys.go` (86) - `KeyMap` / `DefaultKeyMap`
- `styles.go` (65) - app-level `Styles` (components own theirs)
- `components/` - 9 leaf renderers + tests (1555 LOC tests)

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Add a view mode | app.go:40-50 + `View()` app.go:204-302 | enum value + render branch; branch order sets IsActive() priority |
| Add key handling | `handleKey` app.go:430-501; per-mode handlers app.go:502-653 | whichever component is active intercepts keys first |
| Add an async service call | app.go:340-427 | `tea.Cmd` -> Msg pattern (e.g. `switchConfigCmd` / `switchCompleteMsg`) |
| Transient status text | `status.SetMessage` + `clearMessageAfter` | 3s auto-clear via `errorClearDuration` (app.go:37) |
| Terminal size gate | app.go:204-210 | below 80x24 (`MinWidth`/`MinHeight`) shows resize prompt |
| Key bindings | keys.go:7-27 | arrows + vim bindings (README key table) |

## CONVENTIONS
- Depends on the consumer-side `configService` interface (app.go:16-28): never import `internal/application`; tests mock the interface (app_test.go:58-61).
- Every service call is async: return a `tea.Cmd`, mutate state in `Update` when the Msg arrives.
- Errors become status-bar text ("Error: ..."), auto-cleared after 3s - never panic, never propagate out of the TUI.
- `View()` renders exactly one of backup/diff/validate/detail/info/search/list (first matching `IsActive()` wins); help overlays on top.
- App-level styles live in styles.go; each component receives its own `XxxStyles` value in `Render`.

## ANTI-PATTERNS
- Do NOT let components import `tui.App` - they are leaf nodes (see components/AGENTS.md).
- Do NOT add a screen without both a `View()` branch and a key-handler branch - a missing branch silently no-ops.
- Do NOT put file/config logic here - it belongs in ConfigService.
