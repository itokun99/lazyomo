package tui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/itokun99/lazyomo/internal/editor"
	"github.com/itokun99/lazyomo/internal/mcpfile"
	"github.com/itokun99/lazyomo/internal/workspace"
)

// MCPSurface is the consumer-side editing interface for the MCP document
// (<agentDir>/mcp.json). The real *mcpfile.Session is wired in cmd/lazyomo;
// tests drive the same interface through the fakeWorkspace harness over a
// real session.
type MCPSurface interface {
	Path() string
	DirtyPaths() []string
	Servers() []mcpfile.Server
	Server(name string) (mcpfile.Server, bool)
	Settings() map[string]any
	SetField(server, field string, value any) error
	ToggleEnabled(server string) (bool, error)
	AddServer(name, typ string) error
	RemoveServer(name string) error
	RenameServer(oldName, newName string) (mcpfile.RenameResult, error)
}

var _ MCPSurface = (*mcpfile.Session)(nil)

// mcpRow is one rendered Servers row: the list contract's columns
// (name/type/enabled/auth badge) plus the dirty marker.
type mcpRow struct {
	name    string
	typ     string
	enabled bool
	auth    string
	dirty   bool
}

// mcpDetailLine is one selectable row of the selected server's detail.
// Field names the server member an edit writes ("name" is the rename row);
// settings and unknown-field rows are read-only. Secret marks env/headers
// maps, whose values stay masked outside the reveal and edit overlays.
type mcpDetailLine struct {
	Label    string
	Value    string
	Field    string
	Kind     mcpfile.FieldKind
	Editable bool
	Secret   bool
}

// mcpAddTypes are the templates the add flow offers.
var mcpAddTypes = []string{"stdio", "http"}

// bindMCP picks the editable MCP session: the first registered source with
// the mcpservers schema whose session implements MCPSurface. A registered
// source without a session (unreadable or invalid file) keeps its path for
// the empty state but leaves the surface nil.
func (m *Model) bindMCP() {
	if m.ws == nil {
		return
	}
	m.mcp = nil
	m.mcpPath = ""
	for _, src := range m.ws.Sources() {
		if src.Schema != workspace.SchemaMCPServers {
			continue
		}
		m.mcpPath = src.Path
		if surface, ok := src.Session.(MCPSurface); ok {
			m.mcp = surface
		}
		return
	}
}

// sideMCP reports whether the selected side row is the MCP servers surface.
func (m *Model) sideMCP() bool {
	row := m.selSide()
	return row != nil && row.mcp
}

// listLen is the entries-pane cursor bound for the active surface.
func (m *Model) listLen() int {
	if m.sideMCP() {
		return len(m.mcpRows)
	}
	return len(m.entries)
}

// detailListLen is the selectable-line count of the detail pane.
func (m *Model) detailListLen() int {
	if m.sideMCP() {
		return len(m.mcpDetail)
	}
	return len(m.detail.Lines)
}

// detailListViewport is the selectable rows of the detail pane; in MCP mode
// the fixed source-path header consumes one painted row.
func (m *Model) detailListViewport() int {
	v := m.detailViewport()
	if m.sideMCP() {
		v--
	}
	if v < 1 {
		v = 1
	}
	return v
}

// refreshMCP rebuilds the MCP list rows and the selected server's detail
// from the bound session. An unbound session leaves both empty; the panes
// render the diagnostic empty state.
func (m *Model) refreshMCP() {
	m.entries = nil
	m.mcpRows = m.buildMCPRows()
	m.entryIdx = clamp(m.entryIdx, len(m.mcpRows))
	m.entryOffset = adjustOffset(m.entryOffset, m.entryIdx, m.entriesViewport())
	m.detail = editor.Detail{}
	m.mcpDetail = m.buildMCPDetail()
	m.detailLn = clamp(m.detailLn, len(m.mcpDetail))
	m.detailOff = adjustOffset(m.detailOff, m.detailLn, m.detailListViewport())
}

