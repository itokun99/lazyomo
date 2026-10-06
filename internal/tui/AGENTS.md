# internal/tui

Bubble Tea app shell for the lazygit-style editor. Contract: [docs/spec-tui-v2.md](../../docs/spec-tui-v2.md).

## OVERVIEW
Renders the grouped side panel, the entries pane, the detail pane, the keybar, and the single status line, plus the overlays (edit, add, confirm, help, error, MCP add, secret reveal, model picker). One binding table drives dispatch, the keybar, and the help overlay. The data layer is consumed through consumer-side interfaces declared here; the real `*workspace.Registry`, `*editor.Editor`, and `*mcpfile.Session` are wired in `cmd/lazyomo/main.go`.

## STRUCTURE
- `model.go` - `Model` state, `New`, `Workspace`/`ConfigSurface`/`MCPSurface` interfaces, pane/overlay enums, dirty tracking, viewport math
- `update.go` - `Update`, context resolution, dispatch through the binding table, overlay/filter/edit/add flows, save and reload confirms
- `view.go` - `renderBase`, hand-painted bordered panes, keybar, status line, overlay boxes, help
- `bindings.go` - the ONE binding table (`bindingTables`), `Action` enum, `keybarFor`, `lookupAction`
- `side.go` - grouped side panel (CONFIG / MCP / PROVIDERS) and its flat cursor mapping
- `mcp.go` - MCP servers surface: list rows, detail rows, add/remove/rename/toggle/reveal, masked secrets
- `providers.go` - read-only Connections and Catalog panes
- `tools.go` - read-only Tools pane
- `picker.go` - `OverlayPicker` (fuzzy model picker): geometry, keys, preview, write-back
- `styles.go` - lipgloss styles; `StylesWithRenderer` pins a color profile for Ascii goldens
- `*_test.go` - unit and golden tests; `testdata/layout-80x24.golden` and `layout-120x40.golden` are the density goldens

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Add or change a key | `bindings.go` | one row in the right `ContextTable`; dispatch, keybar, and help all follow |
| Add a pane or overlay | `view.go` (`renderBase`, `renderOverlayBox`) + `update.go` (`contextName`, `handleOverlayKey`) | a new context needs a table and a render branch |
| Side panel groups/rows | `side.go` | `configSideOrder`, `buildSideGroups`; read-only rows carry `(ro)` |
| MCP servers surface | `mcp.go` | list/detail build, `e`/`a`/`d`/`space`/`x` flows, masking |
| Read-only derived panes | `providers.go`, `tools.go` | re-read on open, never write |
| Model picker | `picker.go` | `isPickerPath`, `pickerBox`, `pickerResultRows`, write-back |
| Data-layer interface | `model.go` | `Workspace`, `ConfigSurface`, `MCPSurface` |
| Density / goldens | `view.go`, `layout_test.go` | refresh with `UPDATE_GOLDEN=1 go test ./internal/tui -run TestGoldens` |

## CONVENTIONS
- Consumer-side interfaces only (`Workspace`, `ConfigSurface`, `MCPSurface` in `model.go`); the real registry, editor, and mcpfile session are wired in `cmd/lazyomo/main.go`. Tests use the `fakeWorkspace` harness.
- One binding table per context. Dispatch (`handleNormalKey`), the keybar (`keybarFor`), and help (`helpLines`) all read `bindingTables`; `TestBindingTableConsistency` pins that they agree.
- Every pane is hand-painted to an exact width and height so the density contract holds: exactly `height` lines, each at most `width` columns, at 80x24 and 120x40. `fitWidth`, `padRight`, and `windowOf` do the width math.
- Focus is shown by the green frame plus a `●` title marker; selection by `❯`. Color is never the sole cue.
- Secrets (`env`/`headers` values) stay masked in every settled state; raw values appear only inside the reveal overlay and an explicitly entered edit overlay.
- Nothing derived is ever written; the derived panes read explicit paths and render an empty state on a missing or malformed file.
- Styles come from `Styles`; tests that need deterministic output pin the Ascii profile via `StylesWithRenderer`.

## ANTI-PATTERNS
- Do NOT handle a key outside the binding table. Only printable runes (filter/edit/add/picker buffers) and the global `ctrl+c` quit bypass it.
- Do NOT let a pane paint more than its width or drop the 2-row scroll margin; the goldens and `TestDensity80x24`/`TestDensity120x40` guard it.
- Do NOT write from a read-only or derived pane, and do NOT read or write credential files (`auth.json`, `mcp-auth/`).
- Do NOT import `internal/tui` from lower packages; the TUI depends on them, never the reverse.
