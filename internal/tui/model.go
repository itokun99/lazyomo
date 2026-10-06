package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/itokun99/lazyomo/internal/derived"
	"github.com/itokun99/lazyomo/internal/editor"
	"github.com/itokun99/lazyomo/internal/mcpfile"
	"github.com/itokun99/lazyomo/internal/workspace"
)

// Workspace is the TUI's data layer: the registered configuration sources
// with their provenance plus workspace-wide save and reload. The real
// *workspace.Registry (with its attached sessions) is wired in cmd/lazyomo
// only; tests use the fakeWorkspace harness.
type Workspace interface {
	Sources() []workspace.Source
	SaveAll() workspace.SaveResult
	Reload() error
	Stale() []workspace.StaleSource
}

// ConfigSurface is the editable section/entry surface a config source's
// session exposes. The TUI binds the first registered source whose session
// implements it as the active config document (the user omo.jsonc in the
// default registry).
type ConfigSurface interface {
	Sections() []editor.Section
	Detail(section editor.SectionID, key string) (editor.Detail, error)
	SetScalar(path, value string) error
	ToggleBool(path string) error
	AddEntry(section editor.SectionID, key string) error
	RemoveEntry(section editor.SectionID, key string) error
}

// Compile-time contract: the real editor is a config surface.
var _ ConfigSurface = (*editor.Editor)(nil)

// Pane is the focused zone.
type Pane int

const (
	PaneSections Pane = iota
	PaneEntries
	PaneDetail
)

// Overlay is the centered popup on screen.
type Overlay int

const (
	OverlayNone Overlay = iota
	OverlayConfirm
	OverlayHelp
	OverlayEdit
	OverlayAdd
	OverlayError
	OverlayMCPAdd
	OverlayMCPReveal
	OverlayPicker
)

// confirmKind selects the pending confirmed action.
type confirmKind int

const (
	confirmNone confirmKind = iota
	confirmSave
	confirmReload
	confirmQuit
	confirmDelete
	confirmMCPRemove
	confirmRename
)

// scrollMargin keeps the cursor away from list edges.
const scrollMargin = 2

// Minimum terminal dimensions.
const (
	MinWidth  = 80
	MinHeight = 24
)

// Model is the Bubble Tea model for the lazyomo editor TUI.
type Model struct {
	ws     Workspace
	config ConfigSurface
	// configPath is the path of the source backing config; the status line
	// shows it as the active document.
	configPath string
	styles     Styles

	sections []editor.Section
	// secIdx is the cursor over the flat selectable side-panel rows
	// (CONFIG + MCP + PROVIDERS), not over sections. curSection
	// resolves the bound editor section for config rows.
	secIdx   int
	groups   []sideGroup
	flat     []sideRow
	entries  []editor.Entry
	entryIdx int
	detail   editor.Detail
	detailLn int

	focus       Pane
	overlay     Overlay
	confirmKind confirmKind
	confirmText string
	delSec      editor.SectionID
	delKey      string
	delPath     string
	editPath    string
	editTitle   string
	editBuf     string
	editErr     string
	addBuf      string
	addErr      string
	errorText   string

	// MCP servers surface state: the bound session, its rendered list and
	// detail, and the overlay targets for the add/rename/reveal flows.
	mcp           MCPSurface
	mcpPath       string
	mcpRows       []mcpRow
	mcpDetail     []mcpDetailLine
	mcpAddStage   int
	mcpAddTypeIdx int
	mcpAddType    string
	// Derived providers/catalog/tools state: explicit file paths plus the
	// last loaded snapshots. Panes re-read on open so a changed file
	// reflects on the next refresh; nothing derived is ever written.
	modelsPath     string
	storePath      string
	toolsPath      string
	providers      derived.ProvidersSnapshot
	providersErr   string
	providerRows   []providerRow
	providerDetail []providerDetailLine
	catalog        derived.CatalogSnapshot
	catalogErr     string
	catalogRows    []catalogRow
	catalogDetail  []catalogDetailLine
	tools          derived.ToolsSnapshot
	toolsErr       string
	toolsRows      []toolsRow
	toolsDetail    []toolsDetailLine
	editMCP        bool
	editRename     bool
	editServer     string
	editField      string
	editKind       mcpfile.FieldKind
	revealText     string
	pendingMCP     string
	renameFrom     string
	renameTo       string
	helpScroll     int
	picker         *pickerState
	pickerBuild    func(targetPath string) []workspace.Candidate

	filter     string
	filterMode bool
	status     string

	width  int
	height int

	secOffset   int
	entryOffset int
	detailOff   int
}

