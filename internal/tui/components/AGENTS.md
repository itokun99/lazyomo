# TUI Components

Score 9/17 -> kept: 9 components, 1116 LOC source + 1555 LOC tests; largest package by file count.

9 Bubble Tea components. Each owns its styles struct.

## PATTERN
```go
type FooModel struct { /* state */ }
type FooStyles struct { /* lipgloss styles */ }
func NewFooModel(...) FooModel
func (m *FooModel) Render(styles FooStyles, width int) string
func (m *FooModel) Show(...)
func (m *FooModel) Hide()
func (m FooModel) IsActive() bool
```

## WHERE TO LOOK
| Task | File | Notes |
|------|------|-------|
| Modify config list | `list.go` | `ListModel` cursor/scroll/grouping; largest component (194 LOC) |
| Add search filter | `search.go` | `SearchModel` with `textinput` |
| Change detail view | `detail.go` | `DetailModel` with scroll |
| Add help overlay | `help.go` | `HelpModel` toggled by `?` |
| Modify validation | `validate.go` | `ValidateModel` shows bulk results |
| Change backup UI | `backup.go` | `BackupModel` with restore confirmation |
| Add diff view | `diff.go` | `DiffModel` side-by-side; naive line diff, no LCS (155 LOC) |
| Show config info | `info.go` | `InfoModel` metadata display |
| Change status bar | `status.go` | `StatusModel` bottom bar; canonical smallest component (78 LOC) |

## CONVENTIONS
- Each component defines its own `Styles` struct — never share styles across components
- Components are stateful: `Show()` activates, `Hide()` deactivates, `IsActive()` checks
- `Render()` takes styles + width, returns string; value receivers for `Render`/`IsActive`, pointer receivers for `Show()`/`Hide()`
- Tests: same-package, no mocks needed - drive state/`Render` directly; table-driven with `t.Run()` where cases fit (6/9 files)

## ANTI-PATTERNS
- Do NOT put styles in a shared location — causes circular imports
- Do NOT access `tui.App` from components — components are leaf nodes
- Do NOT rely on `computeDiff` for correctness: index-by-index, one inserted line flags every following line (diff.go:78-103)
