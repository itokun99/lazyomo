# Command Guidelines - lazyomo

## Overview

v1 has no switcher verbs. The launcher does two things: no args opens the editor, `--help` or `-h` prints usage. Unknown args fail with usage text. This file records that surface plus the style to keep if edit verbs return.

## Current Launcher (cmd/lazyomo/main.go)

```go
// no args: editor.LoadDefault() (~/.omo/omo.jsonc), then tui.New(ed)
// --help, -h: print usage to stdout
// anything else: return error with usage text, main prints Error: ... to stderr
```

Usage text:

```
lazyomo - editor for ~/.omo/omo.jsonc

Usage:
  lazyomo           open the interactive editor
  lazyomo --help    print this help
```

Do not document `--list`, `--current`, `show <alias>`, bare-alias switching, or `--cli`. Those belonged to the deleted switcher and no longer exist.

## If a Verb Returns

Keep manual dispatch, no Cobra or other CLI frameworks. The spec sketches future edit verbs (`get`, `set`, `add`, `rm`, `move`) routed through `internal/editor` per `docs/spec-editor-v1.md` section 2. Until one lands, this stays a sketch, not a claim:

```go
func run(args []string) error {
    switch args[0] {
    case "--help", "-h":
        fmt.Fprint(os.Stdout, usage)
        return nil
    // case "get": return cmdGet(...)
    default:
        return fmt.Errorf("unknown args %q\n\n%s", args, usage)
    }
}
```

Conventions for any future verb:

| Element | Convention | Example |
|---------|------------|---------|
| Function name | `cmd` + CamelCase | `cmdGet` |
| Flags | `--kebab-case`, short `-x` only when needed | `--help`, `-h` |
| Output | Normal text to stdout | Usage or values |
| Errors | `Error: ...` to stderr, nonzero exit | `return fmt.Errorf(...)` |
| Routing | Through `internal/editor` ops | Never direct `omodit` or file writes |

## Error Handling

```go
if err != nil {
    fmt.Fprintf(os.Stderr, "Error: %v\n", err)
    os.Exit(1)
}
```

Unknown input echoes usage so the user sees the two valid forms.

## Testing the Launcher

Table-driven tests beside the changed package, stdlib `testing` only. Cover: no args path needs a config (use temp dirs through `editor.Load`, not the live file), `--help` prints usage, unknown args return an error containing usage.

## Command Reference

| Input | Behavior |
|-------|----------|
| `lazyomo` | Open the TUI editor on `~/.omo/omo.jsonc` |
| `lazyomo --help`, `lazyomo -h` | Print usage |
