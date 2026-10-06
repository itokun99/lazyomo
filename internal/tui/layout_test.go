package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/itokun99/lazyomo/internal/editor"
)

// asciiModel returns a model with an explicitly pinned Ascii profile: the
// contract's zero-ESC assertion and the readable goldens both build on it.
func asciiModel(w, h int) *Model {
	m := newTestModel(newFakeWorkspace(newFakeEditor()))
	r := lipgloss.NewRenderer(os.Stdout)
	r.SetColorProfile(termenv.Ascii)
	m.styles = StylesWithRenderer(r)
	m.width = w
	m.height = h
	m.refresh()
	return m
}

func baseLines(m *Model) []string {
	return strings.Split(m.View(), "\n")
}

func TestDensity80x24(t *testing.T) {
	m := newTestModel(newFakeWorkspace(newFakeEditor()))
	m.width = 80
	m.height = 24
	m.refresh()
	lines := baseLines(m)
	if len(lines) != 24 {
		t.Fatalf("painted lines = %d, want exactly 24", len(lines))
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > 80 {
			t.Errorf("line %d width = %d, want <= 80: %q", i, w, l)
		}
	}
	if got := m.entriesViewport(); got < 11 {
		t.Errorf("entry rows = %d, want >= 11", got)
	}
	if got := m.detailViewport(); got < 6 {
		t.Errorf("detail rows = %d, want >= 6", got)
	}
}

func TestDensity120x40(t *testing.T) {
	m := newTestModel(newFakeWorkspace(newFakeEditor()))
	m.width = 120
	m.height = 40
	m.refresh()
	lines := baseLines(m)
	if len(lines) != 40 {
		t.Fatalf("painted lines = %d, want exactly 40", len(lines))
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > 120 {
			t.Errorf("line %d width = %d, want <= 120: %q", i, w, l)
		}
	}
}

func TestPaneBordersAndTitles(t *testing.T) {
	m := asciiModel(80, 24)
	view := m.View()
	if got := strings.Count(view, "╭"); got != 3 {
		t.Errorf("pane top corners = %d, want 3", got)
	}
	if got := strings.Count(view, "╰"); got != 3 {
		t.Errorf("pane bottom corners = %d, want 3", got)
	}
	if got := strings.Count(view, "╮") + strings.Count(view, "╯"); got != 6 {
		t.Errorf("right corners = %d, want 6", got)
	}
	for _, want := range []string{"Sections", "Models", "models/k1"} {
		if !strings.Contains(view, want) {
			t.Errorf("pane titles missing %q:\n%s", want, view)
		}
	}
	first := baseLines(m)[0]
	if !strings.Contains(first, "╭─ Sections ") && !strings.Contains(first, "╭─ ● Sections ") {
		t.Errorf("side panel title must read Sections inside its top border: %q", first)
	}
}

func TestSingleFocusCue(t *testing.T) {
	for _, focus := range []Pane{PaneSections, PaneEntries, PaneDetail} {
		m := asciiModel(80, 24)
		m.focus = focus
		if got := strings.Count(m.View(), "●"); got != 1 {
			t.Errorf("focus %v: focus markers = %d, want exactly 1", focus, got)
		}
	}
}

func TestCursorOncePerList(t *testing.T) {
	m := asciiModel(80, 24)
	if got := strings.Count(m.View(), "❯"); got != 3 {
		t.Errorf("❯ count = %d, want exactly 3 (one per list)", got)
	}
	m.focus = PaneEntries
	if got := strings.Count(m.View(), "❯"); got != 3 {
		t.Errorf("❯ count after focus move = %d, want 3", got)
	}
}

func TestKeybarFromTable(t *testing.T) {
	contexts := map[Pane]string{
		PaneSections: ctxSections,
		PaneEntries:  ctxEntries,
		PaneDetail:   ctxDetail,
	}
	for focus, ctx := range contexts {
		m := asciiModel(100, 30)
		m.focus = focus
		m.refresh()
		lines := baseLines(m)
		bar := strings.TrimRight(lines[len(lines)-2], " ")
		if bar != keybarFor(ctx) {
			t.Errorf("focus %v keybar = %q, want table %q", focus, bar, keybarFor(ctx))
		}
		if w := lipgloss.Width(bar); w > m.width {
			t.Errorf("focus %v keybar width = %d", focus, w)
		}
	}
}

