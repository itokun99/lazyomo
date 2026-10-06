# lazyomo v2 Editor Scope Spec

> Supersedes [docs/spec-editor-v1.md](spec-editor-v1.md). v1 described a single-document editing surface. v2 adds the `git_master` section, real read-only rendering for every other present top-level key, the `model_profile` pin rule, and the second editable document (`mcp.json`), and it corrects the live-key list below.

Source facts (2026-10-06): the live `~/.omo/omo.jsonc` carries exactly 9 top-level keys, verified by reading the file and cross-checking the editor's section code in this repo: `$schema`, `models`, `model_profiles`, `model_profile`, `agents`, `categories`, `git_master`, `telemetry`, `_migrations`. The keys `memory`, `task`, `teams`, `disabled_skills`, `profiles`, and the `[opencode]`/`[native]`/`[codex]` blocks that v1 listed are not present in the current live file. Engine `internal/omodit` exposes `Load`, `Get(pointer)`, `Set(pointer,value)`, `Add`, `Remove`, `Save` (comment-preserving, atomic, timestamped backup). Credentials live in `~/.omo/agent/` and stay out of scope, always.

The editor handles two kinds of sections. Editable sections are `models`, `model_profiles`, `agents`, `categories`, `telemetry`, and `git_master`. Every other present top-level key becomes its own read-only section, sorted by key; with the live file that is `$schema`, `model_profile`, and `_migrations`.

## 1. Editable surface

Everything not listed here is read-only. The editor never writes unknown keys, never drops comments, and never touches credentials.

| Section | Editable fields | Value kinds + validation | Read-only in v2 |
|---|---|---|---|
| `models.*` | Add alias, remove alias, edit `model`, edit `reasoning` | Alias: `^[a-zA-Z0-9_-]+$`, unique, max 32 chars. `model`: non-empty `<provider>/<model-id>` string. `reasoning`: one of `low`, `medium`, `high`, `max`, or empty (unset). Refuse empty `model` on save. | Provider auth, pricing, per-provider params |
| `model_profiles.*` | Add profile, remove profile, edit `display_name`, edit `models` chain | Profile key: same alias rules. `display_name`: non-empty string, max 64 chars. `models`: non-empty array of `provider/model[:suffix]` or `alias[:suffix]` strings, kept verbatim in order, no dedup. | Profile scheduling, weights, conditions |
| `model_profile` | Set to an existing profile key, or to a literal `<provider>/<model-id>` pin, or clear | String. A pin matches `modelRefPattern` and always contains a slash. Clearing is blocked while `model_profiles` is non-empty. Rendered in the read-only section (see below). | Nothing else, it is a scalar |
| `agents.*` | Edit `model`, `models` chain, `reasoning`, `disable`; add or remove overlay entry | `model`: alias referencing an existing `models.*` key, or a full `<provider>/<name>` string. `models`: array like profiles. `reasoning`: same enum. `disable`: boolean, default false. | Prompts, tools, permissions, descriptions |
| `categories.*` | Edit `model`, `models` chain | Same rules as `agents.*` minus `disable`. Unknown category names are rejected; the set is fixed at 10. | Category definitions, routing weights |
| `telemetry.enabled` | Boolean toggle | Strict boolean. Missing block is created as `{"enabled": bool}` on first toggle. | Endpoint, sampling, payload shape |
| `git_master.*` | `commit_footer`, `include_co_authored_by` toggles | Strict booleans, both default false. The block is created on the first toggle; the editor writes only the touched key. | Hooks, signing, remote config |

Global rules: alias deletes are blocked while referenced by `agents.*`, `categories.*`, or `model_profiles.*` chains, and the error names the referrer. Profile deletes are blocked while selected by `model_profile`. `Save` runs after each confirmed edit, with atomic write plus timestamped backup. Failed validation leaves the file untouched and shows the field error inline.

### `git_master`

A dedicated section with exactly two boolean rows, `commit_footer` and `include_co_authored_by`, both shown even when the block is absent and defaulting to `false`. `space` on a row toggles it and creates the `git_master` block when it does not exist. The detail view of a row exposes only that toggle; `e`, `a`, and `d` there report `read-only section`. `SetScalar` on a `git_master` path is not accepted; booleans are written through `ToggleBool` only.

### `model_profile`

The value is either an existing `model_profiles` key or a literal `<provider>/<model-id>` pin. An unknown string with no slash is rejected (`unknown model profile "..."`). Clearing to an empty string is blocked while any profile exists. The live value renders in the read-only section, so the active profile or pin is always visible.

