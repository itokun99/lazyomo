# lazyomo TUI v2: UI and Navigation Spec

Target: a lazygit-style editor for `~/.omo/omo.jsonc` plus the MCP servers file at `<agentDir>/mcp.json`.
Audience: implementers of the Bubble Tea TUI. This spec is decision-complete for v2 and describes the as-built surface.

> Supersedes [docs/spec-tui-v1.md](spec-tui-v1.md). v1 described a three-zone layout over a single document. v2 replaces it with the grouped side panel, the single binding table, the MCP servers surface, the read-only derived panes, and the fuzzy model picker.

## 1. Layout (wireframe A, as shipped)

Three bordered panes plus two fixed lines. The left column is the grouped side panel; the upper right pane lists entries for the selected side row; the lower right pane is the detail view. The bottom two lines are the keybar and the single status line. The frame below is the committed `internal/tui/testdata/layout-80x24.golden` (a fixture with `k1`/`b1`/`ch`/`al` model entries and one read-only `$schema` key), so it is the exact as-built render at 80x24:

```
╭─ ● Sections ─────────╮╭─ Models ─────────────────────────────────────────────╮
│CONFIG                ││❯ k1 [scalar] v1                                      │
│❯ Models              ││  b1 [bool] false                                     │
│  Model Profiles      ││  ch [chain] 2 items                                  │
│  Agents              ││  al [alias] m1                                       │
│  Categories          ││                                                      │
│  Telemetry           ││                                                      │
│  Git                 ││                                                      │
│  $schema (ro)        ││                                                      │
│MCP                   ││                                                      │
│  Servers             ││                                                      │
│  Tools (ro)          ││                                                      │
│PROVIDERS (ro)        │╰──────────────────────────────────────────────────────╯
│  Connections (ro)    │╭─ models/k1 ──────────────────────────────────────────╮
│  Catalog (ro)        ││❯ k1: v1                                              │
│                      ││                                                      │
│                      ││                                                      │
│                      ││                                                      │
│                      ││                                                      │
│                      ││                                                      │
│                      ││                                                      │
╰──────────────────────╯╰──────────────────────────────────────────────────────╯
j/k move  enter open  [ ] cycle  1-9,0 jump  s save  r reload  ? help  q quit   
/tmp/omo.jsonc · clean                                                     ready
```

Pane geometry, all hand-painted so every line is exactly its pane width:

| Zone | Size | Shows |
|---|---|---|
| Sections (left) | 30% width, min 24 cols, max width-20; full height minus the bottom 2 lines | The grouped side panel (see below) |
| Entries (upper right) | Remaining width; 60% of the content height | Rows of the selected side row: editor entries, MCP servers, or derived rows |
| Detail (lower right) | Remaining width; 40% of the content height | Detail of the selected row; secrets masked, derived rows provenance-badged |
| Keybar (line `height-1`) | Full width, 1 line | The active context's keybar pairs, from the binding table |
| Status (line `height`) | Full width, 1 line | Left: active source path and clean/dirty; right: message or `ready` |
| Overlay (popup) | Centered | Confirm, help, edit, add, error, MCP add, MCP reveal, picker |

Below 80x24 the whole view is replaced by `Terminal too small (need 80x24, got WxH)`.

### Grouped side panel

The side panel renders three headed groups. Headers are display only; the cursor, `[`/`]`, and the digit keys move over the flat list of selectable rows in display order.

| Group | Rows | Provenance |
|---|---|---|
| `CONFIG` | Models, Model Profiles, Agents, Categories, Telemetry, Git, then one read-only row per other present top-level key | Editable, except appended read-only rows |
| `MCP` | Servers, Tools | Servers editable (when a session is bound), Tools read-only |
| `PROVIDERS (ro)` | Connections, Catalog | Read-only, derived |

Read-only rows carry an `(ro)` suffix; the `PROVIDERS` group header carries `(ro)`. The `❯` marker appears exactly once in the panel. The CONFIG rows are the editor sections in contract order, with any extra present top-level keys appended after them; see [docs/spec-editor-v2.md](spec-editor-v2.md) for the section list.

## 2. P1-P6 rules (as implemented)