// buildMCPRows maps the session's servers onto list rows, filtered by the
// active name filter.
func (m *Model) buildMCPRows() []mcpRow {
	if m.mcp == nil {
		return nil
	}
	dirty := map[string]bool{}
	for _, pointer := range m.mcp.DirtyPaths() {
		if name, ok := dirtyServer(pointer); ok {
			dirty[name] = true
		}
	}
	var rows []mcpRow
	for _, server := range m.mcp.Servers() {
		if !containsFold(server.Name, m.filter) {
			continue
		}
		rows = append(rows, mcpRow{
			name:    server.Name,
			typ:     server.Type,
			enabled: server.Enabled,
			auth:    authBadge(server.Fields),
			dirty:   dirty[server.Name],
		})
	}
	return rows
}

// dirtyServer extracts the server name from a /mcpServers/<name> pointer.
func dirtyServer(pointer string) (string, bool) {
	const prefix = "/mcpServers/"
	if !strings.HasPrefix(pointer, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(pointer, prefix)
	name := rest
	if i := strings.Index(rest, "/"); i >= 0 {
		name = rest[:i]
	}
	if name == "" {
		return "", false
	}
	return unescapePointer(name), true
}

// unescapePointer decodes an RFC 6901 pointer token.
func unescapePointer(token string) string {
	return strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
}

// authBadge returns the list badge for the server's auth mode; auth is a
// mode string, never a secret, and false/absent means no badge.
func authBadge(fields map[string]any) string {
	if mode, ok := fields["auth"].(string); ok && mode != "" {
		return mode
	}
	return ""
}

// buildMCPDetail renders every known schema field of the selected server
// (present or not, so every field is reachable for editing), then unknown
// fields preserved in the file, then the read-only settings block.
// env/headers values are masked here; raw values exist only inside the
// reveal and edit overlays.
func (m *Model) buildMCPDetail() []mcpDetailLine {
	row := m.selMCPRow()
	if m.mcp == nil || row == nil {
		return nil
	}
	server, ok := m.mcp.Server(row.name)
	if !ok {
		return nil
	}
	lines := []mcpDetailLine{{Label: "name", Value: server.Name, Field: "name", Kind: mcpfile.FieldString, Editable: true}}
	known := map[string]bool{}
	for _, spec := range mcpfile.Fields() {
		known[spec.Name] = true
		value, present := server.Fields[spec.Name]
		line := mcpDetailLine{Label: spec.Name, Field: spec.Name, Kind: spec.Kind, Editable: true}
		switch {
		case spec.Kind == mcpfile.FieldStringMap && present:
			line.Secret = true
			line.Value = maskedMapLabel(value)
		case present:
			line.Value = formatMCPValue(value)
		}
		lines = append(lines, line)
	}
	unknown := make([]string, 0)
	for name := range server.Fields {
		if !known[name] {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)
	for _, name := range unknown {
		lines = append(lines, mcpDetailLine{Label: name, Value: formatMCPValue(server.Fields[name])})
	}
	if settings := m.mcp.Settings(); len(settings) > 0 {
		keys := make([]string, 0, len(settings))
		for key := range settings {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			lines = append(lines, mcpDetailLine{Label: "settings." + key, Value: formatMCPValue(settings[key])})
		}
	} else {
		lines = append(lines, mcpDetailLine{Label: "settings", Value: "(none)"})
	}
	return lines
}

// selMCPRow returns the selected list row.
func (m *Model) selMCPRow() *mcpRow {
	if len(m.mcpRows) == 0 || m.entryIdx < 0 || m.entryIdx >= len(m.mcpRows) {
		return nil
	}
	return &m.mcpRows[m.entryIdx]
}

// selMCPLine returns the selected detail line.
func (m *Model) selMCPLine() *mcpDetailLine {
	if len(m.mcpDetail) == 0 || m.detailLn < 0 || m.detailLn >= len(m.mcpDetail) {
		return nil
	}
	return &m.mcpDetail[m.detailLn]
}

// mcpSelectedName names the selected server, or "" when none is selected.
func (m *Model) mcpSelectedName() string {
	if row := m.selMCPRow(); row != nil {
		return row.name
	}
	return ""
}

// mcpOpenEdit routes `e`: on the list it edits the endpoint field (command
// for stdio, url for http); in the detail it edits the selected row, with
// the name row opening the rename flow.
func (m *Model) mcpOpenEdit() {
	if m.mcp == nil {
		m.status = "mcp.json unavailable"
		return
	}
	name := m.mcpSelectedName()
	if name == "" {
		m.status = "no server selected"
		return
	}
	if m.focus == PaneEntries {
		server, _ := m.mcp.Server(name)
		field := "command"
		if server.Type == "http" {
			field = "url"
		}
		m.mcpStartFieldEdit(name, field, mcpfile.FieldString)
		return
	}
	line := m.selMCPLine()
	if line == nil {
		return
	}
	if line.Field == "name" && line.Editable {
		m.mcpStartRename(name)
		return
	}
	if !line.Editable {
		m.status = "read-only"
		return
	}
	if line.Kind == mcpfile.FieldBool {
		m.status = "use space to toggle bools"
		return
	}
	m.mcpStartFieldEdit(name, line.Field, line.Kind)
}

// mcpStartFieldEdit opens the edit overlay pre-filled with the field's
// current JSON form. Secret maps pre-fill their raw values here by design:
// raw values exist only inside an explicitly entered overlay.
func (m *Model) mcpStartFieldEdit(name, field string, kind mcpfile.FieldKind) {
	buf := ""
	if server, ok := m.mcp.Server(name); ok {
		buf = mcpEditBuffer(server, field, kind)
	}
	m.overlay = OverlayEdit
	m.editMCP = true
	m.editRename = false
	m.editServer = name
	m.editField = field
	m.editKind = kind
	m.editTitle = "Edit mcpServers/" + name + "/" + field
	m.editBuf = buf
	m.editErr = ""
}

// mcpEditBuffer renders the overlay's pre-filled buffer for a field.
func mcpEditBuffer(server mcpfile.Server, field string, kind mcpfile.FieldKind) string {
	value, present := server.Fields[field]
	if !present {
		switch kind {
		case mcpfile.FieldStrings, mcpfile.FieldBoolOrStrings:
			return "[]"
		case mcpfile.FieldStringMap, mcpfile.FieldObject:
			return "{}"
		}
		return ""
	}
	switch kind {
	case mcpfile.FieldString:
		text, _ := value.(string)
		return text
	case mcpfile.FieldStringOrFalse:
		if b, ok := value.(bool); ok && !b {
			return "false"
		}
		text, _ := value.(string)
		return text
	}
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

// submitMCPField applies the MCP edit overlay's buffer. Parsing or session
// validation failures keep the overlay open with an inline error and never
// write.
func (m *Model) submitMCPField() {
	buf := strings.TrimSpace(m.editBuf)
	if buf == "" {
		m.editErr = "value must not be empty"
		return
	}
	value, err := parseMCPValue(m.editKind, buf)
	if err != nil {
		m.editErr = err.Error()
		return
	}
	if err := m.mcp.SetField(m.editServer, m.editField, value); err != nil {
		m.editErr = err.Error()
		return
	}
	name, field := m.editServer, m.editField
	m.closeOverlay()
	m.status = "edited mcpServers/" + name + "/" + field
	m.refresh()
}

// parseMCPValue converts the overlay buffer to the JSON value for the
// field's kind; typed errors surface inline.
func parseMCPValue(kind mcpfile.FieldKind, buf string) (any, error) {
	switch kind {
	case mcpfile.FieldString:
		return buf, nil
	case mcpfile.FieldStringOrFalse:
		if buf == "false" {
			return false, nil
		}
		return buf, nil
	}
	var value any
	if err := json.Unmarshal([]byte(buf), &value); err != nil {
		return nil, fmt.Errorf("value must be valid JSON: %w", err)
	}
	switch kind {
	case mcpfile.FieldNumber:
		if _, ok := value.(float64); !ok {
			return nil, fmt.Errorf("value must be a number")
		}
	case mcpfile.FieldBool:
		if _, ok := value.(bool); !ok {
			return nil, fmt.Errorf("value must be true or false")
		}
	}
	return value, nil
}

// mcpStartRename opens the rename edit over the selected server's name.
func (m *Model) mcpStartRename(name string) {
	m.overlay = OverlayEdit
	m.editMCP = true
	m.editRename = true
	m.editServer = name
	m.editField = "name"
	m.editKind = mcpfile.FieldString
	m.editTitle = "Rename server " + name
	m.editBuf = name
	m.editErr = ""
}

// submitRename validates the new name and raises the orphan-auth warning
// confirm; the rename itself runs on confirm.
func (m *Model) submitRename() {
	newName := strings.TrimSpace(m.editBuf)
	if newName == "" {
		m.editErr = "name must not be empty"
		return
	}
	if newName == m.editServer {
		m.closeOverlay()
		m.status = "unchanged"
		return
	}
	if _, exists := m.mcp.Server(newName); exists {
		m.editErr = "server already exists"
		return
	}
	m.overlay = OverlayConfirm
	m.confirmKind = confirmRename
	m.renameFrom = m.editServer
	m.renameTo = newName
	m.confirmText = "Rename " + m.editServer + " to " + newName + "?\nCredentials in mcp-auth are keyed by the old name; OAuth tokens may be orphaned. Re-authenticate via senpi /mcp auth if needed."
}

// mcpToggle flips the selected server's enabled flag; the session refuses
// enabling a server without its required command or url.
func (m *Model) mcpToggle() {
	if m.mcp == nil {
		m.status = "mcp.json unavailable"
		return
	}
	name := m.mcpSelectedName()
	if name == "" {
		m.status = "no server selected"
		return
	}
	enabled, err := m.mcp.ToggleEnabled(name)
	if err != nil {
		m.status = err.Error()
		return
	}
	if enabled {
		m.status = "enabled " + name
	} else {
		m.status = "disabled " + name
	}
	m.refresh()
}

// mcpOpenDelete raises the remove confirm for the selected server.
func (m *Model) mcpOpenDelete() {
	if m.mcp == nil {
		m.status = "mcp.json unavailable"
		return
	}
	name := m.mcpSelectedName()
	if name == "" {
		m.status = "no server selected"
		return
	}
	m.overlay = OverlayConfirm
	m.confirmKind = confirmMCPRemove
	m.pendingMCP = name
	m.confirmText = "Remove server " + name + "?\nThis edits mcp.json only; mcp-auth is never touched."
}

// mcpOpenAdd starts the add flow: name, then a stdio|http template, then
// confirm.
func (m *Model) mcpOpenAdd() {
	if m.mcp == nil {
		m.status = "mcp.json unavailable"
		return
	}
	m.overlay = OverlayMCPAdd
	m.mcpAddStage = 0
	m.mcpAddTypeIdx = 0
	m.mcpAddType = ""
	m.addBuf = ""
	m.addErr = ""
}

// mcpAddSubmit advances the add flow one stage; the final stage creates a
// disabled server (creating mcp.json itself when the file was missing).
func (m *Model) mcpAddSubmit() {
	switch m.mcpAddStage {
	case 0:
		name := strings.TrimSpace(m.addBuf)
		if name == "" {
			m.addErr = "name must not be empty"
			return
		}
		if _, exists := m.mcp.Server(name); exists {
			m.addErr = "server already exists"
			return
		}
		m.addBuf = name
		m.addErr = ""
		m.mcpAddStage = 1
	case 1:
		m.mcpAddType = mcpAddTypes[m.mcpAddTypeIdx]
		m.addErr = ""
		m.mcpAddStage = 2
	case 2:
		name := m.addBuf
		if err := m.mcp.AddServer(name, m.mcpAddType); err != nil {
			m.addErr = err.Error()
			return
		}
		m.closeOverlay()
		m.status = "added " + name + " (" + m.mcpAddType + ", disabled)"
		m.refresh()
		for i, row := range m.mcpRows {
			if row.name == name {
				m.entryIdx = i
				break
			}
		}
		m.refresh()
	}
}

// mcpAddMove moves the template cursor of the add overlay.
func (m *Model) mcpAddMove(dir int) {
	if m.overlay != OverlayMCPAdd || m.mcpAddStage != 1 {
		return
	}
	m.mcpAddTypeIdx = clamp(m.mcpAddTypeIdx+dir, len(mcpAddTypes))
}

// mcpReveal opens the reveal overlay: the raw env/headers values of the
// selected server, the only settled-state escape hatch for secrets.
func (m *Model) mcpReveal() {
	if m.mcp == nil {
		m.status = "mcp.json unavailable"
		return
	}
	name := m.mcpSelectedName()
	if name == "" {
		m.status = "no server selected"
		return
	}
	server, ok := m.mcp.Server(name)
	if !ok {
		return
	}
	lines := []string{name + " · " + server.Type, ""}
	lines = append(lines, mcpSecretLines("env", server.Fields["env"])...)
	lines = append(lines, mcpSecretLines("headers", server.Fields["headers"])...)
	m.revealText = joinLines(lines)
	m.overlay = OverlayMCPReveal
}

// mcpSecretLines renders one secret map's raw values in key order.
func mcpSecretLines(label string, value any) []string {
	entries := mapEntries(value)
	if len(entries) == 0 {
		return []string{label + ": (none)"}
	}
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := []string{label + ":"}
	for _, key := range keys {
		lines = append(lines, "  "+key+" = "+entries[key])
	}
	return lines
}

// mapEntries flattens a string-map value into displayable key/values.
func mapEntries(value any) map[string]string {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]string, len(v))
		for key, item := range v {
			if text, ok := item.(string); ok {
				out[key] = text
			} else {
				out[key] = formatMCPValue(item)
			}
		}
		return out
	case map[string]string:
		return v
	}
	return nil
}