func TestKeybarHiddenUnderOverlay(t *testing.T) {
	m := asciiModel(100, 30)
	m = sendKeys(t, m, "?")
	view := m.View()
	for _, ctx := range []string{ctxSections, ctxEntries, ctxDetail} {
		bar := keybarFor(ctx)
		for _, l := range strings.Split(view, "\n") {
			if l == bar {
				t.Errorf("overlay open but keybar %q still painted", bar)
			}
		}
	}
}

func TestStatusLineFormat(t *testing.T) {
	m := asciiModel(80, 24)
	lines := baseLines(m)
	status := lines[len(lines)-1]
	if !strings.Contains(status, m.activePath()) {
		t.Errorf("status missing active path: %q", status)
	}
	if !strings.Contains(status, "·") {
		t.Errorf("status missing · separator: %q", status)
	}
	if !strings.Contains(status, "clean") {
		t.Errorf("status missing clean state: %q", status)
	}
	count := 0
	for _, l := range lines {
		if strings.Contains(l, " · ") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("status-pattern lines = %d, want exactly 1", count)
	}
	f := newFakeEditor()
	f.dirty = []string{"/models/k1", "/models/b1"}
	m2 := asciiModel(80, 24)
	m2.ws = newFakeWorkspace(f)
	m2.bindConfig()
	m2.refresh()
	status = baseLines(m2)[23]
	if !strings.Contains(status, "● dirty(2)") {
		t.Errorf("dirty status = %q, want ● dirty(2)", status)
	}
}

func TestStaleInSaveConfirm(t *testing.T) {
	f := newFakeEditor()
	f.dirty = []string{"/models/k1"}
	ws := newFakeWorkspace(f)
	mcp := &stubSession{path: "/tmp/agent/mcp.json", dirty: []string{"/mcpServers/fetch"}}
	ws.addSource("mcp", mcp.path, "external", "mcpservers", mcp)
	ws.stale = map[string]bool{"mcp": true}
	m := newTestModel(ws)
	m = sendKeys(t, m, "s")
	if m.overlay != OverlayConfirm {
		t.Fatalf("overlay = %v, want confirm", m.overlay)
	}
	for _, want := range []string{f.path, mcp.path, "stale, blocked"} {
		if !strings.Contains(m.confirmText, want) {
			t.Errorf("confirm missing %q:\n%s", want, m.confirmText)
		}
	}
}

func TestBindingTableConsistency(t *testing.T) {
	for _, table := range bindingTables {
		bar := keybarFor(table.Name)
		parts := strings.Split(bar, "  ")
		allowed := map[string]bool{}
		for _, b := range table.Keys {
			pair := b.Label + " " + b.Desc
			allowed[pair] = true
			if b.Bar && !strings.Contains(bar, pair) {
				t.Errorf("context %s keybar missing bar pair %q", table.Name, pair)
			}
		}
		for _, p := range parts {
			if p == "" {
				continue
			}
			if !allowed[p] {
				t.Errorf("context %s keybar has extra pair %q outside the table", table.Name, p)
			}
		}
	}
	m := asciiModel(100, 30)
	help := strings.Join(m.helpLines(), "\n")
	for _, table := range bindingTables {
		if !strings.Contains(help, table.Title) {
			t.Errorf("help missing context %q", table.Title)
		}
		for _, b := range table.Keys {
			if !strings.Contains(help, b.Label) || !strings.Contains(help, b.Desc) {
				t.Errorf("help missing pair %q/%q of context %s", b.Label, b.Desc, table.Name)
			}
		}
	}
}

func TestGroupedSidePanelOrder(t *testing.T) {
	m := newTestModel(newFakeWorkspace(newFakeEditor()))
	if len(m.groups) != 3 {
		t.Fatalf("groups = %d, want 3", len(m.groups))
	}
	if m.groups[0].title != "CONFIG" || m.groups[1].title != "MCP" || m.groups[2].title != "PROVIDERS" {
		t.Fatalf("group titles = %q %q %q", m.groups[0].title, m.groups[1].title, m.groups[2].title)
	}
	var got []string
	for _, row := range m.flat {
		got = append(got, row.label)
	}
	want := []string{"Models", "Model Profiles", "Agents", "Categories", "Telemetry", "Git", "$schema", "Servers", "Tools", "Connections", "Catalog"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("side rows = %v, want %v", got, want)
	}
	if !m.groups[2].ro {
		t.Error("PROVIDERS group must be read-only")
	}
}