* **P1 border and titles.** Every pane is a hand-painted 1-line rounded border box with its title inside the top border, corners `╭ ╮ ╰ ╯` (`paintPane`). Overlay boxes use the same corners (`paintBox`).
* **P2 focus.** Exactly one pane is focused. The focused pane uses the green frame plus a `● ` marker in its title; color is never the sole cue. `❯` appears exactly once per list (side panel, entries, detail). Pinned by `TestSingleFocusCue` and `TestCursorOncePerList`.
* **P3 keybar.** The keybar shows exactly the binding table's bar subset for the active context, joined as `label desc` pairs with two spaces. While any overlay is open the base keybar is hidden (the overlay box carries its own footer). Pinned by `TestKeybarFromTable` and `TestKeybarHiddenUnderOverlay`.
* **P4 status.** Exactly one status line. Left side: `activePath · clean` or `activePath · ● dirty(n)`, where `n` is the total changed path count across every registered source. Right side: the last message, or `ready`; an active filter is prefixed as `filter: <text>`. Errors surface in status or in the error overlay. Pinned by `TestStatusLineFormat`.
* **P5 bindings.** One table per context drives dispatch, the keybar, and the help overlay (`internal/tui/bindings.go`). `esc` cancels the topmost overlay everywhere; `enter` opens or selects; the picker captures every printable rune as query input. Only printable runes (filter/edit/add/picker buffers) and the global `ctrl+c` quit are handled outside the table. Pinned by `TestBindingTableConsistency`.
* **P6 density.** At 80x24 the view paints exactly 24 lines, every line at most 80 columns, with no wrapping and a 2-row scroll margin; entries viewport is at least 11 rows, detail viewport at least 6 rows, and the picker shows at least 14 result rows (24 at 120x40). Pinned by `TestDensity80x24`, `TestDensity120x40`, `TestScrollMarginTwoRows`, and `TestPickerGeometryContract`.

## 3. Keymap (the single binding table)

`internal/tui/bindings.go` declares every key once per context. The keybar and help render from the same table, so this section is a faithful transcription. `bar` marks the pairs the keybar shows.

### Context `sections` (side panel)

| Keys | Action | Keybar |
|---|---|---|
| `j`, `k`, `down`, `up` | move | `j/k move` |
| `enter` | open | `enter open` |
| `esc` | back | |
| `l`, `h`, `right`, `left` | pane | |
| `tab` | next pane | |
| `shift+tab` | prev pane | |
| `[`, `]` | cycle | `[ ] cycle` |
| `1`-`9`, `0` | jump | `1-9,0 jump` |
| `s` | save | `s save` |
| `r` | reload | `r reload` |
| `?` | help | `? help` |
| `q` | quit | `q quit` |

Keybar: `j/k move  enter open  [ ] cycle  1-9,0 jump  s save  r reload  ? help  q quit`

### Context `entries`

| Keys | Action | Keybar |
|---|---|---|
| `j`, `k`, `down`, `up` | move | |
| `enter` | detail | |
| `esc` | back | |
| `e` | edit | `e edit` |
| `a` | add | `a add` |
| `d` | del | `d del` |
| `space` | toggle | `space toggle` |
| `/` | filter | |
| `h`, `l`, `left`, `right` | pane | |
| `tab`, `shift+tab` | next/prev pane | |
| `[`, `]` | cycle | |
| `1`-`9`, `0` | jump | |
| `s`, `r`, `?`, `q` | save/reload/help/quit | `s save  r reload  ? help  q quit` |

Keybar: `e edit  a add  d del  space toggle  s save  r reload  ? help  q quit`

### Context `detail`

| Keys | Action | Keybar |
|---|---|---|
| `j`, `k`, `down`, `up` | move | `j/k move` |
| `e` | edit | `e edit` |
| `space` | toggle | `space toggle` |
| `esc` | back | `esc back` |
| `h`, `l`, `left`, `right` | pane | |
| `tab`, `shift+tab` | next/prev pane | |
| `s`, `r`, `?`, `q` | save/reload/help/quit | `s save  r reload  ? help  q quit` |

Keybar: `j/k move  e edit  space toggle  esc back  s save  r reload  ? help  q quit`

### Context `mcp-servers` (Servers list)

| Keys | Action | Keybar |
|---|---|---|
| `j`, `k`, `down`, `up` | move | `j/k move` |
| `enter` | detail | `enter detail` |
| `esc` | back | |
| `e` | edit | `e edit` |
| `a` | add | `a add` |
| `d` | del | `d del` |
| `space` | toggle | `space toggle` |
| `x` | reveal | `x reveal` |
| `/` | filter | |
| `h`, `l`, `left`, `right` | pane | |
| `tab`, `shift+tab` | next/prev pane | |
| `[`, `]` | cycle | `[ ] cycle` |
| `1`-`9`, `0` | jump | `1-9,0 jump` |
| `s`, `r`, `?`, `q` | save/reload/help/quit | `s save  r reload  ? help  q quit` |