// maskedMapLabel is the settled-state rendering of a secret map.
func maskedMapLabel(value any) string {
	n := 0
	switch v := value.(type) {
	case map[string]any:
		n = len(v)
	case map[string]string:
		n = len(v)
	}
	word := "keys"
	if n == 1 {
		word = "key"
	}
	return "••• " + itoa(n) + " " + word
}

// formatMCPValue renders one JSON value for the detail pane; maps and
// arrays marshal with sorted keys, so renders are deterministic.
func formatMCPValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(data)
}

// mcpEntryLines renders the Servers list for the inner width, or the empty
// state when no session is bound or no server matches the filter.
func (m *Model) mcpEntryLines(inner int) []string {
	if m.mcp == nil {
		return []string{m.styles.Help.Render(padRight(fitWidth("  mcp.json unavailable (unreadable or invalid)", inner), inner))}
	}
	if len(m.mcpRows) == 0 {
		text := "  (no servers) - press a to add the first one"
		if m.filter != "" {
			text = "  (no matches)"
		}
		return []string{m.styles.Help.Render(padRight(fitWidth(text, inner), inner))}
	}
	focused := m.focus == PaneEntries
	nameW := m.mcpNameWidth()
	lines := make([]string, 0, len(m.mcpRows))
	end := min(m.entryOffset+max(1, m.entriesViewport()), len(m.mcpRows))
	for i := m.entryOffset; i < end; i++ {
		row := m.mcpRows[i]
		state := "disabled"
		if row.enabled {
			state = "enabled"
		}
		text := padRight(row.name, nameW) + "  " + padRight(row.typ, 5) + "  " + padRight(state, 8)
		if row.auth != "" {
			text += "  \u26BF" + row.auth
		}
		dot := ""
		if row.dirty {
			dot = " " + m.styles.DirtyDot.Render("●")
		}
		text = fitWidth(text, max(0, inner-2-lipgloss.Width(" ●")))
		line := padRight("  "+text+dot, inner)
		if i == m.entryIdx {
			plain := "❯ " + text
			if focused {
				line = m.styles.Selected.Render(padRight(plain+dot, inner))
			} else {
				line = padRight(plain+dot, inner)
			}
		} else {
			line = m.styles.Normal.Render(line)
		}
		lines = append(lines, line)
	}
	return lines
}