// New creates a TUI model over ws.
func New(ws Workspace) *Model {
	m := &Model{ws: ws, styles: DefaultStyles(), focus: PaneSections}
	m.bindConfig()
	m.bindMCP()
	m.bindDerived()
	m.refresh()
	return m
}

// bindConfig picks the active config document: the first registered source
// whose session implements ConfigSurface. A workspace without one leaves
// the sections pane empty.
func (m *Model) bindConfig() {
	if m.ws == nil {
		return
	}
	for _, src := range m.ws.Sources() {
		if surface, ok := src.Session.(ConfigSurface); ok {
			m.config = surface
			m.configPath = src.Path
			return
		}
	}
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) selSide() *sideRow {
	if len(m.flat) == 0 || m.secIdx < 0 || m.secIdx >= len(m.flat) {
		return nil
	}
	return &m.flat[m.secIdx]
}

func (m *Model) curSection() *editor.Section {
	row := m.selSide()
	if row == nil || row.hasFuture {
		return nil
	}
	for i := range m.sections {
		if m.sections[i].ID == row.section {
			return &m.sections[i]
		}
	}
	return nil
}

func (m *Model) selEntry() *editor.Entry {
	if len(m.entries) == 0 || m.entryIdx < 0 || m.entryIdx >= len(m.entries) {
		return nil
	}
	return &m.entries[m.entryIdx]
}

func (m *Model) selLine() *editor.DetailLine {
	if len(m.detail.Lines) == 0 || m.detailLn < 0 || m.detailLn >= len(m.detail.Lines) {
		return nil
	}
	return &m.detail.Lines[m.detailLn]
}

// dirtyCount totals the changed paths across every registered source, the
// multi-document count the status line reports.
func (m *Model) dirtyCount() int {
	if m.ws == nil {
		return 0
	}
	total := 0
	for _, source := range m.ws.Sources() {
		if source.Session != nil {
			total += len(source.Session.DirtyPaths())
		}
	}
	return total
}

// activePath names the document the sections pane edits.
func (m *Model) activePath() string {
	if m.sideMCP() && m.mcpPath != "" {
		return m.mcpPath
	}
	if m.sideProviders() && m.modelsPath != "" {
		return m.modelsPath
	}
	if m.sideCatalog() && m.storePath != "" {
		return m.storePath
	}
	if m.sideTools() && m.toolsPath != "" {
		return m.toolsPath
	}
	if m.configPath == "" {
		return "(no file)"
	}
	return m.configPath
}

// refresh reloads sections, entries, and detail from the active config
// surface.
func (m *Model) refresh() {
	if m.config == nil {
		return
	}
	m.sections = m.config.Sections()
	m.groups = buildSideGroups(m.sections)
	m.flat = flattenSide(m.groups)
	m.secIdx = clamp(m.secIdx, len(m.flat))
	m.secOffset = clampWindow(m.secOffset, sideCursorLine(m.groups, m.secIdx), m.secViewport(), sideTotalLines(m.groups))
	if m.sideMCP() {
		m.refreshMCP()
		return
	}
	if m.sideProviders() {
		m.refreshProviders()
		return
	}
	if m.sideCatalog() {
		m.refreshCatalog()
		return
	}
	if m.sideTools() {
		m.refreshTools()
		return
	}
	sec := m.curSection()
	m.entries = nil
	if sec != nil {
		m.entries = filterEntries(sec.Entries, m.filter)
	}
	m.entryIdx = clamp(m.entryIdx, len(m.entries))
	m.detail = editor.Detail{}
	m.detailLn = 0
	if sec != nil {
		if e := m.selEntry(); e != nil {
			if d, err := m.config.Detail(sec.ID, e.Key); err == nil {
				m.detail = d
			}
		}
	}
	m.detailLn = clamp(m.detailLn, len(m.detail.Lines))
	m.secOffset = adjustOffset(m.secOffset, m.secIdx, m.secViewport())
	m.entryOffset = adjustOffset(m.entryOffset, m.entryIdx, m.entriesViewport())
	m.detailOff = adjustOffset(m.detailOff, m.detailLn, m.detailViewport())
}

