# lazyomo v1 Editor Scope Spec

Source facts (2026-10-05): live config keys are `$schema`, `models`, `model_profiles`, `model_profile`, `agents`, `categories`, `telemetry`, `git_master`, `memory`, `task`, `teams`, `disabled_skills`, `profiles`, `[opencode]/[native]/[codex]`, `_migrations`. Live data holds 12 model aliases, 3 profiles, 6 agents, 10 categories, `telemetry.enabled=false`, 2 `git_master` booleans. Engine `internal/omodit` exposes `Load`, `Get(pointer)`, `Set(pointer,value)`, `Add`, `Remove`, `Save` (comment-preserving, atomic, timestamped backup). Credentials live in `~/.omo/agent/` and stay out of scope, always.

## 1. V1 Editable Surface

Everything not listed here is read-only in v1. The editor never writes unknown keys, never drops comments, never touches credentials.

| Section | Editable fields | Value kinds + validation | Read-only in v1 |
|---|---|---|---|
| `models.*` | Add alias, remove alias, edit `model`, edit `reasoning` | Alias: `^[a-zA-Z0-9_-]+$`, unique, max 32 chars. `model`: non-empty `provider/name` string, must match `<provider>/<model-id>` shape. `reasoning`: one of `low`, `medium`, `high`, `max`, or empty (unset). Refuse empty `model` on save. | Provider auth, pricing, per-provider params |
| `model_profiles.*` | Add profile, remove profile, edit `display_name`, edit `models` chain | Profile key: same alias rules as above. `display_name`: non-empty string, max 64 chars. `models`: non-empty array of `provider/model:suffix` strings (suffix like `:max` optional but kept verbatim). Min length 1. No dedup, order kept. | Profile scheduling, weights, conditions |
| `model_profile` | Selector for active profile | String, must equal an existing key in `model_profiles`. Clearing it is blocked while profiles exist. Changing it never edits profile bodies. | Nothing else, it's a scalar |
| `agents.*` | Edit `model`, `models` chain, `reasoning`, `disable`; add or remove overlay entry | `model`: alias referencing an existing `models.*` key, or full `provider/name` string. `models`: array like profiles, optional fallback chain. `reasoning`: same enum as models. `disable`: boolean, default false. New overlay keys follow alias rules; built-in agent names can't be renamed, just overlaid or disabled. | Prompts, tools, permissions, descriptions |
| `categories.*` | Edit `model`, `models` chain | Same rules as `agents.*` minus `disable`. `model` must resolve to a known alias or full `provider/name`. Empty chain falls back to `model`. Unknown category names are rejected, the set is fixed at 10. | Category definitions, routing weights |
| `telemetry.enabled` | Boolean toggle | Strict boolean. No other `telemetry` child is editable. Missing block is created as `{"enabled": bool}` on first toggle. | Endpoint, sampling, payload shape |
| `git_master.*` | `commit_footer`, `include_co_authored_by` toggles | Strict booleans. Both default false when absent. Editor writes only the touched key. | Hooks, signing, remote config |

Why this set: it covers what the live config actually changes day to day (aliases, chains, per-agent routing, two toggle groups) and it maps 1:1 onto `Get`/`Set`/`Add`/`Remove` pointers. Keys like `memory`, `task`, `teams`, `profiles`, provider blocks, `$schema`, and `_migrations` change rarely, carry migration risk, and don't earn v1 UI.

Global v1 rules: alias deletes are blocked while referenced by `agents.*`, `categories.*`, or `model_profiles.*` chains (editor offers retarget first). Profile deletes are blocked while selected by `model_profile`. `Save` runs after each confirmed edit, with atomic write plus timestamped backup. Failed validation leaves the file untouched and shows the field error inline.

## 2. Engine Contract for UI

Each row names the UI control, the pointer it touches, and the omodit op behind it.

| UI element | Pointer | Omodit op |
|---|---|---|
| Alias list, add alias | `/models/<alias>` | `Add` then `Save` |
| Alias list, remove alias | `/models/<alias>` | `Remove` then `Save` (blocked if referenced) |
| Alias form, model string | `/models/<alias>/model` | `Get` to fill, `Set` to write, `Save` |
| Alias form, reasoning | `/models/<alias>/reasoning` | `Get` to fill, `Set` (or `Remove` when unset), `Save` |
| Profile list, add or remove | `/model_profiles/<name>` | `Add` or `Remove` then `Save` |
| Profile form, display name | `/model_profiles/<name>/display_name` | `Get`/`Set` then `Save` |
| Profile form, models chain rows | `/model_profiles/<name>/models` | `Get` whole array, `Set` whole array, `Save` |
| Active profile picker | `/model_profile` | `Get` for current, `Set` to switch, `Save` |
| Agent row, model or reasoning | `/agents/<name>/model`, `/agents/<name>/reasoning` | `Get`/`Set` then `Save` |
| Agent row, fallback chain | `/agents/<name>/models` | `Get`/`Set` whole array, `Save` |
| Agent row, disable switch | `/agents/<name>/disable` | `Set` bool, `Save` |
| Category row, model or chain | `/categories/<name>/model`, `/categories/<name>/models` | `Get`/`Set` then `Save` |
| Telemetry switch | `/telemetry/enabled` | `Get`/`Set` bool, `Save` |
| Git master switches | `/git_master/commit_footer`, `/git_master/include_co_authored_by` | `Get`/`Set` bool, `Save` |

Gap: chain row reorder (move profile model up or down, reorder fallback chains) isn't expressible through `Get`/`Set`/`Add`/`Remove` without a read-modify-write of the full array from UI code. That's workable but it pushes merge logic into the view. Minimal addition: `Move(pointer, fromIndex, toIndex)` on array pointers, validated bounds, single undo step, implemented as one array splice before `Save`.

## 3. Refactor Map for P3 (Informational)

| Package | Verdict | Reason |
|---|---|---|
| `internal/domain` | Morph | Keep pure types and validation shape, retarget from switch groups to editable sections and field rules above |
| `internal/infrastructure` | Morph | Keep atomic write and backup habits, swap file-switch ops for omodit-backed `Load`/`Get`/`Set`/`Add`/`Remove`/`Save` |
| `internal/application` | Morph | Keep service orchestration, replace switch flow with edit, validate, blocked-delete, and save flows |
| `internal/cli` | Keep | Keep manual dispatch style, add edit verbs (`get`, `set`, `add`, `rm`, `move`) per section 2 |
| `internal/tui` | Keep | Keep Bubble Tea shell and status bar pattern, replace switch views with section 1 forms |
| `internal/tui/components` | Keep | Reuse styles and leaf renderers, add field rows, chain rows, and toggle rows |
| `cmd/lazyomo` | Keep | Keep entry wiring as is, it just serves the new service |
| `bin/lazyomo.js` | Delete | Drop the JS switcher duplicate, one Go editor owns the surface so paths can't skew again |

## 4. Non-Goals for v1 + Open Questions

Non-goals: credential or provider auth editing; `memory`, `task`, `teams`, `profiles`, and provider block editing; `$schema` or `_migrations` editing; free-form JSON editing; multi-file switch, backup restore UI, or diff UI; schema migration tooling; remote sync.

Open questions:
1. Should alias delete retarget references automatically or always ask?
2. Should `categories.*` accept custom names, or stay fixed at 10?
3. Should `disable` exist on categories too, or agents just?
4. Should `Move` also cover string arrays outside chains, or chains just?
5. Should toggles write missing parent blocks, or prompt first?
