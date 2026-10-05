# Provider Integration Guide - lazyomo

## Overview

lazyomo edits one file, `~/.omo/omo.jsonc`. Providers appear as `provider/model` strings inside `models`, `model_profiles` chains, `agents`, and `categories`. There is no per-provider config file, no discovery directory, and no switch flow. To point an agent at a new provider, you edit the alias or chain that agent resolves to.

## Current Model

```
~/.omo/omo.jsonc
├── models.<alias>.model            # "provider/model-id", e.g. "acme/code-large"
├── models.<alias>.reasoning        # low, medium, high, max, or empty
├── model_profiles.<name>.models[]  # fallback chain of "provider/model:suffix"
├── model_profile                   # active profile key
├── agents.<name>.model             # alias or full "provider/model-id"
├── agents.<name>.models[]          # optional fallback chain
└── categories.<name>.model         # alias or full "provider/model-id"
```

Credentials live under `~/.omo/agent/` and are never touched by the editor. Provider auth, pricing, and per-provider params are read-only in v1.

## Pointing an Agent at a New Provider

### Step 1: Add or reuse a model alias

In the TUI: select Models, press `a`, name the alias (`^[a-zA-Z0-9_-]+$`, max 32 chars), then edit its `model` field to the full `provider/model-id` string.

Rules for `model` values: non-empty, must match `<provider>/<model-id>` shape. Chain elements allow an optional `:suffix` kept verbatim (for example `acme/code-large:max`).

### Step 2: Route the agent or category to it

Edit `/agents/<name>/model` to the alias or to the full `provider/model-id` string. Optionally set `/agents/<name>/models` to a fallback chain (min length 1 when present, order kept, no dedup). Categories work the same minus `disable`: `/categories/<name>/model` plus optional `/categories/<name>/models`.

Unknown category names are rejected in v1. The set is fixed at 10: `architect`, `artistry`, `deep-high`, `deep-low`, `quick`, `ultrabrain`, `unspecified-high`, `unspecified-low`, `visual-engineering`, `writing`.

### Step 3: Save

Press `s`, confirm, and the editor validates, writes a timestamped backup next to the config, then saves atomically. Bad values keep the popup open with a one-line reason and the file stays untouched.

## Value Rules

| Field | Rule |
|-------|------|
| Alias or profile key | `^[a-zA-Z0-9_-]+$`, unique in section, max 32 chars |
| `model` | Non-empty `provider/model-id`; agents and categories also accept an existing alias |
| Chain element | `provider/model` with optional `:suffix`, kept verbatim |
| `reasoning` | One of `low`, `medium`, `high`, `max`, or empty (unset) |
| `display_name` | Non-empty string, max 64 chars |
| `model_profile` | Must equal an existing `model_profiles` key |
| `telemetry.enabled` | Strict bool, toggled with `space` |

Deletes are blocked while referenced: an alias in use by `agents`, `categories`, or a profile chain can't be removed until retargeted, and the selected profile can't be deleted while `model_profile` points at it.

## Validation

`internal/editor/validate.go` owns these rules, enforced before `omodit` is touched. To extend them, add a check there plus a table case in the editor tests, and keep the convention: invalid input returns an inline-displayable error and leaves the document untouched.

## Integration Testing

```bash
# Build and run the editor against a copy
go build -o lazyomo ./cmd/lazyomo
cp ~/.omo/omo.jsonc /tmp/omo-test.jsonc

# Exercise the engine in tests
go test ./internal/editor/... ./internal/omodit/...
```

Manual path: open the TUI, add an alias, point one agent at it, press `s`, confirm the backup sibling appears, then `r` or reopen to confirm the values stuck. Never test against credentials; `~/.omo/agent/` stays out of scope.

## Troubleshooting

### Edit rejected

Check the inline reason: alias shape, `provider/model-id` shape, empty chain, unknown category, or `model_profile` with no matching profile. Fix the value, confirm again.

### Delete blocked

The entry is still referenced. Retarget the agents, categories, or chains that point at it first, then delete.

### Save left no backup

Save only writes when dirty. A clean save flashes `already saved` and skips both backup and write. Make an edit first.

### File not found on launch

`editor.LoadDefault` reads `~/.omo/omo.jsonc` via `os.UserHomeDir`. Create or restore that file first; the editor doesn't provision a fresh config in v1.

## Reference

- [cli-architecture.md](cli-architecture.md) - Chain, packages, and save flow
- [go-standards.md](go-standards.md) - Go coding standards
- `docs/spec-editor-v1.md` - Editable scope and engine contract
- `docs/spec-tui-v1.md` - Panes and keymap
