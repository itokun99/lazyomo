package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/itokun99/lazyomo/internal/editor"
)

const (
	omoConfigName     = "omo.jsonc"
	omoConfigJSONName = "omo.json"
	mcpConfigName     = "mcp.json"
	// maxProjectWalkDepth mirrors the upstream walk guard in
	// packages/omo-config-core/src/loader/paths.ts.
	maxProjectWalkDepth = 256
)

// RegistryConfig supplies the roots the registry derives source paths from.
// Callers resolve them; environment-variable handling for the agent
// directory is out of scope for this package.
type RegistryConfig struct {
	HomeDir  string
	Cwd      string
	AgentDir string
}

// Registry is the resolved set of configuration sources. Build one with
// NewRegistry and rebuild it to observe file changes; a zero Registry is
// empty and usable.
type Registry struct {
	sources     []Source
	diagnostics []Diagnostic
	snapshots   map[string]fileSnapshot
}

// NewRegistry resolves the default source set: the user omo.jsonc, the
// nearest project-layer omo.jsonc, and <AgentDir>/mcp.json. Every layer
// that fails to register becomes a diagnostic, never an error.
func NewRegistry(cfg RegistryConfig) *Registry {
	reg := &Registry{}
	reg.loadUser(cfg)
	reg.loadProject(cfg)
	reg.loadMCP(cfg)
	return reg
}

// Sources returns the registered sources, in registration order.
func (r *Registry) Sources() []Source {
	return append([]Source(nil), r.sources...)
}

// Diagnostics returns one diagnostic per layer that failed to register.
func (r *Registry) Diagnostics() []Diagnostic {
	return append([]Diagnostic(nil), r.diagnostics...)
}

// Source returns the source registered under id.
func (r *Registry) Source(id string) (Source, bool) {
	for _, source := range r.sources {
		if source.ID == id {
			return source, true
		}
	}
	return Source{}, false
}

func (r *Registry) add(source Source) { r.sources = append(r.sources, source) }

func (r *Registry) skip(diagnostic Diagnostic) {
	r.diagnostics = append(r.diagnostics, diagnostic)
}

// loadUser registers the writable user config through the editor.
func (r *Registry) loadUser(cfg RegistryConfig) {
	path := filepath.Join(cfg.HomeDir, ".omo", omoConfigName)
	ed, err := editor.Load(path)
	if err != nil {
		r.skip(Diagnostic{SourceID: IDUser, Path: path, Reason: err.Error()})
		return
	}
	r.add(Source{
		ID:       IDUser,
		Path:     path,
		Kind:     KindUser,
		Schema:   SchemaOmo,
		Writable: true,
		Session:  ed,
	})
	r.remember(IDUser, path)
}

// loadProject registers the nearest project-layer config as a read-only
// source. The walk mirrors the upstream loader (paths.ts): from Cwd up to
// HomeDir (exclusive; the home .omo is the user layer and must not count
// twice), trying .omo/omo.jsonc then .omo/omo.json at each level, skipping
// symlinked directories and files, and continuing past skips.
func (r *Registry) loadProject(cfg RegistryConfig) {
	path, skips := findProjectConfig(cfg)
	for _, skipped := range skips {
		r.skip(Diagnostic{SourceID: IDProject, Path: skipped.path, Reason: skipped.reason})
	}
	if path == "" {
		r.skip(Diagnostic{
			SourceID: IDProject,
			Reason:   fmt.Sprintf("no loadable project config found from %s up to %s", cfg.Cwd, cfg.HomeDir),
		})
		return
	}
	// Parse-validate through the editor, the package chain's JSONC entry
	// point; the loaded document is discarded because the project layer is
	// display-only and keeps Session nil.
	if _, err := editor.Load(path); err != nil {
		r.skip(Diagnostic{SourceID: IDProject, Path: path, Reason: err.Error()})
		return
	}
	r.add(Source{
		ID:       IDProject,
		Path:     path,
		Kind:     KindProject,
		Schema:   SchemaOmo,
		Writable: false,
	})
}

// projectSkip records a candidate the walk refused because it is a symlink.
type projectSkip struct {
	path   string
	reason string
}

// findProjectConfig returns the nearest loadable project config path and any
// symlinked candidates that were skipped on the way up.
func findProjectConfig(cfg RegistryConfig) (string, []projectSkip) {
	var skips []projectSkip
	realHome := realOrSelf(cfg.HomeDir)
	dir := cfg.Cwd
	for depth := 0; depth < maxProjectWalkDepth; depth++ {
		if dir == cfg.HomeDir || realOrSelf(dir) == realHome {
			break
		}
		omoDir := filepath.Join(dir, ".omo")
		if isSymlink(omoDir) {
			skips = append(skips, projectSkip{path: omoDir, reason: "skipped symlinked .omo directory"})
		} else {
			for _, name := range []string{omoConfigName, omoConfigJSONName} {
				candidate := filepath.Join(omoDir, name)
				if exists(candidate) && !isSymlink(candidate) {
					return candidate, skips
				}
				if isSymlink(candidate) {
					skips = append(skips, projectSkip{path: candidate, reason: "skipped symlinked project config"})
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", skips
}

// loadMCP registers the global MCP file. Its path is fixed and known, so a
// missing file also registers as the writable creation target: the editor's
// first add creates it. A directory, or a file that cannot be read, stays a
// diagnostic without a source (the runtime cannot load it either). Unlike
// the project layer, symlinks are allowed here: the agent directory is a
// fixed, user-owned location that dotfile setups commonly link.
func (r *Registry) loadMCP(cfg RegistryConfig) {
	path := filepath.Join(cfg.AgentDir, mcpConfigName)
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			r.skip(Diagnostic{SourceID: IDMCP, Path: path, Reason: "mcp.json not found; the first add creates it"})
			r.add(Source{
				ID:       IDMCP,
				Path:     path,
				Kind:     KindExternal,
				Schema:   SchemaMCPServers,
				Writable: true,
			})
			return
		}
		r.skip(Diagnostic{SourceID: IDMCP, Path: path, Reason: err.Error()})
		return
	}
	if !info.Mode().IsRegular() {
		r.skip(Diagnostic{SourceID: IDMCP, Path: path, Reason: "not a regular file"})
		return
	}
	if _, err := os.ReadFile(path); err != nil {
		r.skip(Diagnostic{SourceID: IDMCP, Path: path, Reason: err.Error()})
		return
	}
	r.add(Source{
		ID:       IDMCP,
		Path:     path,
		Kind:     KindExternal,
		Schema:   SchemaMCPServers,
		Writable: true,
	})
	r.remember(IDMCP, path)
}

// exists reports whether path exists, without following symlinks.
func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// isSymlink reports whether path itself is a symlink. Missing paths are not
// symlinks; they are simply absent.
func isSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

// realOrSelf resolves symlinks in path, falling back to path itself when the
// path does not exist or cannot be resolved.
func realOrSelf(path string) string {
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return real
}