// mcpNameWidth fits the name column to the longest visible name.
func (m *Model) mcpNameWidth() int {
	w := 8
	for _, row := range m.mcpRows {
		if n := lipgloss.Width(row.name); n > w {
			w = n
		}
	}
	if w > 20 {
		w = 20
	}
	return w
}

// mcpDetailLines renders the selected server's detail: the fixed
// source-path header, then the selectable rows (masked secrets included).
func (m *Model) mcpDetailLines(inner int) []string {
	if m.mcp == nil {
		return []string{m.styles.Help.Render(padRight(fitWidth("  mcp.json unavailable (unreadable or invalid)", inner), inner))}
	}
	if len(m.mcpDetail) == 0 {
		return []string{m.styles.Help.Render(padRight("  (no server selected)", inner))}
	}
	header := m.mcpSelectedName() + " · editable · " + m.mcpPath
	lines := []string{m.styles.Help.Render(padRight(fitWidth("  "+header, inner), inner))}
	labelW := m.mcpLabelWidth()
	focused := m.focus == PaneDetail
	end := min(m.detailOff+max(1, m.detailListViewport()), len(m.mcpDetail))
	for i := m.detailOff; i < end; i++ {
		entry := m.mcpDetail[i]
		text := padRight(entry.Label, labelW) + " " + entry.Value
		if entry.Secret {
			text += "    [x] reveal"
		}
		text = fitWidth(text, max(0, inner-2))
		painted := padRight("  "+text, inner)
		if i == m.detailLn {
			plain := "❯ " + text
			if focused {
				painted = m.styles.Selected.Render(padRight(plain, inner))
			} else {
				painted = padRight(plain, inner)
			}
		} else {
			painted = m.styles.Normal.Render(painted)
		}
		lines = append(lines, painted)
	}
	return lines
}

