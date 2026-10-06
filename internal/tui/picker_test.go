package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/itokun99/lazyomo/internal/editor"
	"github.com/itokun99/lazyomo/internal/fuzzy"
	"github.com/itokun99/lazyomo/internal/workspace"
)

func pickerCandsFixture() []workspace.Candidate {
	return []workspace.Candidate{
		{Value: "acme/code-large", Group: fuzzy.GroupRef, Provider: "acme", Usages: []string{"/models/k3/model"}},
		{Value: "acme/other-1", Group: fuzzy.GroupRef, Provider: "acme"},
		{Value: "beta/model-x", Group: fuzzy.GroupRef, Provider: "beta", Usages: []string{"/categories/quick/models/0"}},
		{Value: "k1", Group: fuzzy.GroupAlias, Usages: []string{"/agents/sisyphus/model"}},
		{Value: "sonnet", Group: fuzzy.GroupAlias, Usages: []string{"/categories/quick/model"}},
	}
}

func pickerFakeEditor() *fakeEditor {
	f := newFakeEditor()
	for i, s := range f.sections {
		if s.ID == editor.SectionAgents {
			f.sections[i].Entries = append(f.sections[i].Entries, editor.Entry{
				Key: "sisyphus", Path: "/agents/sisyphus", Kind: editor.KindAlias, Value: "k1",
			})
		}
		if s.ID == editor.SectionCategories {
			f.sections[i].Entries = append(f.sections[i].Entries, editor.Entry{
				Key: "quick", Path: "/categories/quick", Kind: editor.KindAlias, Value: "sonnet",
			})
		}
	}
	f.details["agents\x00sisyphus"] = editor.Detail{Title: "sisyphus", Lines: []editor.DetailLine{
		{Label: "model", Path: "/agents/sisyphus/model", Value: "k1", Editable: true},
		{Label: "models", Path: "/agents/sisyphus/models", Value: `["k1"]`, Editable: true},
	}}
	f.details["categories\x00quick"] = editor.Detail{Title: "quick", Lines: []editor.DetailLine{
		{Label: "model", Path: "/categories/quick/model", Value: "sonnet", Editable: true},
	}}
	return f
}

func newPickerModel(cands []workspace.Candidate) (*fakeEditor, *Model) {
	f := pickerFakeEditor()
	ws := newFakeWorkspace(f)
	m := newTestModel(ws)
	cp := append([]workspace.Candidate(nil), cands...)
	m.pickerBuild = func(targetPath string) []workspace.Candidate {
		return append([]workspace.Candidate(nil), cp...)
	}
	m.width = 100
	m.height = 30
	m.refresh()
	return f, m
}

func openModelsPicker(t *testing.T, m *Model) *Model {
	t.Helper()
	m.secIdx = 0
	m.entryIdx = 3
	m.focus = PaneEntries
	m.refresh()
	m = sendKeys(t, m, "enter")
	if m.focus != PaneDetail {
		t.Fatalf("focus = %v, want detail", m.focus)
	}
	m.detailLn = 0
	m = sendKeys(t, m, "e")
	return m
}

func openAgentsPicker(t *testing.T, m *Model) *Model {
	t.Helper()
	m.secIdx = 2
	m.entryIdx = 0
	m.focus = PaneEntries
	m.refresh()
	m = sendKeys(t, m, "enter")
	if m.focus != PaneDetail {
		t.Fatalf("focus = %v, want detail", m.focus)
	}
	m.detailLn = 0
	m = sendKeys(t, m, "e")
	return m
}

func runeKey(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func ctrlKey(typ tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: typ}
}

func specialKey(typ tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: typ}
}

func sendMsg(t *testing.T, m *Model, msg tea.KeyMsg) *Model {
	t.Helper()
	model, _ := m.Update(msg)
	return model.(*Model)
}