func TestDigitJumpAcrossGroups(t *testing.T) {
	m := newTestModel(newFakeWorkspace(newFakeEditor()))
	m = sendKeys(t, m, "6")
	if m.flat[m.secIdx].label != "Git" {
		t.Errorf("key 6 -> %q, want Git", m.flat[m.secIdx].label)
	}
	m = sendKeys(t, m, "0")
	if m.flat[m.secIdx].label != "Connections" {
		t.Errorf("key 0 -> %q, want Connections", m.flat[m.secIdx].label)
	}
	m = sendKeys(t, m, "8")
	if row := m.selSide(); row == nil || row.label != "Servers" {
		t.Errorf("key 8 -> %+v, want Servers", row)
	}
	if len(m.entries) != 0 {
		t.Errorf("placeholder entries = %d, want empty", len(m.entries))
	}
	m = sendKeys(t, m, "e", "a", "d", " ")
	if m.status != "read-only section" {
		t.Errorf("status = %q, want read-only section", m.status)
	}
	if m.overlay != OverlayNone {
		t.Errorf("overlay = %v, want none on placeholder", m.overlay)
	}
}

func TestScrollMarginTwoRows(t *testing.T) {
	f := newFakeEditor()
	for i := 0; i < 30; i++ {
		key := "k" + itoa(i+10)
		f.sections[0].Entries = append(f.sections[0].Entries, editor.Entry{
			Key: key, Path: "/models/" + key, Kind: editor.KindScalar, Value: "v",
		})
	}
	m := newTestModel(newFakeWorkspace(f))
	m.width = 80
	m.height = 24
	m.focus = PaneEntries
	m.refresh()
	last := len(m.entries) - 1
	for i := 0; i < last; i++ {
		m = sendKeys(t, m, "j")
	}
	if m.entryIdx != last {
		t.Fatalf("entryIdx = %d, want %d", m.entryIdx, last)
	}
	view := max(1, m.entriesViewport())
	if want := last - view + 1 + scrollMargin; m.entryOffset != want {
		t.Errorf("entryOffset = %d, want %d (2-row margin)", m.entryOffset, want)
	}
	for _, l := range baseLines(m) {
		if w := lipgloss.Width(l); w > 80 {
			t.Errorf("scrolled line width = %d: %q", w, l)
		}
	}
}

func TestLongNamesNeverWrap(t *testing.T) {
	f := newFakeEditor()
	long := strings.Repeat("abcdefghij", 20)
	f.sections[0].Entries = append(f.sections[0].Entries, editor.Entry{
		Key: long, Path: "/models/" + long, Kind: editor.KindScalar, Value: long,
	})
	m := newTestModel(newFakeWorkspace(f))
	m.width = 80
	m.height = 24
	m.refresh()
	lines := baseLines(m)
	if len(lines) != 24 {
		t.Fatalf("painted lines = %d, want 24", len(lines))
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > 80 {
			t.Errorf("line %d width = %d, want <= 80", i, w)
		}
	}
}

func TestTerminalTooSmall79x23(t *testing.T) {
	m := asciiModel(79, 23)
	if !strings.Contains(m.View(), "Terminal too small") {
		t.Errorf("79x23 must refuse:\n%s", m.View())
	}
}

func TestAsciiProfileZeroESC(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		m := asciiModel(size[0], size[1])
		if strings.Contains(m.View(), "\x1b") {
			t.Errorf("Ascii profile at %dx%d must carry zero ESC bytes", size[0], size[1])
		}
	}
}

func TestPickerGeometryContract(t *testing.T) {
	if got := pickerResultRows(80, 24); got != 14 {
		t.Errorf("picker rows at 80x24 = %d, want 14", got)
	}
	if got := pickerResultRows(120, 40); got != 24 {
		t.Errorf("picker rows at 120x40 = %d, want 24", got)
	}
}

func TestGoldens(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		m := asciiModel(size[0], size[1])
		got := m.View()
		path := filepath.Join("testdata", filepath.FromSlash(
			"layout-"+itoa(size[0])+"x"+itoa(size[1])+".golden"))
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			if err := os.MkdirAll("testdata", 0o755); err != nil {
				t.Fatalf("mkdir testdata: %v", err)
			}
			if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
				t.Fatalf("writing %s: %v", path, err)
			}
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s (run with UPDATE_GOLDEN=1 first): %v", path, err)
		}
		if string(raw) != got {
			t.Errorf("golden %dx%d mismatch: run UPDATE_GOLDEN=1 to refresh after review", size[0], size[1])
		}
	}
}
