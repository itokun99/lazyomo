# CLI Architecture Guide - lazyomo

## Architecture Overview

lazyomo is a TUI editor for `~/.omo/omo.jsonc`. It uses a short chain with clear ownership at each step.

```
cmd/lazyomo -> internal/tui -> internal/editor -> internal/omodit
```

There is no switching, no config discovery, and no CLI verb surface in v1. The binary opens the editor, the TUI renders panes and dialogs, `editor` enforces the editable scope and validation, and `omodit` owns the comment-preserving JSONC document plus atomic save with backup.

## Package Structure

```
cmd/lazyomo/main.go          # Entry point, minimal dispatch
internal/
├── omodit/                     # JSONC engine: Load, Get, Set, Add, Remove, Save
│   └── omodit.go
├── editor/                     # Editable surface: sections, entries, validation, dirty set
│   ├── editor.go
│   ├── edit.go
│   └── validate.go
└── tui/                        # Bubble Tea editor UI
    ├── model.go                # Model plus Editor interface (consumer side)
    ├── update.go               # Key handling
    ├── view.go                 # Three-zone layout plus overlays
    └── styles.go               # Lipgloss styles
```

Deleted switcher packages stay deleted: `internal/domain`, `internal/application`, `internal/infrastructure`, `internal/cli` do not exist. Do not reintroduce `ConfigService`, `KnownGroups`, `FilesystemStore`, or `omo_configs` paths.

## Dependency Flow

```
main.go
└── editor.LoadDefault()            # ~/.omo/omo.jsonc
└── tui.New(ed)
    └── Model holds an Editor interface (real *editor.Editor, fake in tests)
        └── SetScalar, ToggleBool, AddEntry, RemoveEntry
        └── DirtyPaths, Save, Reload
            └── omodit.Document: Get, Set, Add, Remove, Save
```

Interfaces sit next to the consumer. `internal/tui/model.go` declares the `Editor` interface it needs instead of importing editor internals. `editor` is the only package that imports `omodit`. The TUI never touches `omodit` directly.

## Entry Point (cmd/lazyomo/main.go)

```go
ed, err := editor.LoadDefault()   // ~/.omo/omo.jsonc
p := tea.NewProgram(tui.New(ed), tea.WithAltScreen())
_, err = p.Run()
```

The launcher accepts no args (open the editor) plus `--help` / `-h` (print usage). Unknown args fail with usage text. If edit verbs return later, keep this manual dispatch style: switch on `args[0]`, pass explicit deps, return errors to `main` for `Error: ...` output.

## Editor (internal/editor/)

`editor` implements the v1 editable surface from `docs/spec-editor-v1.md` on top of `omodit`:

| Section | What it covers |
|---------|----------------|
| `models` | Alias add/remove, `model`, `reasoning` |
| `model_profiles` | Profile add/remove, `display_name`, `models` chain, plus the `model_profile` picker |
| `agents` | `model`, `models` chain, `reasoning`, `disable`, overlay add/remove |
| `categories` | `model`, `models` chain (fixed set of 10) |
| `telemetry` | `enabled` toggle |

Every mutation validates path and value before touching the document, so bad input leaves memory and disk untouched. Edits accumulate in a dirty path set. `Save` writes atomically with a timestamped backup, `Reload` drops unsaved edits.

TUI-facing surface (see `model.go` for the exact interface):

```go
Path() string
Sections() []editor.Section
Detail(section editor.SectionID, key string) (editor.Detail, error)
SetScalar(path, value string) error
ToggleBool(path string) error
AddEntry(section editor.SectionID, key string) error
RemoveEntry(section editor.SectionID, key string) error
DirtyPaths() []string
Save() (string, error)   // returns backup name on success
Reload() error
```

Blocked deletes (alias still referenced, profile still selected) fail with an inline-displayable error. The file is never written mid-edit.

## Omodit (internal/omodit/)

`omodit` loads, edits, and saves JSONC while preserving comments, key order, and formatting outside the edited subtree. It parses once with hujson, applies RFC 6902 patches, and packs back bytes identical to the input wherever nothing changed.

```go
doc, err := omodit.Load(path)
v, ok := doc.Get("/models/k3/reasoning")
err = doc.Set("/models/k3/reasoning", "high")
err = doc.Add("/models/new-alias", ...)
err = doc.Remove("/models/old-alias")
err = doc.Save()   // backup, then atomic write
```

`Save` copies the pre-save file to `<config>.bak.<UTC timestamp>` next to the config, then writes via temp file plus rename. Pointers are RFC 6901 strings such as `/agents/sisyphus/model`.

## TUI (internal/tui/)

Bubble Tea Elm setup: `Model` in `model.go`, key handling in `update.go`, rendering in `view.go`, Lipgloss styles in `styles.go`.

Panes: Sections (left, 5 rows), Entries (main upper), Detail (main lower). Bottom lines: keybar plus status (`clean` or `dirty (n)`, path, filter, last message). Overlays: confirm, help, edit, add, error. Focus moves with `h`, `l`, `tab`, `shift+tab`, numbers jump to sections, `enter` drills or confirms, `esc` climbs back. Full keymap lives in `docs/spec-tui-v1.md` and the README.

The TUI never writes files. It calls `editor` ops, renders returned errors inline or in status, and confirms save, reload, delete, and dirty quit through the confirm overlay.

## Config File Paths

| Path | Purpose |
|------|---------|
| `~/.omo/omo.jsonc` | The single live config (read and written) |
| `~/.omo/omo.jsonc.bak.<UTC timestamp>` | Automatic pre-save backup, next to the config |
| `~/.omo/agent/` | Credentials, never read or written by lazyomo |

## Key Bindings (TUI)

v1 keymap: `1-5` select sections, `0` swap main subpane, `j`/`k`/`h`/`l` move, `tab` / `shift+tab` cycle panes, `[` / `]` switch sections, `enter` drill or confirm, `esc` back, `e` edit, `a` add, `d` delete (confirm), `space` toggle bool, `s` save (confirm plus auto backup), `r` reload or discard (confirm when dirty), `/` filter, `?` help, `q` quit (confirm when dirty).
