package tui

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/itokun99/lazyomo/internal/mcpfile"
	"github.com/itokun99/lazyomo/internal/workspace"
)

// mcpSurfaceFixture writes a synthetic mcp.json (two servers: a stdio one
// with an env secret and an http one with a header secret and an oauth
// badge) and wires the REAL mcpfile session into the fakeWorkspace harness.
func mcpSurfaceFixture(t *testing.T) (*fakeWorkspace, *mcpfile.Session, string) {
	t.Helper()
	mcpPath := filepath.Join(t.TempDir(), "agent", "mcp.json")
	writeFixture(t, mcpPath, `{
  "settings": {"toolPrefix": "mcp"},
  "mcpServers": {
    "fetch": {"type": "stdio", "command": "npx", "args": ["-y", "fetch-mcp"], "env": {"FETCH_TOKEN": "sekret-fetch-token"}, "enabled": true},
    "notion": {"type": "http", "url": "https://notion.example/mcp", "headers": {"Authorization": "Bearer sekret-notion-header"}, "auth": "oauth", "enabled": true}
  }
}
`)
	session, err := mcpfile.Load(mcpPath)
	if err != nil {
		t.Fatalf("mcpfile.Load(%s) error = %v", mcpPath, err)
	}
	ws := newFakeWorkspace(newFakeEditor())
	ws.addSource(workspace.IDMCP, mcpPath, workspace.KindExternal, workspace.SchemaMCPServers, session)
	return ws, session, mcpPath
}

// selectSideRow puts the side cursor on a labelled row and lands focus on
// the entries pane, mirroring a real jump.
func selectSideRow(t *testing.T, m *Model, label string) *Model {
	t.Helper()
	for i, row := range m.flat {
		if row.label == label {
			m.secIdx = i
			m.entryIdx = 0
			m.focus = PaneEntries
			m.refresh()
			return m
		}
	}
	t.Fatalf("side row %q not found", label)
	return m
}

// detailIndex finds a rendered detail line by label, or -1.
func detailIndex(m *Model, label string) int {
	for i, line := range m.mcpDetail {
		if line.Label == label {
			return i
		}
	}
	return -1
}