func TestPickerFlows(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "open via e shows picker with footer",
			run: func(t *testing.T) {
				_, m := newPickerModel(pickerCandsFixture())
				m = openAgentsPicker(t, m)
				if m.overlay != OverlayPicker {
					t.Fatalf("overlay = %v, want picker", m.overlay)
				}
				if m.picker == nil {
					t.Fatal("picker state is nil")
				}
				view := m.View()
				for _, want := range []string{"Pick model", "type to filter", "enter select", "tab preview", "ctrl+e raw", "esc cancel"} {
					if !strings.Contains(view, want) {
						t.Errorf("picker view missing %q:\n%s", want, view)
					}
				}
				if got := m.contextName(); got != ctxPicker {
					t.Errorf("context = %q, want picker", got)
				}
			},
		},
		{
			name: "cancel via esc writes nothing",
			run: func(t *testing.T) {
				f, m := newPickerModel(pickerCandsFixture())
				before := len(f.DirtyPaths())
				m = openAgentsPicker(t, m)
				m = sendKeys(t, m, "x", "y")
				m = sendKeys(t, m, "esc")
				if m.overlay != OverlayNone {
					t.Errorf("overlay = %v, want none", m.overlay)
				}
				if len(f.sets) != 0 {
					t.Errorf("sets = %v, want none", f.sets)
				}
				if len(f.DirtyPaths()) != before {
					t.Errorf("dirty = %v, want unchanged", f.DirtyPaths())
				}
			},
		},
		{
			name: "select writes exact id verbatim",
			run: func(t *testing.T) {
				f, m := newPickerModel(pickerCandsFixture())
				m = openAgentsPicker(t, m)
				m = sendMsg(t, m, runeKey("code-large"))
				if len(m.picker.results) == 0 {
					t.Fatalf("no results for code-large")
				}
				top := m.picker.cands[m.picker.results[m.picker.cursor].Index].Value
				if top != "acme/code-large" {
					t.Fatalf("top = %q, want acme/code-large", top)
				}
				m = sendMsg(t, m, specialKey(tea.KeyEnter))
				if m.overlay != OverlayNone {
					t.Errorf("overlay = %v, want none after select", m.overlay)
				}
				if len(f.sets) != 1 {
					t.Fatalf("sets = %v, want one write", f.sets)
				}
				if f.sets[0] != [2]string{"/agents/sisyphus/model", "acme/code-large"} {
					t.Errorf("sets[0] = %v, want exact id write-back", f.sets[0])
				}
				if !strings.Contains(m.status, "set /agents/sisyphus/model") {
					t.Errorf("status = %q, want write-path", m.status)
				}
				if len(f.DirtyPaths()) == 0 {
					t.Error("expected dirty after select")
				}
			},
		},
		{
			name: "paste appends all runes at once",
			run: func(t *testing.T) {
				_, m := newPickerModel(pickerCandsFixture())
				m = openAgentsPicker(t, m)
				m = sendMsg(t, m, runeKey("code-lar"))
				if got := string(m.picker.query); got != "code-lar" {
					t.Errorf("query = %q, want code-lar", got)
				}
				if len(m.picker.results) == 0 {
					t.Error("paste query should still match")
				}
			},
		},
		{
			name: "no-match enter stays open",
			run: func(t *testing.T) {
				f, m := newPickerModel(pickerCandsFixture())
				m = openAgentsPicker(t, m)
				m = sendMsg(t, m, runeKey("zzz-no-such-model"))
				if len(m.picker.results) != 0 {
					t.Fatalf("results = %d, want 0", len(m.picker.results))
				}
				if !strings.Contains(m.View(), "no matches") {
					t.Errorf("view should say no matches:\n%s", m.View())
				}
				m = sendMsg(t, m, specialKey(tea.KeyEnter))
				if m.overlay != OverlayPicker {
					t.Errorf("overlay = %v, want picker kept open on no-match", m.overlay)
				}
				if len(f.sets) != 0 {
					t.Errorf("sets = %v, want none", f.sets)
				}
			},
		},
		{
			name: "already-set closes unchanged with no write",
			run: func(t *testing.T) {
				f, m := newPickerModel([]workspace.Candidate{
					{Value: "acme/code-large", Group: fuzzy.GroupRef, Provider: "acme"},
					{Value: "acme/other-1", Group: fuzzy.GroupRef, Provider: "acme"},
				})
				m.secIdx = 2
				m.entryIdx = 0
				m.focus = PaneEntries
				m.refresh()
				m = sendKeys(t, m, "enter")
				m.detailLn = 0
				f.details["agents\x00sisyphus"] = editor.Detail{Title: "sisyphus", Lines: []editor.DetailLine{
					{Label: "model", Path: "/agents/sisyphus/model", Value: "acme/code-large", Editable: true},
				}}
				m.refresh()
				m = sendKeys(t, m, "e")
				if m.overlay != OverlayPicker {
					t.Fatalf("overlay = %v, want picker", m.overlay)
				}
				m = sendMsg(t, m, runeKey("code-large"))
				m = sendMsg(t, m, specialKey(tea.KeyEnter))
				if m.overlay != OverlayNone {
					t.Errorf("overlay = %v, want closed", m.overlay)
				}
				if !strings.Contains(m.status, "unchanged") {
					t.Errorf("status = %q, want unchanged", m.status)
				}
				if len(f.sets) != 0 {
					t.Errorf("sets = %v, want no write on unchanged", f.sets)
				}
				if len(f.DirtyPaths()) != 0 {
					t.Errorf("dirty = %v, want none", f.DirtyPaths())
				}
			},
		},
		{
			name: "dupes select first without extra write",
			run: func(t *testing.T) {
				f, m := newPickerModel([]workspace.Candidate{
					{Value: "acme/code-large", Group: fuzzy.GroupRef, Provider: "acme", Usages: []string{"/a"}},
					{Value: "acme/code-large", Group: fuzzy.GroupRef, Provider: "acme", Usages: []string{"/b"}},
					{Value: "beta/model-x", Group: fuzzy.GroupRef, Provider: "beta"},
				})
				m = openAgentsPicker(t, m)
				m = sendMsg(t, m, runeKey("code-large"))
				if len(m.picker.results) != 2 {
					t.Fatalf("dupe results = %d, want 2", len(m.picker.results))
				}
				m = sendMsg(t, m, specialKey(tea.KeyEnter))
				if m.overlay != OverlayNone {
					t.Fatalf("overlay = %v, want closed", m.overlay)
				}
				if len(f.sets) != 1 || f.sets[0][1] != "acme/code-large" {
					t.Errorf("sets = %v, want single dupe write", f.sets)
				}
			},
		},
		{
			name: "long query caps at 128 with tail display",
			run: func(t *testing.T) {
				_, m := newPickerModel(pickerCandsFixture())
				m = openAgentsPicker(t, m)
				long := strings.Repeat("abcdefghij", 30)
				if len([]rune(long)) <= 255 {
					t.Fatalf("fixture long len = %d, want >255", len([]rune(long)))
				}
				m = sendMsg(t, m, runeKey(long))
				if got := len(m.picker.query); got != pickerMaxQuery {
					t.Errorf("query runes = %d, want cap %d", got, pickerMaxQuery)
				}
				view := m.View()
				for _, l := range strings.Split(view, "\n") {
					if w := lipgloss.Width(l); w > m.width {
						t.Errorf("picker line width %d exceeds %d: %q", w, m.width, l)
					}
				}
				tail := string(m.picker.query[len(m.picker.query)-10:])
				if !strings.Contains(view, tail) {
					t.Errorf("view missing query tail %q:\n%s", tail, view)
				}
				if strings.Contains(view, long) {
					t.Error("view must not contain the full 300-rune query")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func TestPickerRefsOnlyVsFullSet(t *testing.T) {
	mixed := []workspace.Candidate{
		{Value: "k1", Group: fuzzy.GroupAlias},
		{Value: "sonnet", Group: fuzzy.GroupAlias},
		{Value: "acme/code-large", Group: fuzzy.GroupRef, Provider: "acme"},
		{Value: "beta/model-x", Group: fuzzy.GroupRef, Provider: "beta"},
	}
	_, m := newPickerModel(mixed)
	m = openModelsPicker(t, m)
	if m.overlay != OverlayPicker {
		t.Fatalf("models overlay = %v, want picker", m.overlay)
	}
	for _, c := range m.picker.cands {
		if c.Group == fuzzy.GroupAlias {
			t.Errorf("models-section picker must not offer alias %q", c.Value)
		}
	}
	if len(m.picker.cands) != 2 {
		t.Errorf("models cands = %v, want 2 refs only", m.picker.cands)
	}
	m = sendKeys(t, m, "esc")

	_, m2 := newPickerModel(mixed)
	m2 = openAgentsPicker(t, m2)
	if m2.overlay != OverlayPicker {
		t.Fatalf("agents overlay = %v, want picker", m2.overlay)
	}
	foundAlias := false
	for _, c := range m2.picker.cands {
		if c.Group == fuzzy.GroupAlias {
			foundAlias = true
		}
	}
	if !foundAlias {
		t.Errorf("alias-accepting field must keep aliases, got %v", m2.picker.cands)
	}
	if len(m2.picker.cands) != 4 {
		t.Errorf("agents cands = %d, want full set 4", len(m2.picker.cands))
	}
}

func TestPickerRebuildPerOpen(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "models-store.json")
	writeFixture(t, storePath, `{"acme": {"models": [{"id": "first-1", "provider": "acme"}]}}`)
	modelsPath := filepath.Join(dir, "models.json")
	writeFixture(t, modelsPath, `{"providers": {}}`)
	f := pickerFakeEditor()
	ws := newFakeWorkspace(f)
	m := newTestModel(ws)
	m.pickerBuild = func(targetPath string) []workspace.Candidate {
		return workspace.CatalogCandidates(storePath, modelsPath)
	}
	m.width = 100
	m.height = 30
	m.refresh()
	m = openAgentsPicker(t, m)
	if !strings.Contains(m.View(), "acme/first-1") {
		t.Fatalf("first open missing first-1:\n%s", m.View())
	}
	m = sendKeys(t, m, "esc")
	writeFixture(t, storePath, `{"beta": {"models": [{"id": "second-2", "provider": "beta"}]}}`)
	m = openAgentsPicker(t, m)
	view := m.View()
	if !strings.Contains(view, "beta/second-2") {
		t.Errorf("second open missing rebuilt second-2:\n%s", view)
	}
	if strings.Contains(view, "acme/first-1") {
		t.Errorf("rebuilt picker must not keep stale first-1:\n%s", view)
	}
}

func TestPickerGeometry(t *testing.T) {
	for _, tc := range []struct {
		w, h, wantRows int
	}{
		{80, 24, 14},
		{120, 40, 24},
	} {
		_, m := newPickerModel(pickerCandsFixture())
		m.width = tc.w
		m.height = tc.h
		m.refresh()
		m = openAgentsPicker(t, m)
		if got := pickerResultRows(tc.w, tc.h); got != tc.wantRows {
			t.Errorf("%dx%d rows = %d, want %d", tc.w, tc.h, got, tc.wantRows)
		}
		if bw, bh := pickerBox(tc.w, tc.h); bw != min(110, tc.w-4) || bh != min(30, tc.h-4) {
			t.Errorf("box = %dx%d, want %dx%d", bw, bh, min(110, tc.w-4), min(30, tc.h-4))
		}
		view := m.View()
		for _, want := range []string{"Pick model", "type to filter", "enter select", "tab preview", "ctrl+e raw", "esc cancel"} {
			if !strings.Contains(view, want) {
				t.Errorf("%dx%d view missing %q:\n%s", tc.w, tc.h, want, view)
			}
		}
		for i, l := range strings.Split(view, "\n") {
			if w := lipgloss.Width(l); w > tc.w {
				t.Errorf("%dx%d line %d width %d exceeds %d: %q", tc.w, tc.h, i, w, tc.w, l)
			}
		}
	}
}

func TestPickerNavigationPreviewRawQuit(t *testing.T) {
	many := []workspace.Candidate{}
	for i := 0; i < 30; i++ {
		many = append(many, workspace.Candidate{
			Value: "acme/model-" + itoa(i), Group: fuzzy.GroupRef, Provider: "acme",
		})
	}
	_, m := newPickerModel(many)
	m.width = 100
	m.height = 30
	m.refresh()
	m = openAgentsPicker(t, m)
	if !m.picker.preview {
		t.Error("width 100 must auto-enable preview")
	}
	for _, want := range []string{"value:", "group:", "provider:", "used-in:", "write-path:"} {
		if !strings.Contains(m.View(), want) {
			t.Errorf("preview missing %q:\n%s", want, m.View())
		}
	}
	start := m.picker.cursor
	m = sendMsg(t, m, specialKey(tea.KeyDown))
	if m.picker.cursor != start+1 {
		t.Errorf("down cursor = %d, want %d", m.picker.cursor, start+1)
	}
	m = sendMsg(t, m, ctrlKey(tea.KeyCtrlP))
	if m.picker.cursor != start {
		t.Errorf("ctrl+p cursor = %d, want %d", m.picker.cursor, start)
	}
	m = sendMsg(t, m, ctrlKey(tea.KeyCtrlN))
	if m.picker.cursor != start+1 {
		t.Errorf("ctrl+n cursor = %d, want %d", m.picker.cursor, start+1)
	}
	m = sendMsg(t, m, specialKey(tea.KeyPgDown))
	if m.picker.cursor <= start+1 {
		t.Errorf("pgdown should page, cursor = %d", m.picker.cursor)
	}
	m = sendMsg(t, m, specialKey(tea.KeyPgUp))
	m = sendMsg(t, m, specialKey(tea.KeyEnd))
	if m.picker.cursor != len(m.picker.results)-1 {
		t.Errorf("end cursor = %d, want last", m.picker.cursor)
	}
	m = sendMsg(t, m, specialKey(tea.KeyHome))
	if m.picker.cursor != 0 {
		t.Errorf("home cursor = %d, want 0", m.picker.cursor)
	}
	m = sendMsg(t, m, specialKey(tea.KeyUp))
	if m.picker.cursor != 0 {
		t.Errorf("up at top = %d, want 0", m.picker.cursor)
	}
	before := m.picker.preview
	m = sendMsg(t, m, specialKey(tea.KeyTab))
	if m.picker.preview == before {
		t.Error("tab must toggle preview")
	}
	m = sendMsg(t, m, ctrlKey(tea.KeyCtrlE))
	if m.overlay != OverlayEdit {
		t.Fatalf("ctrl+e overlay = %v, want edit", m.overlay)
	}
	if m.editPath != "/agents/sisyphus/model" || m.editBuf != "k1" {
		t.Errorf("raw escape = %q %q", m.editPath, m.editBuf)
	}
	m = sendKeys(t, m, "esc")
	m = openAgentsPicker(t, m)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c must quit from picker")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("ctrl+c cmd = %T, want QuitMsg", cmd())
	}
}

func TestPickerQueryKeysAndClear(t *testing.T) {
	_, m := newPickerModel(pickerCandsFixture())
	m = openAgentsPicker(t, m)
	for _, k := range []string{"q", " ", "1"} {
		m = sendKeys(t, m, k)
	}
	if got := string(m.picker.query); !strings.Contains(got, "q") || !strings.Contains(got, " ") || !strings.Contains(got, "1") {
		t.Errorf("query = %q, want q/space/1 typed as query", got)
	}
	if m.overlay != OverlayPicker {
		t.Errorf("overlay = %v, want picker kept open for q/space/1", m.overlay)
	}
	m = sendMsg(t, m, ctrlKey(tea.KeyCtrlU))
	if len(m.picker.query) != 0 {
		t.Errorf("ctrl+u query = %q, want cleared", string(m.picker.query))
	}
	m = sendMsg(t, m, runeKey("ab"))
	m = sendMsg(t, m, specialKey(tea.KeyBackspace))
	if got := string(m.picker.query); got != "a" {
		t.Errorf("backspace query = %q, want a", got)
	}
}

func TestPickerNoWriteOnEscRealBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "omo.jsonc")
	content := `{"models": {"al": {"model": "acme/old-1"}}, "agents": {"s1": {"model": "al"}}}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ed, err := editor.Load(path)
	if err != nil {
		t.Fatalf("editor.Load: %v", err)
	}
	ws := &fakeWorkspace{sources: []workspace.Source{{
		ID: workspace.IDUser, Path: ed.Path(), Kind: workspace.KindUser,
		Schema: workspace.SchemaOmo, Writable: true, Session: ed,
	}}}
	m := New(ws)
	m.width = 100
	m.height = 30
	m.storePath = filepath.Join(dir, "models-store.json")
	m.modelsPath = filepath.Join(dir, "models.json")
	writeFixture(t, m.storePath, `{"acme": {"models": [{"id": "new-9", "provider": "acme"}]}}`)
	writeFixture(t, m.modelsPath, `{"providers": {}}`)
	m.refresh()
	for i, row := range m.flat {
		if row.label == "Agents" {
			m.secIdx = i
			break
		}
	}
	m.entryIdx = 0
	m.focus = PaneEntries
	m.refresh()
	m = sendKeys(t, m, "enter")
	m.detailLn = 0
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}
	m = sendKeys(t, m, "e")
	if m.overlay != OverlayPicker {
		t.Fatalf("overlay = %v, want picker", m.overlay)
	}
	m = sendMsg(t, m, runeKey("new"))
	m = sendKeys(t, m, "esc")
	if m.overlay != OverlayNone {
		t.Errorf("overlay = %v, want none", m.overlay)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("esc mutated bytes:\nbefore %q\nafter  %q", before, after)
	}
	if len(ed.DirtyPaths()) != 0 {
		t.Errorf("dirty = %v, want none after esc", ed.DirtyPaths())
	}
}

func TestPickerLongValueMiddleTruncated(t *testing.T) {
	longVal := strings.Repeat("model-id-", 40)
	if len([]rune(longVal)) <= 255 {
		t.Fatalf("longVal runes = %d, want >255", len([]rune(longVal)))
	}
	_, m := newPickerModel([]workspace.Candidate{
		{Value: longVal, Group: fuzzy.GroupRef, Provider: "acme"},
		{Value: "beta/model-x", Group: fuzzy.GroupRef, Provider: "beta"},
	})
	m.width = 80
	m.height = 24
	m.refresh()
	m = openAgentsPicker(t, m)
	view := m.View()
	for i, l := range strings.Split(view, "\n") {
		if w := lipgloss.Width(l); w > 80 {
			t.Errorf("line %d width %d exceeds 80: %q", i, w, l)
		}
	}
	if strings.Contains(view, longVal) {
		t.Error("view must not contain the full 300-rune value")
	}
	if !strings.Contains(view, "…") {
		t.Errorf("truncated rows must carry … marker:\n%s", view)
	}
}

func TestPickerAsciiZeroESC(t *testing.T) {
	_, m := newPickerModel(pickerCandsFixture())
	r := lipgloss.NewRenderer(os.Stdout)
	r.SetColorProfile(termenv.Ascii)
	m.styles = StylesWithRenderer(r)
	m.width = 80
	m.height = 24
	m.refresh()
	m = openAgentsPicker(t, m)
	if strings.Contains(m.View(), "\x1b") {
		t.Error("Ascii picker at 80x24 must carry zero ESC bytes")
	}
	m.width = 120
	m.height = 40
	m.refresh()
	if strings.Contains(m.View(), "\x1b") {
		t.Error("Ascii picker at 120x40 must carry zero ESC bytes")
	}
}

func TestIsPickerPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/models/k1/model", true},
		{"/agents/sisyphus/model", true},
		{"/categories/quick/model", true},
		{"/models/k1/models", false},
		{"/models/k1/models/0", false},
		{"/models/k1/reasoning", false},
		{"/model_profile", false},
		{"/telemetry/enabled", false},
		{"/models/k1/model/extra", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := isPickerPath(tt.path); got != tt.want {
				t.Errorf("isPickerPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestMiddleTruncate(t *testing.T) {
	if got := middleTruncate("hello", 10); got != "hello" {
		t.Errorf("short = %q", got)
	}
	got := middleTruncate(strings.Repeat("a", 50), 20)
	if w := lipgloss.Width(got); w != 20 {
		t.Errorf("truncated width = %d, want 20: %q", w, got)
	}
	if !strings.Contains(got, "…") {
		t.Errorf("truncated must carry …: %q", got)
	}
}