Keybar: `j/k move  enter detail  e edit  a add  d del  space toggle  x reveal  [ ] cycle  1-9,0 jump  s save  r reload  ? help  q quit`

### Context `mcp-detail`

| Keys | Action | Keybar |
|---|---|---|
| `j`, `k`, `down`, `up` | move | `j/k move` |
| `e` | edit | `e edit` |
| `space` | toggle | `space toggle` |
| `x` | reveal | `x reveal` |
| `esc` | back | `esc back` |
| `h`, `l`, `left`, `right` | pane | |
| `tab`, `shift+tab` | next/prev pane | |
| `s`, `r`, `?`, `q` | save/reload/help/quit | `s save  r reload  ? help  q quit` |

Keybar: `j/k move  e edit  space toggle  x reveal  esc back  s save  r reload  ? help  q quit`

### Context `mcp-add` (Add MCP server)

| Keys | Action | Keybar |
|---|---|---|
| `enter` | next | `enter next` |
| `esc` | cancel | `esc cancel` |
| `up`, `down` | template | |
| `backspace` | delete | |

Keybar: `enter next  esc cancel`

### Context `mcp-reveal` (Reveal secrets)

| Keys | Action | Keybar |
|---|---|---|
| `esc` | close | `esc close` |
| `enter`, `x` | close | |

Keybar: `esc close`

### Context `picker` (fuzzy model picker)

| Keys | Action | Keybar |
|---|---|---|
| `enter` | select | `enter select` |
| `esc` | cancel | `esc cancel` |
| `tab` | preview | `tab preview` |
| `ctrl+e` | raw | `C-e raw` |
| `up`, `down` | move | `up/dn move` |
| `ctrl+n`, `ctrl+p` | move | |
| `pgup`, `pgdown` | page | |
| `home`, `end` | ends | |
| `backspace` | delete | |
| `ctrl+u` | clear | |

Keybar: `enter select  esc cancel  tab preview  C-e raw  up/dn move`

### Contexts `filter`, `confirm`, `edit`, `add`, `error`, `help`

| Context | Keys | Keybar |
|---|---|---|
| `filter` | `enter` done, `esc` clear, `backspace` delete | `enter done  esc clear` |
| `confirm` | `enter` confirm, `esc` cancel | `enter confirm  esc cancel` |
| `edit` | `enter` apply, `esc` cancel, `backspace` delete | `enter apply  esc cancel` |
| `add` | `enter` add, `esc` cancel, `backspace` delete | `enter add  esc cancel` |
| `error` | `enter` close, `esc` close | `enter close  esc close` |
| `help` | `?` close, `esc` close, `j`/`k`/`down`/`up` scroll | `? close  esc close` |

`ctrl+c` quits from anywhere and is not part of the table.

## 4. Picker overlay (fuzzy model picker)

The picker is the `OverlayPicker`. It opens when `e` is pressed on a detail line whose path matches `^/(models|agents|categories)/[^/]+/model$` (`isPickerPath`). When the rebuilt candidate set is empty, `e` falls back to the plain text edit overlay so `e` always edits.

**Geometry.** The box is `min(110, w-4) × min(30, h-4)` (`pickerBox`). The non-result chrome is 6 lines (top and bottom borders, query line, header, footer, and one spacer), so result rows are `boxHeight - 6`: 14 rows at 80x24 and 24 at 120x40 (`pickerResultRows`, pinned by `TestPickerGeometryContract`).

**Keys.** Printable runes (including `q`, `space`, and digits) extend the query, capped at 128 runes and tail-shown when it overflows (`pickerQueryDisplay`). `backspace` deletes; `ctrl+u` clears; `up`/`down`, `ctrl+p`/`ctrl+n`, `pgup`/`pgdown`, and `home`/`end` move the cursor; `tab` toggles the preview column; `enter` selects; `esc` cancels with zero mutation; `ctrl+c` quits.

**Preview.** The preview column shows `value`, `group` (`alias` or `ref`), `provider`, `used-in` (the document paths that reference the value), and `write-path` (the pointer that would be written). It is on by default when the terminal width is at least 100 columns, and `tab` toggles it. Rows are middle-truncated (`middleTruncate`) so long ids never wrap; the cursor uses `❯`.