func TestMCPServersListRendersRows(t *testing.T) {
	ws, _, mcpPath := mcpSurfaceFixture(t)
	m := newTestModel(ws)
	m = selectSideRow(t, m, "Servers")
	view := m.View()
	for _, want := range []string{"MCP servers ·", "fetch", "notion", "stdio", "http", "enabled", "⚿oauth"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
	if got := len(m.mcpRows); got != 2 {
		t.Fatalf("mcp rows = %d, want 2", got)
	}
	if len(m.entries) != 0 {
		t.Errorf("editor entries = %d, want 0 in MCP mode", len(m.entries))
	}
	if m.mcpPath != mcpPath {
		t.Errorf("mcpPath = %q, want %q", m.mcpPath, mcpPath)
	}
	if got := m.activePath(); got != mcpPath {
		t.Errorf("activePath() = %q, want the mcp path", got)
	}
}

func TestMCPSecretsMaskedInSettledStates(t *testing.T) {
	ws, _, _ := mcpSurfaceFixture(t)
	fetchSecret := "sekret-fetch-token"
	notionSecret := "sekret-notion-header"
	m := newTestModel(ws)
	m = selectSideRow(t, m, "Servers")
	m = sendKeys(t, m, "enter")
	if m.focus != PaneDetail {
		t.Fatalf("focus = %v, want detail", m.focus)
	}
	view := m.View()
	if !strings.Contains(view, "•••") {
		t.Errorf("settled detail must show the masked marker:\n%s", view)
	}
	for _, raw := range []string{fetchSecret, notionSecret} {
		if strings.Contains(view, raw) {
			t.Errorf("settled view leaks %q:\n%s", raw, view)
		}
	}
	// e on the env row pre-fills raw values in the edit overlay only.
	envIdx := detailIndex(m, "env")
	if envIdx < 0 {
		t.Fatal("env line missing from detail")
	}
	m.detailLn = envIdx
	m = sendKeys(t, m, "e")
	if m.overlay != OverlayEdit {
		t.Fatalf("overlay = %v, want edit", m.overlay)
	}
	if !strings.Contains(m.editBuf, fetchSecret) {
		t.Errorf("edit buffer must hold the raw value, got %q", m.editBuf)
	}
	if !strings.Contains(m.View(), fetchSecret) {
		t.Error("edit overlay must render the raw value")
	}
	m = sendKeys(t, m, "esc")
	if strings.Contains(m.View(), fetchSecret) {
		t.Error("raw leaked back into the settled view")
	}
	// x opens the reveal overlay: the explicitly entered surface where the
	// selected server's raw values are allowed.
	m = sendKeys(t, m, "x")
	if m.overlay != OverlayMCPReveal {
		t.Fatalf("overlay = %v, want reveal", m.overlay)
	}
	reveal := m.View()
	if !strings.Contains(reveal, fetchSecret) {
		t.Errorf("reveal overlay missing raw %q:\n%s", fetchSecret, reveal)
	}
	if strings.Contains(reveal, notionSecret) {
		t.Errorf("reveal must stay scoped to the selected server:\n%s", reveal)
	}
	m = sendKeys(t, m, "x")
	if m.overlay != OverlayNone {
		t.Fatalf("overlay = %v, want closed", m.overlay)
	}
	view = m.View()
	for _, raw := range []string{fetchSecret, notionSecret} {
		if strings.Contains(view, raw) {
			t.Errorf("raw %q still visible after closing reveal:\n%s", raw, view)
		}
	}
	if !strings.Contains(view, "•••") {
		t.Errorf("masked marker missing after reveal:\n%s", view)
	}
	// The header secret reveals when its own server is selected.
	m.entryIdx = 1
	m.refresh()
	m = sendKeys(t, m, "x")
	if m.overlay != OverlayMCPReveal || !strings.Contains(m.View(), notionSecret) {
		t.Fatalf("overlay = %v, want notion's header secret revealed:\n%s", m.overlay, m.View())
	}
	m = sendKeys(t, m, "esc")
	if !strings.Contains(m.View(), "•••") {
		t.Error("masked marker missing after closing the second reveal")
	}
}

func TestMCPAddEditToggleSaveReloadFlow(t *testing.T) {
	ws, session, mcpPath := mcpSurfaceFixture(t)
	m := newTestModel(ws)
	m = selectSideRow(t, m, "Servers")
	m = sendKeys(t, m, "a")
	if m.overlay != OverlayMCPAdd || m.mcpAddStage != 0 {
		t.Fatalf("overlay = %v stage = %d, want add stage 0", m.overlay, m.mcpAddStage)
	}
	m = sendKeys(t, m, "m", "y", "s", "enter")
	if m.mcpAddStage != 1 {
		t.Fatalf("stage = %d, want template", m.mcpAddStage)
	}
	m = sendKeys(t, m, "enter")
	if m.mcpAddStage != 2 {
		t.Fatalf("stage = %d, want confirm", m.mcpAddStage)
	}
	m = sendKeys(t, m, "enter")
	if m.overlay != OverlayNone {
		t.Fatalf("overlay = %v, want closed after create", m.overlay)
	}
	if !strings.Contains(m.status, "added mys") {
		t.Errorf("status = %q", m.status)
	}
	if m.mcpSelectedName() != "mys" {
		t.Fatalf("selected %q, want the new server", m.mcpSelectedName())
	}
	// e on the list edits the stdio endpoint field (command).
	m = sendKeys(t, m, "e")
	if m.overlay != OverlayEdit || m.editField != "command" || m.editServer != "mys" {
		t.Fatalf("edit = %v %q %q, want mys/command", m.overlay, m.editServer, m.editField)
	}
	m = sendKeys(t, m, "n", "p", "x", "enter")
	if m.overlay != OverlayNone {
		t.Fatalf("overlay = %v, want closed after edit", m.overlay)
	}
	if !strings.Contains(m.status, "edited mcpServers/mys/command") {
		t.Errorf("status = %q", m.status)
	}
	// space enables the server (its required command now exists).
	m = sendKeys(t, m, " ")
	if !strings.Contains(m.status, "enabled mys") {
		t.Errorf("status = %q", m.status)
	}
	if got := m.dirtyCount(); got != 3 {
		t.Errorf("dirty count = %d, want 3", got)
	}
	// save writes the real file with a timestamped backup.
	m = sendKeys(t, m, "s")
	if m.overlay != OverlayConfirm {
		t.Fatalf("overlay = %v, want save confirm", m.overlay)
	}
	m = sendKeys(t, m, "enter")
	if m.overlay != OverlayNone {
		t.Fatalf("overlay = %v, errorText = %q", m.overlay, m.errorText)
	}
	if !strings.Contains(m.status, "saved + backup") {
		t.Errorf("status = %q", m.status)
	}
	if got := m.dirtyCount(); got != 0 {
		t.Errorf("dirty after save = %d, want 0", got)
	}
	raw, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("reading mcp.json: %v", err)
	}
	var doc struct {
		MCPServers map[string]struct {
			Type    string `json:"type"`
			Command string `json:"command"`
			Enabled bool   `json:"enabled"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("saved mcp.json is not JSON: %v\n%s", err, raw)
	}
	added := doc.MCPServers["mys"]
	if added.Type != "stdio" || added.Command != "npx" || !added.Enabled {
		t.Errorf("mys on disk = %+v, want stdio/npx/enabled", added)
	}
	backups, err := filepath.Glob(mcpPath + ".bak.*")
	if err != nil {
		t.Fatalf("globbing backups: %v", err)
	}
	if len(backups) != 1 {
		t.Errorf("backups = %v, want exactly 1", backups)
	}
	// r reloads from disk: an externally added server appears.
	writeFixture(t, mcpPath, `{"mcpServers":{"extra":{"type":"http","url":"https://extra.example/mcp"},"fetch":{"type":"stdio","command":"npx"},"mys":{"type":"stdio","command":"npx","enabled":true},"notion":{"type":"http","url":"https://notion.example/mcp"}}}`)
	m = sendKeys(t, m, "r")
	if m.overlay != OverlayNone {
		t.Fatalf("overlay = %v, want none after clean reload", m.overlay)
	}
	if !strings.Contains(m.View(), "extra") {
		t.Errorf("reload did not pick up the new server:\n%s", m.View())
	}
	if len(session.DirtyPaths()) != 0 {
		t.Errorf("dirty after reload = %v, want none", session.DirtyPaths())
	}
}

func TestMCPAbsentFileEmptyStateAndFirstAdd(t *testing.T) {
	mcpPath := filepath.Join(t.TempDir(), "agent", "mcp.json")
	session, err := mcpfile.Load(mcpPath)
	if err != nil {
		t.Fatalf("loading missing mcp.json: %v", err)
	}
	ws := newFakeWorkspace(newFakeEditor())
	ws.addSource(workspace.IDMCP, mcpPath, workspace.KindExternal, workspace.SchemaMCPServers, session)
	m := newTestModel(ws)
	m = selectSideRow(t, m, "Servers")
	if view := m.View(); !strings.Contains(view, "no servers") {
		t.Errorf("empty state missing:\n%s", view)
	}
	if _, err := os.Stat(mcpPath); !os.IsNotExist(err) {
		t.Fatalf("mcp.json must not exist before the first add: %v", err)
	}
	m = sendKeys(t, m, "a", "f", "i", "r", "s", "t", "enter", "enter", "enter")
	if m.overlay != OverlayNone {
		t.Fatalf("overlay = %v, want closed after create", m.overlay)
	}
	if !strings.Contains(m.status, "added first") {
		t.Errorf("status = %q", m.status)
	}
	if _, err := os.Stat(mcpPath); err != nil {
		t.Fatalf("first add must create the file: %v", err)
	}
	m = sendKeys(t, m, "s", "enter")
	if !strings.Contains(m.status, "saved") {
		t.Errorf("status = %q", m.status)
	}
	raw, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("reading created mcp.json: %v", err)
	}
	if !strings.Contains(string(raw), "first") {
		t.Errorf("created file missing the server:\n%s", raw)
	}
	backups, err := filepath.Glob(mcpPath + ".bak.*")
	if err != nil {
		t.Fatalf("globbing backups: %v", err)
	}
	if len(backups) != 0 {
		t.Errorf("first add must not leave a backup: %v", backups)
	}
}

func TestMCPUnavailableEmptyState(t *testing.T) {
	m := newTestModel(newFakeWorkspace(newFakeEditor()))
	m = selectSideRow(t, m, "Servers")
	if view := m.View(); !strings.Contains(view, "mcp.json unavailable") {
		t.Errorf("view missing the diagnostic:\n%s", view)
	}
	for _, key := range []string{"e", "a", "d", " "} {
		m = sendKeys(t, m, key)
		if m.status != "mcp.json unavailable" {
			t.Errorf("key %q status = %q, want the diagnostic", key, m.status)
		}
		if m.overlay != OverlayNone {
			t.Errorf("key %q opened overlay %v", key, m.overlay)
		}
	}
}

func TestMCPInlineValidationRefusesWrites(t *testing.T) {
	ws, session, mcpPath := mcpSurfaceFixture(t)
	before, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	m := newTestModel(ws)
	m = selectSideRow(t, m, "Servers")
	// duplicate name on add
	m = sendKeys(t, m, "a", "f", "e", "t", "c", "h", "enter")
	if m.overlay != OverlayMCPAdd || m.mcpAddStage != 0 || m.addErr == "" {
		t.Fatalf("overlay = %v stage = %d err = %q, want inline duplicate error", m.overlay, m.mcpAddStage, m.addErr)
	}
	m = sendKeys(t, m, "esc")
	// empty name on add
	m = sendKeys(t, m, "a", "enter")
	if m.addErr == "" {
		t.Fatal("expected inline empty-name error")
	}
	m = sendKeys(t, m, "esc")
	// changing the enabled stdio server to http trips the endpoint rule
	m = sendKeys(t, m, "enter")
	typeIdx := detailIndex(m, "type")
	if typeIdx < 0 {
		t.Fatal("type line missing")
	}
	m.detailLn = typeIdx
	m = sendKeys(t, m, "e")
	m.editBuf = "http"
	m = sendKeys(t, m, "enter")
	if m.overlay != OverlayEdit || !strings.Contains(m.editErr, "Required for enabled http server") {
		t.Fatalf("overlay = %v err = %q, want the endpoint rule error", m.overlay, m.editErr)
	}
	m = sendKeys(t, m, "esc")
	// blank command is refused before it reaches the session
	cmdIdx := detailIndex(m, "command")
	if cmdIdx < 0 {
		t.Fatal("command line missing")
	}
	m.detailLn = cmdIdx
	m = sendKeys(t, m, "e")
	m.editBuf = ""
	m = sendKeys(t, m, "enter")
	if m.editErr == "" {
		t.Fatal("blank value must be refused inline")
	}
	// interpolation rejection
	m.editBuf = "$(evil)"
	m = sendKeys(t, m, "enter")
	if m.editErr == "" || !strings.Contains(m.editErr, "rejected") {
		t.Fatalf("editErr = %q, want the interpolation rejection", m.editErr)
	}
	m = sendKeys(t, m, "esc")
	// nothing reached the disk or the session
	after, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("rereading fixture: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Error("validation failures must not write")
	}
	if len(session.DirtyPaths()) != 0 {
		t.Errorf("dirty = %v, want none after refused edits", session.DirtyPaths())
	}
}

func TestMCPSettingsReadOnlyAndRenameWarns(t *testing.T) {
	ws, session, mcpPath := mcpSurfaceFixture(t)
	m := newTestModel(ws)
	m = selectSideRow(t, m, "Servers")
	m = sendKeys(t, m, "enter")
	settingsIdx := detailIndex(m, "settings.toolPrefix")
	if settingsIdx < 0 {
		t.Fatalf("settings line missing: %+v", m.mcpDetail)
	}
	m.detailLn = settingsIdx
	m = sendKeys(t, m, "e")
	if m.overlay != OverlayNone || m.status != "read-only" {
		t.Fatalf("settings edit = overlay %v status %q, want read-only refusal", m.overlay, m.status)
	}
	// rename through the name row
	nameIdx := detailIndex(m, "name")
	if nameIdx < 0 {
		t.Fatal("name line missing")
	}
	m.detailLn = nameIdx
	m = sendKeys(t, m, "e")
	if m.overlay != OverlayEdit || !m.editRename {
		t.Fatalf("overlay = %v rename = %v", m.overlay, m.editRename)
	}
	m.editBuf = "fetch2"
	m = sendKeys(t, m, "enter")
	if m.overlay != OverlayConfirm {
		t.Fatalf("overlay = %v, want the orphan-auth confirm", m.overlay)
	}
	if !strings.Contains(m.confirmText, "orphan") {
		t.Errorf("confirm must warn about orphaned credentials: %q", m.confirmText)
	}
	m = sendKeys(t, m, "enter")
	if _, ok := session.Server("fetch2"); !ok {
		t.Error("rename did not apply")
	}
	if _, ok := session.Server("fetch"); ok {
		t.Error("old name still present")
	}
	if !strings.Contains(m.status, "orphaned") {
		t.Errorf("status = %q, want the orphan-auth warning", m.status)
	}
	m = sendKeys(t, m, "s", "enter")
	raw, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("reading mcp.json: %v", err)
	}
	if !strings.Contains(string(raw), "fetch2") || strings.Contains(string(raw), `"fetch"`) {
		t.Errorf("rename not persisted:\n%s", raw)
	}
}

func TestMCPStaleSaveConfirmBlocked(t *testing.T) {
	reg, session, _, mcpPath := realRegistryFixture(t)
	m := newTestModel(reg)
	m = selectSideRow(t, m, "Servers")
	m = sendKeys(t, m, " ")
	if got := m.dirtyCount(); got == 0 {
		t.Fatalf("expected a dirty mcp session (status %q)", m.status)
	}
	tampered := []byte(`{"mcpServers":{"fetch":{"type":"stdio","command":"tampered"}}}`)
	if err := os.WriteFile(mcpPath, tampered, 0o644); err != nil {
		t.Fatalf("tampering mcp.json: %v", err)
	}
	m = sendKeys(t, m, "s")
	if m.overlay != OverlayConfirm {
		t.Fatalf("overlay = %v, want the save confirm", m.overlay)
	}
	if !strings.Contains(m.confirmText, "stale, blocked") || !strings.Contains(m.confirmText, mcpPath) {
		t.Errorf("confirm must list the stale file as blocked:\n%s", m.confirmText)
	}
	m = sendKeys(t, m, "enter")
	if m.overlay != OverlayError || !strings.Contains(m.errorText, "stale") {
		t.Fatalf("overlay = %v errorText = %q, want the stale refusal", m.overlay, m.errorText)
	}
	after, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("reading tampered file: %v", err)
	}
	if !bytes.Equal(after, tampered) {
		t.Error("refused save touched the file")
	}
	if len(session.DirtyPaths()) == 0 {
		t.Error("stale refusal must keep the unsaved edits")
	}
}

func TestMCPRemoveConfirmDeletes(t *testing.T) {
	ws, session, mcpPath := mcpSurfaceFixture(t)
	m := newTestModel(ws)
	m = selectSideRow(t, m, "Servers")
	m.entryIdx = 1
	m.refresh()
	if got := m.mcpSelectedName(); got != "notion" {
		t.Fatalf("selected %q, want notion", got)
	}
	m = sendKeys(t, m, "d")
	if m.overlay != OverlayConfirm || !strings.Contains(m.confirmText, "notion") {
		t.Fatalf("overlay = %v confirm = %q", m.overlay, m.confirmText)
	}
	m = sendKeys(t, m, "esc")
	if _, ok := session.Server("notion"); !ok {
		t.Fatal("esc must keep the server")
	}
	m = sendKeys(t, m, "d", "enter")
	if _, ok := session.Server("notion"); ok {
		t.Fatal("server still present after confirm")
	}
	if len(m.mcpRows) != 1 || m.mcpRows[0].name != "fetch" {
		t.Fatalf("rows after remove = %+v, want only fetch", m.mcpRows)
	}
	if !strings.Contains(m.status, "removed notion") {
		t.Errorf("status = %q", m.status)
	}
	m = sendKeys(t, m, "s", "enter")
	raw, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("reading mcp.json: %v", err)
	}
	if strings.Contains(string(raw), "notion") {
		t.Errorf("removed server still on disk:\n%s", raw)
	}
}

func TestMCPDensity80x24(t *testing.T) {
	ws, _, _ := mcpSurfaceFixture(t)
	m := newTestModel(ws)
	m = selectSideRow(t, m, "Servers")
	m.width = 80
	m.height = 24
	m.refresh()
	for _, focus := range []Pane{PaneEntries, PaneDetail} {
		m.focus = focus
		lines := strings.Split(m.View(), "\n")
		if len(lines) != 24 {
			t.Fatalf("focus %v painted lines = %d, want 24", focus, len(lines))
		}
		for i, l := range lines {
			if w := lipgloss.Width(l); w > 80 {
				t.Errorf("focus %v line %d width = %d, want <= 80: %q", focus, i, w, l)
			}
		}
	}
	m.focus = PaneDetail
	m = sendKeys(t, m, "x")
	for i, l := range strings.Split(m.View(), "\n") {
		if w := lipgloss.Width(l); w > 80 {
			t.Errorf("reveal overlay line %d width = %d, want <= 80", i, w)
		}
	}
}
