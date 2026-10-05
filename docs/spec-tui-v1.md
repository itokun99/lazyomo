# lazyomo TUI v1: UI and Navigation Spec

Target: a lazygit-style editor for `~/.omo/omo.jsonc`.
Audience: implementers of the Bubble Tea TUI. This spec is decision-complete for v1.

## 1. Pane Model

The screen has three zones plus overlays. It copies lazygit's side-plus-main layout.

| Zone | Size | Shows | lazygit pattern copied |
|---|---|---|---|
| Sections (left column) | 30% width, min 24 cols; full height minus bottom 2 lines | Fixed list of 5 sections: Models, Model Profiles, Agents, Categories, Telemetry | Tabbed side panels in one column |
| Entries (main, upper) | Remaining width; 60% of main height | Keys of the selected section, one row per entry with type hint and dirty dot | Main pane list (focus `0`) |
| Detail (main, lower) | Remaining width; 40% of main height | Preview of the selected entry: scalar values editable, nested JSON shown as indented text | Main pane preview split |
| Keybar (bottom, line 1) | Full width, 1 line, fixed | Context-sensitive keys for the focused pane, e.g. `e edit  a add  d del  s save  ? help` | Bottom keybar |
| Status (bottom, line 2) | Full width, 1 line, fixed | File path, dirty indicator, filter text, last error or `ready` | Status line |
| Overlay (popup) | Centered, max 60 cols by 12 rows | Confirm, help, edit, add, and error dialogs; dims panes behind it | Confirm popup with `enter`/`esc` |

Focus behavior follows lazygit. Only one pane is focused at a time. The focused pane uses a green bold border. Unfocused panes use gray borders. The selected row uses a blue background. Scrolling keeps a 2-line margin at the top and bottom of lists. `h`, `l`, `tab`, and `shift+tab` move between adjacent panes in the order Sections, Entries, Detail. Number keys jump directly (see keymap). `?` opens a fullscreen help overlay. Popups take all input until confirmed or dismissed.

Dirty-state indicator rule: the status line shows `clean` when nothing changed. Once any edit, toggle, add, or delete lands, it switches to `● dirty (n)` where `n` is the count of changed paths. Dirty entries also gain a dot in the Entries list. Saving clears the count. Reloading clears it too.

## 2. Keymap

Keys act on the focused pane unless the Context column says otherwise. The map copies lazygit's `j`/`k`/`h`/`l`, number jumps, `enter`/`esc`, `?`, `q`, and `/` behavior.

| Key | Action | Context |
|---|---|---|
| `1` | Select Models and focus Entries | Global |
| `2` | Select Model Profiles and focus Entries | Global |
| `3` | Select Agents and focus Entries | Global |
| `4` | Select Categories and focus Entries | Global |
| `5` | Select Telemetry and focus Entries | Global |
| `0` | Focus the active main subpane (Entries if Detail is focused, Detail if Entries is focused) | Global |
| `j` | Move selection down one row | Sections, Entries, Detail, Help |
| `k` | Move selection up one row | Sections, Entries, Detail, Help |
| `h` | Move focus to the pane on the left | Entries, Detail |
| `l` | Move focus to the pane on the right | Sections, Entries |
| `tab` | Move focus to the next pane | Global |
| `shift+tab` | Move focus to the previous pane | Global |
| `[` | Select the previous section without moving focus | Global |
| `]` | Select the next section without moving focus | Global |
| `enter` | Drill down: Sections goes to Entries; Entries opens Detail; popup confirms | Sections, Entries, popup |
| `esc` | Climb back: popup closes first, then Detail goes to Entries, Entries goes to Sections | Popup, Detail, Entries |
| `e` | Edit the selected scalar value in a popup editor | Entries, Detail |
| `a` | Add a new entry to the current section | Entries |
| `d` | Delete the selected entry (asks for confirm) | Entries |
| `space` | Toggle the selected bool in place, no popup | Entries, Detail |
| `s` | Save all changes (asks for confirm when dirty) | Global |
| `r` | Reload from disk (asks for confirm when dirty, reloads at once when clean) | Global |
| `?` | Open or close the help overlay | Global |
| `q` | Quit; asks for confirm when dirty | Global |
| `/` | Filter the Entries list; `esc` clears an empty filter | Entries |

Help lists this same table. You don't need to memorize it. `q` on a clean state quits at once. `q` in a dirty state opens a confirm popup with `Quit without saving?`, and `enter` quits while `esc` stays.

## 3. Interaction Flows

Edit scalar: select a row, press `e`, and an editor popup shows the current value. `enter` validates and applies it, then marks the path dirty. `esc` drops the edit. Bad input keeps the popup open with a one-line reason.

Toggle bool: select a bool row and press `space`. It flips at once with no popup. The row and status line gain dirty marks. Pressing `space` on a non-bool row flashes `not a bool` in status.

Add entry: focus Entries and press `a`. A popup asks for the new key name. Names must be non-empty and unique in the section. `enter` inserts a default empty string value and selects the new row. `esc` cancels.

Delete entry: select a row and press `d`. A popup shows the full entry path and asks for confirm. `enter` removes it and marks the parent dirty. `esc` keeps it.

Save: press `s`. When clean, status flashes `already saved` and nothing else happens. When dirty, a popup lists the changed path count and asks for confirm. `enter` creates a timestamped backup automatically, then writes `omo.jsonc`. Success clears dirty marks and reports `saved + backup <name>`. Failure keeps dirty marks and shows the error.

Discard changes: press `r`. When clean, it reloads silently. When dirty, a popup warns `reload and lose n changes?`. `enter` reloads from disk and clears dirty marks. `esc` keeps the edits.

Error and diagnostic display: validation errors appear inline in Detail plus a short note in status. Save or load failures open an error popup with `enter` to retry and `esc` to close. The last error stays in status until the next successful action.

Read-only sections: Telemetry and any unknown top-level keys render read-only. Their Detail pane shows values with an `(read-only)` tag. `e`, `a`, `d`, and `space` there show `read-only section` in status instead of opening popups. Selection, filtering, and help still work.

## 4. Non-Goals v1

* No git integration: no staging, diffing against git, or commit actions.
* Single file only: just `~/.omo/omo.jsonc`, no multi-file tabs or includes.
* No command-log window: the `@` toggle from lazygit doesn't ship in v1.
* No mouse support: keyboard only.
* No schema migration or version upgrades: v1 edits values, it doesn't reshape config versions.
* No plugins, themes, or custom keybind config: the keymap above is fixed.
* No concurrent editing: last save wins, and `r` is the only refresh path.
