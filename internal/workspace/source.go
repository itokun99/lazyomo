// Package workspace owns the multi-source view of lazyomo's configuration
// surfaces: the user's omo.jsonc (editable through internal/editor), the
// nearest project-layer .omo/omo.jsonc (registered read-only for a later
// display wave), and the global MCP file <agentDir>/mcp.json (editable
// through the internal/mcpfile session wired in a later wave).
//
// A Registry is built once from caller-supplied roots. Every layer that
// fails to register becomes a diagnostic instead of an error, so one broken
// layer never takes the whole surface down. Rebuild the registry to observe
// files that appeared, changed, or disappeared on disk.
package workspace

import "github.com/itokun99/lazyomo/internal/editor"

// Kind classifies where a configuration source lives.
type Kind string

// Source kinds. The user layer is the operator's own omo.jsonc; project is
// the nearest .omo/omo.jsonc found from the working directory up to the home
// directory; external covers agent-directory files (mcp.json).
const (
	KindUser     Kind = "user"
	KindProject  Kind = "project"
	KindExternal Kind = "external"
)

// Schema names the document schema a source carries.
type Schema string

// Source schemas: omo.jsonc layers and the MCP servers file.
const (
	SchemaOmo        Schema = "omo"
	SchemaMCPServers Schema = "mcpservers"
)

// Source IDs. A registry registers at most one source per ID, in the order
// user, project, mcp.
const (
	IDUser    = "user"
	IDProject = "project"
	IDMCP     = "mcp"
)

// Session is the minimal uniform handle the registry keeps for an editable
// source's document: where it lives and what changed since the last save or
// reload. *editor.Editor satisfies it today; the mcp.json source gets its
// session (internal/mcpfile) in a later wave, so its Session stays nil until
// then. Save is deliberately absent: per-file save orchestration lands in a
// later todo and owns that signature.
type Session interface {
	Path() string
	DirtyPaths() []string
}

var _ Session = (*editor.Editor)(nil)

// Source is one configuration document the registry knows about.
type Source struct {
	ID       string
	Path     string
	Kind     Kind
	Schema   Schema
	Writable bool
	Session  Session
}

// Diagnostic reports a layer that could not be registered; the remaining
// sources still load.
type Diagnostic struct {
	SourceID string
	Path     string
	Reason   string
}
