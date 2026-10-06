// Package mcpfile edits the strict-JSON MCP configuration file at
// <agentDir>/mcp.json (~/.omo/agent/mcp.json by default) through the
// comment-preserving internal/omodit engine.
//
// The omo runtime reads this file with strict JSON.parse semantics, so the
// package treats it as strict JSON: comments and trailing commas are
// rejected on load and every save is validated before it touches the disk.
// Edits mirror the runtime's structural rules (type inference, endpoint
// requirements, ${VAR} interpolation limits) while unknown fields on
// existing servers are preserved untouched.
package mcpfile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/itokun99/lazyomo/internal/omodit"
)

// bootstrapContent materializes an empty document for a session loaded from
// a missing file. It is removed again before the first save so that omodit
// creates the real file without a backup.
const bootstrapContent = "{}\n"

// Session is an in-memory mcp.json editing session backed by an omodit
// document. A session loaded from a missing file starts empty; the first
// AddServer creates the file.
type Session struct {
	path     string
	doc      *omodit.Document
	skeleton string
	dirty    map[string]struct{}
}

// ResolveAgentDir returns the agent directory holding mcp.json, preferring
// OMO_CODING_AGENT_DIR, then SENPI_CODING_AGENT_DIR, then
// PI_CODING_AGENT_DIR, and falling back to ~/.omo/agent.
func ResolveAgentDir() (string, error) {
	for _, key := range []string{"OMO_CODING_AGENT_DIR", "SENPI_CODING_AGENT_DIR", "PI_CODING_AGENT_DIR"} {
		if dir := os.Getenv(key); dir != "" {
			return dir, nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving agent dir: %w", err)
	}
	return filepath.Join(home, ".omo", "agent"), nil
}

// Load reads the mcp.json at path. A missing file yields an empty session
// whose first AddServer creates the file. A file carrying comments or
// trailing commas is rejected, matching the runtime's strict JSON parser.
func Load(path string) (*Session, error) {
	if path == "" {
		return nil, fmt.Errorf("loading mcp.json: path is empty")
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Session{path: path}, nil
		}
		return nil, fmt.Errorf("loading mcp.json: %w", err)
	}
	doc, err := loadStrict(path)
	if err != nil {
		return nil, err
	}
	session := &Session{path: path, doc: doc}
	if err := session.validateShape(); err != nil {
		return nil, err
	}
	return session, nil
}

// loadStrict reads an existing mcp.json and rejects anything the runtime's
// JSON.parse would refuse.
func loadStrict(path string) (*omodit.Document, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading mcp.json: %w", err)
	}
	var probe any
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, fmt.Errorf("loading mcp.json: file must be strict JSON (comments and trailing commas are not allowed): %w", err)
	}
	doc, err := omodit.Load(path)
	if err != nil {
		return nil, fmt.Errorf("loading mcp.json: %w", err)
	}
	return doc, nil
}

// validateShape rejects documents the schema cannot address: the root,
// mcpServers, settings, and every server entry must be objects.
func (s *Session) validateShape() error {
	root, _ := s.doc.Get("")
	if _, ok := root.(map[string]any); !ok {
		return fmt.Errorf("loading mcp.json: document must be an object")
	}
	if servers, found := s.doc.Get("/mcpServers"); found {
		entries, ok := servers.(map[string]any)
		if !ok {
			return fmt.Errorf("loading mcp.json: mcpServers must be an object")
		}
		names := make([]string, 0, len(entries))
		for name := range entries {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if _, ok := entries[name].(map[string]any); !ok {
				return fmt.Errorf("loading mcp.json: mcpServers.%s must be an object", name)
			}
		}
	}
	if settings, found := s.doc.Get("/settings"); found {
		if _, ok := settings.(map[string]any); !ok {
			return fmt.Errorf("loading mcp.json: settings must be an object")
		}
	}
	return nil
}

// Path returns the mcp.json path the session edits.
func (s *Session) Path() string { return s.path }

// Bytes returns the in-memory document bytes.
func (s *Session) Bytes() []byte {
	if s.doc == nil {
		return nil
	}
	return s.doc.Bytes()
}

// Server is the read view of one MCP server entry. Fields carries every
// decoded member, including members this package does not know.
type Server struct {
	Name    string
	Type    string
	Enabled bool
	Fields  map[string]any
}

// Preview returns the command (stdio) or url (http) shown in lists.
func (sv Server) Preview() string {
	field := "command"
	if sv.Type == "http" {
		field = "url"
	}
	value, _ := sv.Fields[field].(string)
	return value
}

// Servers returns every server sorted by name.
func (s *Session) Servers() []Server {
	if s.doc == nil {
		return nil
	}
	serversValue, found := s.doc.Get("/mcpServers")
	if !found {
		return nil
	}
	entries, ok := serversValue.(map[string]any)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	servers := make([]Server, 0, len(names))
	for _, name := range names {
		raw, ok := entries[name].(map[string]any)
		if !ok {
			continue
		}
		servers = append(servers, newServer(name, raw))
	}
	return servers
}

// Server returns one server by name.
func (s *Session) Server(name string) (Server, bool) {
	raw, ok := s.rawServer(name)
	if !ok {
		return Server{}, false
	}
	return newServer(name, raw), true
}

// Settings returns the read-only settings object, or nil when absent.
// No editing operation touches it; edits elsewhere leave it untouched.
func (s *Session) Settings() map[string]any {
	if s.doc == nil {
		return nil
	}
	value, found := s.doc.Get("/settings")
	if !found {
		return nil
	}
	settings, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return settings
}

