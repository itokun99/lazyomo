package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const toolsCacheFixture = `{
  "servers": {
    "acme": {
      "configHash": "hash-acme",
      "fetchedAt": 1789027833963,
      "tools": [
        {"name": "create-issue", "description": "Create an issue", "inputSchema": {"type": "object"}},
        {"name": "list-issues", "description": "List issues"}
      ]
    },
    "beta": {
      "configHash": "hash-beta",
      "fetchedAt": 1789027900000,
      "tools": [
        {"name": "search", "description": "Search documents"}
      ]
    }
  }
}`

func toolsCachePath(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "mcp-cache.json")
	writeFixture(t, path, toolsCacheFixture)
	return path
}

func newToolsModel(t *testing.T, toolsPath string) *Model {
	t.Helper()
	m := newTestModel(newFakeWorkspace(newFakeEditor()))
	m.setDerivedPaths("", "", toolsPath)
	m.refresh()
	return m
}

func TestToolsRendersCounts(t *testing.T) {
	dir := t.TempDir()
	toolsPath := toolsCachePath(t, dir)
	m := newToolsModel(t, toolsPath)
	m = selectSideRow(t, m, "Tools")
	if got := len(m.toolsRows); got != 2 {
		t.Fatalf("tools rows = %d, want 2", got)
	}
	if got := m.activePath(); got != toolsPath {
		t.Errorf("activePath() = %q, want %q", got, toolsPath)
	}
	if m.tools.Path != toolsPath {
		t.Errorf("tools.Path = %q, want %q", m.tools.Path, toolsPath)
	}
	if m.tools.TotalTools != 3 {
		t.Errorf("tools.TotalTools = %d, want 3", m.tools.TotalTools)
	}
	view := m.View()
	for _, want := range []string{"acme", "beta", "2 tools", "1 tool", "Tools · derived", "refreshed"} {
		if !strings.Contains(view, want) {
			t.Errorf("tools view missing %q:\n%s", want, view)
		}
	}
	m = sendKeys(t, m, "enter")
	view = m.View()
	for _, want := range []string{"create-issue", "list-issues", "Create an issue", "List issues", "fetchedAt", "1789027833963"} {
		if !strings.Contains(view, want) {
			t.Errorf("tools detail missing %q:\n%s", want, view)
		}
	}
	m = sendKeys(t, m, "esc", "j", "enter")
	view = m.View()
	for _, want := range []string{"search", "Search documents", "1789027900000"} {
		if !strings.Contains(view, want) {
			t.Errorf("beta tools detail missing %q:\n%s", want, view)
		}
	}
}

func TestToolsLazyLoadOnOpen(t *testing.T) {
	dir := t.TempDir()
	toolsPath := toolsCachePath(t, dir)
	m := newToolsModel(t, toolsPath)
	if m.tools.Path != "" {
		t.Fatalf("tools loaded at startup: Path = %q, want empty until the pane opens", m.tools.Path)
	}
	if len(m.toolsRows) != 0 {
		t.Fatalf("tools rows = %d at startup, want 0 (lazy load)", len(m.toolsRows))
	}
	m = selectSideRow(t, m, "Tools")
	if m.tools.Path != toolsPath {
		t.Fatalf("after open tools.Path = %q, want %q", m.tools.Path, toolsPath)
	}
	if len(m.toolsRows) != 2 {
		t.Fatalf("after open tools rows = %d, want 2", len(m.toolsRows))
	}
}

func TestToolsReadOnlyRejected(t *testing.T) {
	dir := t.TempDir()
	toolsPath := toolsCachePath(t, dir)
	m := newToolsModel(t, toolsPath)
	m = selectSideRow(t, m, "Tools")
	for _, key := range []string{"e", "a", "d", " "} {
		m = sendKeys(t, m, key)
		if m.status != "read-only section" {
			t.Errorf("tools key %q status = %q, want read-only section", key, m.status)
		}
		if m.overlay != OverlayNone {
			t.Errorf("tools key %q opened overlay %v", key, m.overlay)
		}
	}
	if got := m.dirtyCount(); got != 0 {
		t.Errorf("tools dirty = %d, want 0", got)
	}
}

func TestToolsMissingEmptyState(t *testing.T) {
	dir := t.TempDir()
	m := newToolsModel(t, filepath.Join(dir, "mcp-cache.json"))
	m = selectSideRow(t, m, "Tools")
	if view := m.View(); !strings.Contains(view, "unavailable") {
		t.Errorf("tools missing-file view must say unavailable:\n%s", view)
	}
	for _, key := range []string{"e", "a", "d", " "} {
		m = sendKeys(t, m, key)
		if m.overlay != OverlayNone {
			t.Errorf("missing-file key %q opened overlay %v", key, m.overlay)
		}
	}
}

func TestToolsMalformedNoCrash(t *testing.T) {
	dir := t.TempDir()
	toolsPath := filepath.Join(dir, "mcp-cache.json")
	writeFixture(t, toolsPath, `{"servers": {"acme": {"tools": `)
	m := newToolsModel(t, toolsPath)
	m = selectSideRow(t, m, "Tools")
	if view := m.View(); !strings.Contains(view, "unavailable") {
		t.Errorf("malformed tools view must say unavailable:\n%s", view)
	}
	for _, key := range []string{"e", "a", "d", " "} {
		m = sendKeys(t, m, key)
		if m.overlay != OverlayNone {
			t.Errorf("malformed key %q opened overlay %v", key, m.overlay)
		}
	}
}

func TestToolsRereadReflectsChange(t *testing.T) {
	dir := t.TempDir()
	toolsPath := toolsCachePath(t, dir)
	m := newToolsModel(t, toolsPath)
	m = selectSideRow(t, m, "Tools")
	if len(m.toolsRows) != 2 {
		t.Fatalf("first tools rows = %d, want 2", len(m.toolsRows))
	}
	changed := `{"servers": {"solo": {"fetchedAt": 1789027833963, "tools": [{"name": "only", "description": "Only tool"}]}}}`
	if err := os.WriteFile(toolsPath, []byte(changed), 0o644); err != nil {
		t.Fatalf("rewrite mcp-cache.json: %v", err)
	}
	m.refresh()
	if len(m.toolsRows) != 1 {
		t.Fatalf("rebuilt tools rows = %d, want 1", len(m.toolsRows))
	}
	if !strings.Contains(m.View(), "solo") {
		t.Errorf("rebuilt tools view missing solo:\n%s", m.View())
	}
	m = sendKeys(t, m, "enter")
	if !strings.Contains(m.View(), "only") {
		t.Errorf("rebuilt tools detail missing only:\n%s", m.View())
	}
}

func TestToolsKeybarConsistent(t *testing.T) {
	dir := t.TempDir()
	toolsPath := toolsCachePath(t, dir)
	m := newToolsModel(t, toolsPath)
	m = selectSideRow(t, m, "Tools")
	view := m.View()
	for _, want := range []string{"e edit", "a add", "d del", "space toggle"} {
		if !strings.Contains(view, want) {
			t.Errorf("tools keybar missing %q:\n%s", want, view)
		}
	}
}