// mcpLabelWidth fits the detail label column to the longest label.
func (m *Model) mcpLabelWidth() int {
	w := 4
	for _, entry := range m.mcpDetail {
		if n := lipgloss.Width(entry.Label); n > w {
			w = n
		}
	}
	if w > 20 {
		w = 20
	}
	return w
}

// mcpAddTitle names the current add stage.
func (m *Model) mcpAddTitle() string {
	switch m.mcpAddStage {
	case 1:
		return "Add MCP server (template)"
	case 2:
		return "Add MCP server (confirm)"
	default:
		return "Add MCP server (name)"
	}
}

// mcpAddLines renders the add overlay's body for the current stage.
func (m *Model) mcpAddLines() []string {
	switch m.mcpAddStage {
	case 1:
		lines := []string{"server " + m.addBuf, ""}
		for i, typ := range mcpAddTypes {
			prefix := "  "
			if i == m.mcpAddTypeIdx {
				prefix = "❯ "
			}
			lines = append(lines, prefix+typ)
		}
		return append(lines, "", "[enter] next   [esc] cancel")
	case 2:
		return []string{"Create " + m.addBuf + " (" + m.mcpAddType + ")?", "", "[enter] create   [esc] cancel"}
	default:
		lines := []string{"> " + m.addBuf + "▌"}
		if m.addErr != "" {
			return append(lines, "", m.addErr)
		}
		return append(lines, "", "[enter] next   [esc] cancel")
	}
}