**Write-back semantics.** Selecting a row writes the candidate value verbatim through `ConfigSurface.SetScalar(path, value)`. When the selected value equals the value at picker open, the picker closes with `unchanged <path>` and writes nothing. When there is no match, `enter` is inert and the picker stays open. Candidates are rebuilt on every open: the live catalog (`models-store.json`, `models.json`) as primary `provider/id` pairs, plus document aliases with usages and document refs with usages. For a models-section field the alias candidates are filtered out (refs only); agents and categories fields keep alias candidates.

## 5. MCP servers surface

Bound to the first registered source whose schema is `mcpservers` (see [docs/spec-editor-v2.md](spec-editor-v2.md)). When no session is bound the list and detail render `mcp.json unavailable (unreadable or invalid)` and edits report `mcp.json unavailable`.

**List rows.** Name, type (`stdio`/`http`), `enabled`/`disabled`, an auth badge (`⚿<mode>` when the `auth` field is a non-empty string), and a dirty dot.

**Detail rows.** A `name` row (the rename target), then every known schema field in schema order whether or not it is present (so every field is reachable for editing), then any unknown fields preserved in the file (read-only), then the read-only `settings.*` block. `env` and `headers` values render masked as `••• N keys` with a trailing `[x] reveal` hint. `auth` is a mode string, never a secret.

**Flows.**
* `e` on the list edits the endpoint field (command for stdio, url for http). `e` on a detail row edits that field; the `name` row opens the rename flow; bool rows report `use space to toggle bools`; read-only rows report `read-only`.
* `space` toggles `enabled`. Enabling is refused with the runtime's endpoint error when the required command or url is missing.
* `a` starts the add flow: name, then a `stdio`/`http` template, then confirm. The new server is created disabled and selected.
* `d` raises the remove confirm. The confirm notes that only `mcp.json` is edited and `mcp-auth` is never touched.
* `x` opens the reveal overlay, the only settled-state escape hatch for secrets: the raw `env` and `headers` values in key order.
* Rename raises a confirm that carries the orphan-auth warning (credentials in `mcp-auth` are keyed by the old identity).

**Secret policy.** Secret-bearing values are masked in every settled state. Raw values appear only inside the reveal overlay and the explicitly entered edit overlay, and never in evidence files.

## 6. Read-only derived panes

Three panes render data the runtime maintains, read-only, from explicit paths resolved from the agent directory. Every row is provenance `derived`; the detail header names the source file, and the detail lines include a `refreshed` timestamp (the file mtime) and a `source` path. `e`, `a`, `d`, and `space` report `read-only section`. The panes re-read their file when opened, so a changed file reflects on the next open.

| Pane | File | List rows | Detail lines |
|---|---|---|---|
| Connections | `models.json` | key, name, api, model count | name, api, baseUrl, models, refreshed, source |
| Catalog | `models-store.json` | `provider/id`, name | id, name, provider, cost, contextWindow, checkedAt, etag, refreshed, source |
| Tools | `<agentDir>/cache/mcp-cache.json` | server name, tool count | server, tools, fetchedAt, refreshed, source, then one line per tool name/description |

A missing or malformed file renders a diagnostic empty state instead of crashing.

## 7. Multi-document status and save

The status line's left side names the active source path and the total dirty count across every registered source. `s` raises a confirm that lists each dirty file with its own `dirty(n)` count; a file that changed on disk since load is marked `stale, blocked`. Confirming runs `SaveAll`, which saves each dirty file in registry order with its own atomic write and timestamped backup, continues past a stale or failing file, and reports every backup in status. `r` reloads every session from disk; when dirty it asks for confirmation first.

## 8. Non-goals v2

* No mouse, themes, keybind configuration, plugin system, or ASCII-only mode.
* No `model_profile` picker: `model_profile` renders read-only (see [docs/spec-editor-v2.md](spec-editor-v2.md)).
* No project-layer rendering this wave: the nearest project `.omo/omo.jsonc` is registered read-only but not shown.
* No merged MCP view: the surface shows the global `<agentDir>/mcp.json` only; runtime pickup of a merged view is senpi's concern.
* No writes from any derived pane, and no writes outside the user `omo.jsonc` and the global `mcp.json`.
* No network access and no credential editing; `auth.json`, `mcp-auth/`, and the other credential files are never read or written.