The `model_profile` picker is not shipped in this wave. A model picker ships for `model` fields (see [docs/spec-tui-v2.md](spec-tui-v2.md)); `model_profile` is display-only here.

## 2. Read-only sections

Every present top-level key outside the editable set becomes a read-only section (`Section.ReadOnly = true`). Each carries a single entry that previews the key's value; `Detail` expands an object value into one read-only line per member in sorted key order, and renders any other JSON type as a single read-only line. Selection, filtering, and help still work, but `e`, `a`, `d`, and `space` report `read-only section` instead of opening a popup. With the live file the read-only sections are `$schema`, `model_profile`, and `_migrations`.

## 3. Two-document model

The workspace (`internal/workspace`) resolves a registry of configuration sources, each with a provenance: `Editable`, `ReadOnly`, or `Derived`.

| Source | Path | Schema | Provenance | Notes |
|---|---|---|---|---|
| User config | `~/.omo/omo.jsonc` | `omo` | Editable | Edited through `internal/editor`. Missing or invalid is fatal at startup. |
| Project config | nearest `.omo/omo.jsonc` (or `.omo/omo.json`) from the working directory up to `$HOME` | `omo` | Read-only | Registered read-only and NOT rendered this wave. Symlinked directories and files are skipped. |
| MCP file | `<agentDir>/mcp.json` | `mcpservers` | Editable | Edited through `internal/mcpfile`. A missing file registers as the creation target; the first add creates it. |

Saving is per file: `SaveAll` walks the dirty sessions in registry order, each file getting its own atomic write and timestamped backup, and continues past a stale or failing file. A stale guard records each file's mtime and size at load, attach, save, or reload, and refuses to save a file that changed on disk since (bytes untouched, dirty set kept).

## 4. `mcp.json` surface (`internal/mcpfile`)

The MCP servers file is the second editable document. `internal/mcpfile` edits it through `internal/omodit` and mirrors the runtime's structural rules.

**Agent directory resolution.** `OMO_CODING_AGENT_DIR` > `SENPI_CODING_AGENT_DIR` > `PI_CODING_AGENT_DIR` > `~/.omo/agent`.

**Strict JSON.** The runtime reads this file with strict `JSON.parse` semantics. A file carrying comments or trailing commas is rejected on load with a clear error, and every save is validated as strict JSON before it touches disk.

**Validation (senpi-mirrored).**
* Type inference: an explicit `type` wins; otherwise a non-empty `url` means `http`, anything else `stdio`.
* Endpoint rule: an enabled stdio server requires a non-blank `command`; an enabled http server requires a non-blank `url`. Disabled servers are exempt.
* Interpolation: `${VAR}` and `${VAR:-default}` are preserved verbatim and accepted. A value that starts with `!` after leading whitespace is rejected (mirroring senpi's `trimStart().startsWith("!")`), and a value containing `$(` is rejected.
* `env` and `headers` are string-to-string maps.
* Unknown fields already present on a server are preserved untouched; new unknown members cannot be created. Enum-like fields (`lifecycle`, `exposure`, `logLevel`, `auth`, and others) accept any string, with the known values offered as suggestions, so schema drift never blocks an edit.

**Ops.** `SetField`, `ToggleEnabled`, `AddServer(name, type)`, `RemoveServer`, and `RenameServer`. Adding a server creates it disabled so the endpoint rule holds from the moment it exists; the caller configures the command or url and then enables it. Renaming preserves every field and returns an orphan-auth warning: OAuth tokens live in `mcp-auth/` keyed by the server's old identity, so a rename can orphan them. `mcp-auth/` is never read or written.

**Saving.** Saves go through `omodit` (atomic, timestamped backup). A session loaded from a missing file starts empty; the first `AddServer` creates the file, and that first save creates no backup because the file did not exist before.

Note: v1 displays the global `<agentDir>/mcp.json` only. Runtime pickup of a merged or effective MCP view (for example from a project `.senpi/mcp.json` or `.mcp.json`) is senpi's concern and is out of scope.

## 5. Non-goals v2

* No `model_profile` picker this wave; `model_profile` renders read-only.
* No project-layer rendering this wave (registry-only); no writes to project files.
* No merged MCP view; no writes to any new config location, and no merging `mcp.json` into `omo.jsonc`.
* No credential or provider auth editing; `auth.json`, `mcp-auth/`, and the other credential files are never read or written.
* No free-form JSON editing, backup-restore UI, diff UI, schema migration tooling, or remote sync.
* No network access at runtime.
