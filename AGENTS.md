# PROJECT KNOWLEDGE BASE

**Generated:** 2026-10-05 (post-refactor: lazyomo editor)
**Branch:** main
**Release:** v3.0.0 (GitHub release with 6 platform assets); npm `@itokun99/lazyomo`; Homebrew tap `itokun99/homebrew-lazyomo`

## OVERVIEW
lazyomo - TUI editor for `~/.omo/omo.jsonc` (oh-my-openagent user config) and `<agentDir>/mcp.json`, styled after lazygit. Go 1.26.3 + Charm stack (Bubble Tea / Bubbles / Lipgloss). Chain: `cmd/lazyomo -> internal/tui -> internal/workspace -> {internal/editor, internal/mcpfile} -> internal/omodit`; read-only panes read `internal/derived`, and the model picker ranks with `internal/fuzzy`. Two documents are editable: the user `omo.jsonc` as comment-preserving JSONC, and `mcp.json` as strict JSON. Saves are per file, atomic, and leave a timestamped backup. A nearest project `.omo/omo.jsonc` is registered read-only and not rendered.

## STRUCTURE
```
lazyomo/
├── cmd/lazyomo/            # entrypoint: builds the workspace registry, attaches the mcp.json session, runs tui.New + tea program; --help prints usage
├── internal/omodit/        # JSONC engine (hujson): Load/Get/Set/Add/Remove/Save (atomic + backup), comment-preserving, RFC6901 pointers
├── internal/editor/        # user omo.jsonc model: editable + read-only sections, validation, reference-blocking, dirty tracking, Save/Reload
├── internal/mcpfile/       # strict-JSON mcp.json session: senpi-mirrored validation, add/edit/toggle/rename/remove, masked secrets
├── internal/workspace/     # source registry + provenance, per-file SaveAll with a stale guard, model-candidate builder
├── internal/derived/       # read-only readers for models.json, models-store.json, mcp-cache.json
├── internal/fuzzy/         # stdlib-only fuzzy matcher for the model picker
├── internal/tui/           # Bubble Tea UI: grouped side panel + bordered panes + one binding table (docs/spec-tui-v2.md is the contract)
├── bin/lazyomo.js          # npm launcher (spawns the packaged binary; prints an install hint when missing)
├── scripts/install.js      # npm postinstall: downloads lazyomo-<goos>-<goarch> from the release matching package.json version
├── Formula/lazyomo.rb      # Homebrew formula (sha256 per platform; update after each release)
└── docs/spec-*.md          # spec-tui-v2.md + spec-editor-v2.md (decision-complete contracts; v1 files are superseded)
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Config path resolution | internal/editor/editor.go (`LoadDefault`) | `$HOME/.omo/omo.jsonc`; tests use `Load(path)` with temp copies |
| JSONC edits (set/add/remove) | internal/omodit/omodit.go | never rewrite the whole document; comments/formatting outside the edited subtree survive |
| Backup + atomic write | internal/omodit (`Save`) | temp file + rename; backup `<file>.bak.<YYYY-MM-DDTHH-mm-ss-SSSZ>`; never clobbers an existing backup |
| Editable surface + validation rules | docs/spec-editor-v2.md; internal/editor/validate.go | models / model_profiles / model_profile / agents / categories / telemetry / git_master; every other present top-level key renders read-only |
| MCP servers file | internal/mcpfile/ | strict JSON; senpi-mirrored validation; add/edit/toggle/rename/remove; atomic save + backup |
| Source registry + save-all | internal/workspace/ | provenance Editable/ReadOnly/Derived; per-file save + stale guard; model candidates |
| Read-only derived panes | internal/derived/ | models.json / models-store.json / mcp-cache.json readers; never written |
| TUI layout + keymap | docs/spec-tui-v2.md; internal/tui/bindings.go, view.go | one binding table drives dispatch + keybar + help; grouped side panel; model picker |
| Release pipeline | .github/workflows/release.yml | tag `v*` -> vet + test -> matrix build -> GitHub Release (softprops) |

## CODE MAP
| Symbol | Location | Role |
|--------|----------|------|
| Editor | internal/editor/editor.go | Load/LoadDefault; Sections/Detail; SetScalar/ToggleBool/AddEntry/RemoveEntry/MoveChain; DirtyPaths; Save/Reload |
| Document | internal/omodit/omodit.go | Load/Bytes/Get/Set/Add/Remove/Save over the hujson AST |
| Registry | internal/workspace/registry.go | NewRegistry/Sources/SaveAll/Reload/Stale; provenance rows |
| MCPSession | internal/mcpfile/mcpfile.go | Load/Servers/SetField/ToggleEnabled/AddServer/RemoveServer/RenameServer/Save |
| Derived readers | internal/derived/derived.go | LoadProviders/LoadCatalog/LoadTools (read-only snapshots) |
| Fuzzy | internal/fuzzy/fuzzy.go | Filter/NewCandidate (deterministic fuzzy ranking) |
| Model / update / view | internal/tui/model.go, update.go, view.go | Bubble Tea model, dispatch, overlays, grouped-pane rendering |
| Bindings | internal/tui/bindings.go | the one binding table (dispatch + keybar + help) |

## CONVENTIONS
- Errors: `fmt.Errorf("verb-ing noun: %w", err)`; no custom error types, no sentinels.
- Tests: stdlib `testing` only; table-driven + `t.Run`; temp dirs for all file work; fixtures under `testdata/`.
- TUI testability: declare consumer-side interfaces in `internal/tui` (`Workspace`, `ConfigSurface`, `MCPSurface`); wire the real `*workspace.Registry`, `*editor.Editor`, and `*mcpfile.Session` only in `cmd/lazyomo/main.go`.
- Two editable documents: the user `~/.omo/omo.jsonc` (comment-preserving JSONC via internal/editor) and `<agentDir>/mcp.json` (strict JSON via internal/mcpfile; agentDir from `OMO_CODING_AGENT_DIR` > `SENPI_CODING_AGENT_DIR` > `PI_CODING_AGENT_DIR` > `~/.omo/agent`). The nearest project `.omo/omo.jsonc` is registered read-only and not rendered; derived panes are read-only.
- JSONC invariants (do not regress): comments and trailing commas survive; untouched content is byte-identical (`TestRealConfigRoundTrip`); invalid input returns an error instead of writing broken output.
- Real config in tests: NEVER edit `~/.omo/omo.jsonc` in tests; copy to a temp path and use `LAZYOMO_REAL_CONFIG` for the opt-in integration test.
- Credentials live in `~/.omo/agent/` (auth.json etc.) - out of scope; never read or write.
- Commits: Conventional Commits (`feat:`, `fix:`, `chore:` ...), lowercase subject.

## ANTI-PATTERNS (THIS PROJECT)
| Violation | Why it's wrong | Where it bit/will bite |
|-----------|----------------|------------------------|
| Reintroducing switcher packages (`internal/domain`, `internal/application`, `internal/infrastructure`, `internal/cli`) | deleted on purpose; the editor replaces them | do not recreate ConfigService / KnownGroups / omo_configs paths |
| Rewriting the whole config on save | destroys comments and user formatting | always go through `internal/omodit` |
| Unanchored ignore patterns in `.gitignore` | a bare `lazyomo` pattern once hid `cmd/lazyomo/` from git | keep `/lazyomo` root-anchored |
| Editing `~/.omo/agent/*` | credentials plane | the editor must never touch it |
| Adding a CLI framework | entrypoint stays a tiny manual dispatch | small repo |
| Tests against the live user config | mutates user state | temp copies + env-gated integration test |

## COMMANDS
```bash
go build ./... && go vet ./... && go test ./...     # full gate
go build -o lazyomo ./cmd/lazyomo && ./lazyomo      # run the editor

# opt-in real-data round-trip proof (uses a COPY of the real config):
cp ~/.omo/omo.jsonc /tmp/omo.jsonc
LAZYOMO_REAL_CONFIG=/tmp/omo.jsonc go test ./internal/omodit -run TestRealConfigRoundTrip -v
```

## NOTES
- Release flow: bump `package.json` version -> tag `vX.Y.Z` -> push the tag -> CI builds assets and creates the GitHub Release -> update `Formula/lazyomo.rb` sha256 from the released assets -> sync the tap repo (`itokun99/homebrew-lazyomo`).
- npm: `npm publish --access public` from this repo (scope `@itokun99`); postinstall downloads the binary matching `package.json` version; the JS launcher is intentionally minimal (no fallback implementation).
- Homebrew: `brew tap itokun99/lazyomo && brew install lazyomo`; keep the tap's formula in sync with `Formula/lazyomo.rb` here.
- History: the repo was renamed from `itokun99/omo-switcher`; the old name survives only in CHANGELOG history. The old `itokun99/homebrew-omo-switch` tap is obsolete.

## Related Documentation

- [ai-workflow.md](.agents/rules/ai-workflow.md) - Development workflow
- [cli-architecture.md](.agents/rules/cli-architecture.md) - CLI / entrypoint architecture
- [go-standards.md](.agents/rules/go-standards.md) - Go engineering standards
- [testing-guide.md](.agents/rules/testing-guide.md) - Testing standards
- [provider-integration-guide.md](.agents/rules/provider-integration-guide.md) - omo config surface integration
- [command-guidelines.md](.agents/rules/command-guidelines.md) - Command creation guide
- [bugfix-workflow.md](.agents/rules/bugfix-workflow.md) - Bug investigation
- [refactor-workflow.md](.agents/rules/refactor-workflow.md) - Refactoring guide
- [contributing-guide.md](.agents/rules/contributing-guide.md) - Developer onboarding