// Save writes the document through omodit (atomic, with a timestamped
// backup for an existing file) and returns the path of the backup it
// created, or "" when the file did not exist before. Dirty state is
// cleared only on success. A session loaded from a missing file that was
// never edited saves nothing; the first save after AddServer creates the
// file without a backup.
func (s *Session) Save() (string, error) {
	if s.doc == nil {
		return "", nil
	}
	if err := s.dropSkeleton(); err != nil {
		return "", err
	}
	data := s.doc.Bytes()
	if !json.Valid(data) {
		return "", fmt.Errorf("saving mcp.json: refusing to write non-strict JSON (comments or trailing commas present)")
	}
	before := backupNames(s.path)
	if err := s.doc.Save(); err != nil {
		return "", fmt.Errorf("saving mcp.json: %w", err)
	}
	s.dirty = nil
	return newBackupPath(before, backupNames(s.path)), nil
}

// Reload re-reads the file from disk, discarding unsaved edits. A file that
// disappeared yields the same empty session as Load on a missing path; an
// unreadable or invalid file leaves the current state untouched.
func (s *Session) Reload() error {
	fresh, err := Load(s.path)
	if err != nil {
		return err
	}
	s.doc = fresh.doc
	s.skeleton = fresh.skeleton
	s.dirty = nil
	return nil
}

// DirtyPaths returns the deduplicated pointers changed since the last Save
// or Reload, in sorted order.
func (s *Session) DirtyPaths() []string {
	paths := make([]string, 0, len(s.dirty))
	for pointer := range s.dirty {
		paths = append(paths, pointer)
	}
	sort.Strings(paths)
	return paths
}

// markDirty records one changed pointer.
func (s *Session) markDirty(pointer string) {
	if s.dirty == nil {
		s.dirty = make(map[string]struct{})
	}
	s.dirty[pointer] = struct{}{}
}

// backupNames lists the timestamped backups omodit created so far for path.
func backupNames(path string) map[string]struct{} {
	dir := filepath.Dir(path)
	prefix := filepath.Base(path) + ".bak."
	names := make(map[string]struct{})
	entries, err := os.ReadDir(dir)
	if err != nil {
		return names
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
			names[filepath.Join(dir, entry.Name())] = struct{}{}
		}
	}
	return names
}

// newBackupPath returns the newest backup present after a save but not
// before it, or "" when the save created none.
func newBackupPath(before, after map[string]struct{}) string {
	newest := ""
	for name := range after {
		if _, existed := before[name]; existed {
			continue
		}
		if name > newest {
			newest = name
		}
	}
	return newest
}

// ensureDoc materializes the document for a session loaded from a missing
// file. When another writer created the file in the meantime the existing
// file is adopted instead of being clobbered.
func (s *Session) ensureDoc() error {
	if s.doc != nil {
		return nil
	}
	if _, err := os.Stat(s.path); err == nil {
		doc, err := loadStrict(s.path)
		if err != nil {
			return err
		}
		s.doc = doc
		return s.validateShape()
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("loading mcp.json: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("creating mcp.json directory: %w", err)
	}
	if err := os.WriteFile(s.path, []byte(bootstrapContent), 0o644); err != nil {
		return fmt.Errorf("creating mcp.json: %w", err)
	}
	doc, err := omodit.Load(s.path)
	if err != nil {
		return fmt.Errorf("loading mcp.json: %w", err)
	}
	s.doc = doc
	s.skeleton = bootstrapContent
	return nil
}

// dropSkeleton removes the bootstrap skeleton before the first save, so
// omodit's create path (no backup for a file that did not exist) runs.
// A skeleton the writer no longer owns is kept for omodit to back up.
func (s *Session) dropSkeleton() error {
	if s.skeleton == "" {
		return nil
	}
	current, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			s.skeleton = ""
			return nil
		}
		return fmt.Errorf("saving mcp.json: %w", err)
	}
	if !bytes.Equal(current, []byte(s.skeleton)) {
		s.skeleton = ""
		return nil
	}
	if err := os.Remove(s.path); err != nil {
		return fmt.Errorf("saving mcp.json: %w", err)
	}
	s.skeleton = ""
	return nil
}

// rawServer decodes one server entry.
func (s *Session) rawServer(name string) (map[string]any, bool) {
	if s.doc == nil {
		return nil, false
	}
	value, found := s.doc.Get(serverPointer(name))
	if !found {
		return nil, false
	}
	raw, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	return raw, true
}

// newServer builds the read view of a server entry.
func newServer(name string, raw map[string]any) Server {
	return Server{
		Name:    name,
		Type:    effectiveType(raw),
		Enabled: effectiveEnabled(raw),
		Fields:  raw,
	}
}

// effectiveType mirrors the runtime inference: an explicit type wins,
// otherwise a non-empty url means http and anything else stdio.
func effectiveType(raw map[string]any) string {
	if typ, ok := raw["type"].(string); ok && typ != "" {
		return typ
	}
	if url, ok := raw["url"].(string); ok && url != "" {
		return "http"
	}
	return "stdio"
}

// effectiveEnabled mirrors the runtime default: an absent enabled is true.
func effectiveEnabled(raw map[string]any) bool {
	if enabled, ok := raw["enabled"].(bool); ok {
		return enabled
	}
	return true
}

// serverPointer escapes a server name (arbitrary string) into its RFC 6901
// JSON pointer token.
func serverPointer(name string) string {
	return "/mcpServers/" + strings.ReplaceAll(strings.ReplaceAll(name, "~", "~0"), "/", "~1")
}
