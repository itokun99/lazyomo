package mcpfile

import (
	"fmt"
	"strings"
)

// SetField replaces one known field of an existing server, adding the
// member when absent. Unknown field names are rejected so new unknown
// members can never be created; unknown members already present in the
// file are preserved untouched.
func (s *Session) SetField(server, field string, value any) error {
	spec, ok := fieldSpec(field)
	if !ok {
		return fmt.Errorf("mcpServers.%s.%s: unknown field", server, field)
	}
	raw, ok := s.rawServer(server)
	if !ok {
		return fmt.Errorf("mcpServers.%s: server not found", server)
	}
	normalized, err := normalizeValue(spec, server, value)
	if err != nil {
		return err
	}
	prospective := cloneServer(raw)
	prospective[field] = normalized
	if err := endpointViolation(server, prospective); err != nil {
		return err
	}
	return s.writeField(server, field, normalized)
}

// ToggleEnabled flips the server's enabled flag (absent means enabled) and
// returns the new state. Enabling is refused with the runtime's endpoint
// error when the required command or url is missing.
func (s *Session) ToggleEnabled(server string) (bool, error) {
	raw, ok := s.rawServer(server)
	if !ok {
		return false, fmt.Errorf("mcpServers.%s: server not found", server)
	}
	next := !effectiveEnabled(raw)
	prospective := cloneServer(raw)
	prospective["enabled"] = next
	if err := endpointViolation(server, prospective); err != nil {
		return false, err
	}
	if err := s.writeField(server, "enabled", next); err != nil {
		return false, err
	}
	return next, nil
}

// AddServer creates a disabled server of the given type ("stdio" or
// "http"). The caller configures the command or url and then enables it, so
// the new entry satisfies the endpoint rule from the moment it exists. On a
// session loaded from a missing file this creates the file's document.
func (s *Session) AddServer(name, typ string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("adding server: name must not be empty")
	}
	if typ != "stdio" && typ != "http" {
		return fmt.Errorf("mcpServers.%s.type: want %q or %q, got %q", name, "stdio", "http", typ)
	}
	if err := s.ensureDoc(); err != nil {
		return err
	}
	if _, exists := s.rawServer(name); exists {
		return fmt.Errorf("mcpServers.%s: server already exists", name)
	}
	if _, found := s.doc.Get("/mcpServers"); !found {
		if err := s.doc.Add("/mcpServers", map[string]any{}); err != nil {
			return fmt.Errorf("adding server %q: %w", name, err)
		}
	}
	entry := map[string]any{"type": typ, "enabled": false}
	if err := s.doc.Add(serverPointer(name), entry); err != nil {
		return fmt.Errorf("adding server %q: %w", name, err)
	}
	s.markDirty(serverPointer(name))
	return nil
}

// RemoveServer deletes a server entry.
func (s *Session) RemoveServer(name string) error {
	if _, ok := s.rawServer(name); !ok {
		return fmt.Errorf("mcpServers.%s: server not found", name)
	}
	if err := s.doc.Remove(serverPointer(name)); err != nil {
		return fmt.Errorf("removing mcpServers.%s: %w", name, err)
	}
	s.markDirty(serverPointer(name))
	return nil
}

// RenameResult reports a rename and the credential warning metadata callers
// must surface: OAuth tokens live in mcp-auth/ keyed by the server's old
// identity, so a rename can orphan them.
type RenameResult struct {
	OldName    string
	NewName    string
	OrphanAuth bool
	Warning    string
}

// RenameServer moves a server entry to a new name, preserving every field.
// The result always carries the orphan-auth warning: mcp-auth/ is never
// read, so callers re-authenticate when the server uses OAuth.
func (s *Session) RenameServer(oldName, newName string) (RenameResult, error) {
	raw, ok := s.rawServer(oldName)
	if !ok {
		return RenameResult{}, fmt.Errorf("mcpServers.%s: server not found", oldName)
	}
	if strings.TrimSpace(newName) == "" {
		return RenameResult{}, fmt.Errorf("renaming server %q: new name must not be empty", oldName)
	}
	if _, exists := s.rawServer(newName); exists {
		return RenameResult{}, fmt.Errorf("mcpServers.%s: server already exists", newName)
	}
	if err := s.doc.Add(serverPointer(newName), raw); err != nil {
		return RenameResult{}, fmt.Errorf("renaming server %q: %w", oldName, err)
	}
	if err := s.doc.Remove(serverPointer(oldName)); err != nil {
		return RenameResult{}, fmt.Errorf("renaming server %q: %w", oldName, err)
	}
	s.markDirty(serverPointer(oldName))
	s.markDirty(serverPointer(newName))
	return RenameResult{
		OldName:    oldName,
		NewName:    newName,
		OrphanAuth: true,
		Warning:    fmt.Sprintf("stored credentials for %q may be orphaned (mcp-auth is keyed by the old identity); re-authenticate via senpi /mcp auth if needed", oldName),
	}, nil
}

// writeField stores one known field on a server entry, choosing replace or
// add by current presence.
func (s *Session) writeField(server, field string, value any) error {
	pointer := serverPointer(server) + "/" + field
	if _, found := s.doc.Get(pointer); found {
		if err := s.doc.Set(pointer, value); err != nil {
			return fmt.Errorf("setting mcpServers.%s.%s: %w", server, field, err)
		}
	} else if err := s.doc.Add(pointer, value); err != nil {
		return fmt.Errorf("setting mcpServers.%s.%s: %w", server, field, err)
	}
	s.markDirty(pointer)
	return nil
}