func filterEntries(entries []editor.Entry, filter string) []editor.Entry {
	if filter == "" {
		return entries
	}
	out := make([]editor.Entry, 0, len(entries))
	for _, e := range entries {
		if containsFold(e.Key, filter) {
			out = append(out, e)
		}
	}
	return out
}

func (m *Model) contentH() int {
	return m.height - 2
}

func (m *Model) entriesH() int {
	return m.contentH() * 60 / 100
}

func (m *Model) detailH() int {
	return m.contentH() - m.entriesH()
}

func (m *Model) secViewport() int {
	return max(1, m.contentH()-2)
}

func (m *Model) entriesViewport() int {
	return max(1, m.entriesH()-2)
}

func (m *Model) detailViewport() int {
	return max(1, m.detailH()-2)
}

// pickerChromeLines is the non-result chrome of the todo-13 picker box:
// top/bottom borders, query line, header, footer, and one spacer. At
// 80x24 the box is 76x20 for 14 result rows; at 120x40 it is 110x30
// for 24. The picker overlay itself lands in a later wave; the geometry
// and the binding table already pin its contract.
const pickerChromeLines = 6

func pickerBox(w, h int) (int, int) {
	return min(110, w-4), min(30, h-4)
}

func pickerResultRows(w, h int) int {
	_, bh := pickerBox(w, h)
	return max(1, bh-pickerChromeLines)
}

// dirtyFile is one unsaved source for the save/quit confirms.
type dirtyFile struct {
	path  string
	count int
	stale bool
}

func (m *Model) dirtyFiles() []dirtyFile {
	if m.ws == nil {
		return nil
	}
	stale := map[string]bool{}
	for _, s := range m.ws.Stale() {
		stale[s.SourceID] = true
	}
	var out []dirtyFile
	for _, src := range m.ws.Sources() {
		if !src.Writable || src.Session == nil {
			continue
		}
		if n := len(src.Session.DirtyPaths()); n > 0 {
			out = append(out, dirtyFile{path: src.Path, count: n, stale: stale[src.ID]})
		}
	}
	return out
}

// confirmLines builds a confirm body: title plus one line per dirty file,
// stale files marked blocked.
func confirmLines(title string, files []dirtyFile) string {
	lines := []string{title}
	for _, f := range files {
		line := f.path + " \u00b7 dirty(" + itoa(f.count) + ")"
		if f.stale {
			line += " \u00b7 stale, blocked"
		}
		lines = append(lines, "  "+line)
	}
	return joinLines(lines)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}

func adjustOffset(offset, idx, viewport int) int {
	if viewport <= 0 {
		return 0
	}
	if idx < offset+scrollMargin {
		offset = idx - scrollMargin
	}
	if idx > offset+viewport-1-scrollMargin {
		offset = idx - viewport + 1 + scrollMargin
	}
	if offset < 0 {
		offset = 0
	}
	return offset
}

func clamp(idx, n int) int {
	if n == 0 {
		return 0
	}
	if idx < 0 {
		return 0
	}
	if idx >= n {
		return n - 1
	}
	return idx
}

func containsFold(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(sub) > len(s) {
		// still may match via folding only if lengths allow; fast path out
	}
	ls, lb := toLower(s), toLower(sub)
	return indexOf(ls, lb) >= 0
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

func indexOf(s, sub string) int {
	if sub == "" {
		return 0
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// Compile-time check that Model implements tea.Model.
var _ tea.Model = (*Model)(nil)
