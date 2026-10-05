# lazyomo

TUI editor for `~/.omo/omo.jsonc`, styled after lazygit.

It opens your live omo config in the terminal so you can browse sections, tweak models and routing, then save once. Nothing touches disk until you confirm save, and every save leaves a timestamped backup next to the file.

## Layout

Three zones, one screen. The left column lists five sections: Models, Model Profiles, Agents, Categories, Telemetry. The upper right pane lists entries for the selected section, one row per key with a type hint and a dirty dot when edited. The lower right pane previews the selected entry. Scalar values edit in place, nested JSON shows as indented text.

Two fixed lines sit at the bottom. The first is a keybar with keys that apply to the focused pane. The second is a status line with the file path, a clean or dirty counter, filter text, and the last message. Dialogs (edit, add, confirm, help, error) open centered on top and take all input until you confirm or dismiss them.

Only one pane has focus at a time. The focused pane gets a green bold border, the rest stay gray, and the selected row uses a blue background. You'll mostly live in Sections, jump to Entries with a number key, and drill with enter.

## Install

Go install:

```bash
go install github.com/itokun99/lazyomo/cmd/lazyomo@latest
```

Build from source:

```bash
git clone https://github.com/itokun99/lazyomo.git
cd lazyomo
go build -o lazyomo ./cmd/lazyomo
```

npm:

```bash
npm install -g @itokun99/lazyomo
```

Requires Go 1.26.3 or later for source builds.

## Usage

```bash
lazyomo
```

It opens `~/.omo/omo.jsonc` in the editor. Pass `--help` to print usage. There are no other flags in v1.

## Keymap

Keys act on the focused pane unless noted.

| Key | Action |
|-----|--------|
| `1` | Select Models, focus Entries |
| `2` | Select Model Profiles, focus Entries |
| `3` | Select Agents, focus Entries |
| `4` | Select Categories, focus Entries |
| `5` | Select Telemetry, focus Entries |
| `0` | Focus the other main subpane (Entries or Detail) |
| `j` | Move selection down |
| `k` | Move selection up |
| `h` | Focus pane to the left |
| `l` | Focus pane to the right |
| `tab` | Focus next pane |
| `shift+tab` | Focus previous pane |
| `[` | Previous section, keep focus |
| `]` | Next section, keep focus |
| `enter` | Drill in (Sections to Entries to Detail), confirm in dialogs |
| `esc` | Step back (dialog to Detail to Entries to Sections) |
| `e` | Edit selected scalar in a popup |
| `a` | Add entry to current section |
| `d` | Delete selected entry (asks to confirm) |
| `space` | Toggle selected bool in place |
| `s` | Save all changes (asks to confirm, writes backup automatically) |
| `r` | Reload from disk (asks to confirm when dirty, reloads at once when clean) |
| `/` | Filter Entries list (`esc` clears an empty filter) |
| `?` | Open or close help |
| `q` | Quit (asks to confirm when dirty) |

`space` on a non-bool row reports `not a bool` and changes nothing. `q` on a clean state quits at once.

## Config and backups

The editor reads and writes a single file: `~/.omo/omo.jsonc`.

Each confirmed save first copies the pre-save file to a sibling backup named `<config>.bak.<UTC timestamp>`, for example `omo.jsonc.bak.2026-10-05T12-34-56-789Z`. Backups sit next to the config, never in another directory. `r` reloads from disk and drops unsaved edits (with confirm when dirty).

## What you can edit in v1

| Section | Editable |
|---------|----------|
| `models.*` | Add or remove alias, edit `model`, edit `reasoning` |
| `model_profiles.*` | Add or remove profile, edit `display_name`, edit `models` chain |
| `model_profile` | Picker for the active profile (must match an existing profile) |
| `agents.*` | Edit `model`, `models` chain, `reasoning`, `disable`; add or remove overlay entry |
| `categories.*` | Edit `model`, `models` chain |
| `telemetry.enabled` | Bool toggle |

Everything else is read-only in v1. The Detail pane tags those values `(read-only)`, and edit keys there report `read-only section` instead of opening a popup. Credentials under `~/.omo/agent/` are never read or written by the editor.

Validation runs before a change lands. Bad input keeps the popup open with a one-line reason, and a failed save leaves the file untouched.

## Safety

Edits stay in memory until you press `s`. The status line shows `clean` or `dirty (n)` so you always know what's pending. Save writes atomically: it creates the timestamped backup above, then replaces the file, preserving comments and key order outside the edited subtree. Cancel paths (`esc`, quit without saving, reload) never write.

## Development

```bash
go build -o lazyomo ./cmd/lazyomo
go test ./...
go vet ./...
```

See `docs/spec-tui-v1.md` for the UI spec and `docs/spec-editor-v1.md` for the editing scope.

## License

MIT
