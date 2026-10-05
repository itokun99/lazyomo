package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/itokun99/lazyomo/internal/editor"
)

// Editor mirrors the editor operations used by the TUI. The real
// *editor.Editor is wired in cmd/lazyomo; tests use a fake.
type Editor interface {
	Path() string
	Sections() []editor.Section
	Detail(section editor.SectionID, key string) (editor.Detail, error)
	SetScalar(path, value string) error
	ToggleBool(path string) error
	AddEntry(section editor.SectionID, key string) error
	RemoveEntry(section editor.SectionID, key string) error
	DirtyPaths() []string
	Save() (string, error)
	Reload() error
}

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
)

// confirmKind selects the pending confirmed action.
type confirmKind int

const (
	confirmNone confirmKind = iota
	confirmSave
	confirmReload
	confirmQuit
	confirmDelete
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
	ed     Editor
	styles Styles

	sections []editor.Section
	secIdx   int
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
	helpScroll  int

	filter     string
	filterMode bool
	status     string

	width  int
	height int

	secOffset   int
	entryOffset int
	detailOff   int
}

// New creates a TUI model over ed.
func New(ed Editor) *Model {
	m := &Model{ed: ed, styles: DefaultStyles(), focus: PaneSections}
	m.refresh()
	return m
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) curSection() *editor.Section {
	if len(m.sections) == 0 || m.secIdx < 0 || m.secIdx >= len(m.sections) {
		return nil
	}
	return &m.sections[m.secIdx]
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

func (m *Model) dirtyCount() int {
	if m.ed == nil {
		return 0
	}
	return len(m.ed.DirtyPaths())
}

// refresh reloads sections, entries, and detail from the editor.
func (m *Model) refresh() {
	if m.ed == nil {
		return
	}
	m.sections = m.ed.Sections()
	m.secIdx = clamp(m.secIdx, len(m.sections))
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
			if d, err := m.ed.Detail(sec.ID, e.Key); err == nil {
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

func (m *Model) secViewport() int {
	return max(1, m.height-2-2)
}

func (m *Model) entriesViewport() int {
	h := m.height - 2
	entriesH := h * 60 / 100
	return max(1, entriesH-2)
}

func (m *Model) detailViewport() int {
	h := m.height - 2
	entriesH := h * 60 / 100
	return max(1, h-entriesH-2)
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
