package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/itokun99/lazyomo/internal/editor"
)

var _ Editor = (*fakeEditor)(nil)

type fakeEditor struct {
	path     string
	sections []editor.Section
	details  map[string]editor.Detail
	dirty    []string

	setErr    error
	toggleErr error
	addErr    error
	removeErr error
	saveErr   error
	reloadErr error

	sets    [][2]string
	toggles []string
	adds    [][2]string
	removes [][2]string

	saved    bool
	reloaded bool
	backup   string
}

func newFakeEditor() *fakeEditor {
	f := &fakeEditor{
		path:    "/tmp/omo.jsonc",
		backup:  "/tmp/omo.jsonc.bak.2026-10-05",
		details: map[string]editor.Detail{},
	}
	f.sections = []editor.Section{
		{
			ID: editor.SectionModels, Title: "Models",
			Entries: []editor.Entry{
				{Key: "k1", Path: "/models/k1", Kind: editor.KindScalar, Value: "v1"},
				{Key: "b1", Path: "/models/b1", Kind: editor.KindBool, Value: "false"},
				{Key: "ch", Path: "/models/ch", Kind: editor.KindChain, Value: "2 items"},
				{Key: "al", Path: "/models/al", Kind: editor.KindAlias, Value: "m1"},
			},
		},
		{ID: editor.SectionModelProfiles, Title: "Model Profiles"},
		{ID: editor.SectionAgents, Title: "Agents"},
		{ID: editor.SectionCategories, Title: "Categories"},
		{
			ID: editor.SectionTelemetry, Title: "Telemetry", ReadOnly: true,
			Entries: []editor.Entry{
				{Key: "enabled", Path: "/telemetry/enabled", Kind: editor.KindBool, Value: "false", ReadOnly: true},
			},
		},
	}
	f.details["models\x00k1"] = editor.Detail{Title: "models/k1", Lines: []editor.DetailLine{
		{Label: "k1", Path: "/models/k1", Value: "v1", Editable: true},
	}}
	f.details["models\x00b1"] = editor.Detail{Title: "models/b1", Lines: []editor.DetailLine{
		{Label: "b1", Path: "/models/b1", Value: "false", Bool: true, Editable: true},
	}}
	f.details["models\x00ch"] = editor.Detail{Title: "models/ch", Lines: []editor.DetailLine{
		{Label: "ch[0]", Path: "/models/ch/0", Value: "a", Editable: true},
		{Label: "ch[1]", Path: "/models/ch/1", Value: "b", Editable: true},
	}}
	f.details["models\x00al"] = editor.Detail{Title: "models/al", Lines: []editor.DetailLine{
		{Label: "al.model", Path: "/models/al/model", Value: "m1", Editable: true},
		{Label: "al.reasoning", Path: "/models/al/reasoning", Value: "high", Editable: true},
	}}
	f.details["telemetry\x00enabled"] = editor.Detail{Title: "telemetry/enabled", Lines: []editor.DetailLine{
		{Label: "enabled", Path: "/telemetry/enabled", Value: "false", Bool: true, Editable: false},
	}}
	return f
}

func (f *fakeEditor) Path() string { return f.path }

func (f *fakeEditor) Sections() []editor.Section { return f.sections }

func (f *fakeEditor) Detail(section editor.SectionID, key string) (editor.Detail, error) {
	if d, ok := f.details[string(section)+"\x00"+key]; ok {
		return d, nil
	}
	return editor.Detail{}, nil
}

func (f *fakeEditor) SetScalar(path, value string) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.sets = append(f.sets, [2]string{path, value})
	f.dirty = append(f.dirty, path)
	return nil
}

func (f *fakeEditor) ToggleBool(path string) error {
	if f.toggleErr != nil {
		return f.toggleErr
	}
	f.toggles = append(f.toggles, path)
	for k, d := range f.details {
		lines := d.Lines
		for i, l := range lines {
			if l.Path == path && l.Bool {
				if lines[i].Value == "true" {
					lines[i].Value = "false"
				} else {
					lines[i].Value = "true"
				}
			}
		}
		f.details[k] = d
	}
	for i, s := range f.sections {
		for j, e := range s.Entries {
			if e.Path == path && e.Kind == editor.KindBool {
				if f.sections[i].Entries[j].Value == "true" {
					f.sections[i].Entries[j].Value = "false"
				} else {
					f.sections[i].Entries[j].Value = "true"
				}
			}
		}
	}
	f.dirty = append(f.dirty, path)
	return nil
}

func (f *fakeEditor) AddEntry(section editor.SectionID, key string) error {
	if f.addErr != nil {
		return f.addErr
	}
	for _, s := range f.sections {
		if s.ID == section {
			for _, e := range s.Entries {
				if e.Key == key {
					return errExists(key)
				}
			}
		}
	}
	for i, s := range f.sections {
		if s.ID == section {
			p := "/" + string(section) + "/" + key
			f.sections[i].Entries = append(f.sections[i].Entries, editor.Entry{
				Key: key, Path: p, Kind: editor.KindScalar, Value: "",
			})
			f.details[string(section)+"\x00"+key] = editor.Detail{
				Title: string(section) + "/" + key,
				Lines: []editor.DetailLine{{Label: key, Path: p, Editable: true}},
			}
		}
	}
	f.adds = append(f.adds, [2]string{string(section), key})
	f.dirty = append(f.dirty, "/"+string(section)+"/"+key)
	return nil
}

func (f *fakeEditor) RemoveEntry(section editor.SectionID, key string) error {
	if f.removeErr != nil {
		return f.removeErr
	}
	for i, s := range f.sections {
		if s.ID == section {
			kept := s.Entries[:0]
			for _, e := range s.Entries {
				if e.Key != key {
					kept = append(kept, e)
				}
			}
			f.sections[i].Entries = kept
		}
	}
	delete(f.details, string(section)+"\x00"+key)
	f.removes = append(f.removes, [2]string{string(section), key})
	f.dirty = append(f.dirty, "/"+string(section))
	return nil
}

func (f *fakeEditor) DirtyPaths() []string { return f.dirty }

func (f *fakeEditor) Save() (string, error) {
	if f.saveErr != nil {
		return "", f.saveErr
	}
	f.saved = true
	f.dirty = nil
	return f.backup, nil
}

func (f *fakeEditor) Reload() error {
	if f.reloadErr != nil {
		return f.reloadErr
	}
	f.reloaded = true
	f.dirty = nil
	return nil
}

type errExists string

func (e errExists) Error() string { return "adding entry " + string(e) + ": already exists" }

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func sendKeys(t *testing.T, m *Model, keys ...string) *Model {
	t.Helper()
	for _, k := range keys {
		model, _ := m.Update(keyMsg(k))
		m = model.(*Model)
	}
	return m
}

func newTestModel(f *fakeEditor) *Model {
	m := New(f)
	m.width = 100
	m.height = 30
	m.refresh()
	return m
}

func TestFocusTransitions(t *testing.T) {
	tests := []struct {
		name  string
		keys  []string
		focus Pane
	}{
		{name: "tab to entries", keys: []string{"tab"}, focus: PaneEntries},
		{name: "tab twice to detail", keys: []string{"tab", "tab"}, focus: PaneDetail},
		{name: "tab cycles back", keys: []string{"tab", "tab", "tab"}, focus: PaneSections},
		{name: "shift+tab to detail", keys: []string{"shift+tab"}, focus: PaneDetail},
		{name: "l moves right", keys: []string{"l"}, focus: PaneEntries},
		{name: "l twice to detail", keys: []string{"l", "l"}, focus: PaneDetail},
		{name: "l stops at detail", keys: []string{"l", "l", "l"}, focus: PaneDetail},
		{name: "enter drills to entries", keys: []string{"enter"}, focus: PaneEntries},
		{name: "enter drills to detail", keys: []string{"enter", "enter"}, focus: PaneDetail},
		{name: "esc climbs to entries", keys: []string{"tab", "tab", "esc"}, focus: PaneEntries},
		{name: "esc climbs to sections", keys: []string{"tab", "esc"}, focus: PaneSections},
		{name: "zero toggles to entries", keys: []string{"0"}, focus: PaneEntries},
		{name: "h stops at sections", keys: []string{"h"}, focus: PaneSections},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := sendKeys(t, newTestModel(newFakeEditor()), tt.keys...)
			if m.focus != tt.focus {
				t.Errorf("focus = %v, want %v", m.focus, tt.focus)
			}
		})
	}
}

func TestNumberJumpSelectsSection(t *testing.T) {
	tests := []struct {
		key string
		idx int
	}{
		{"1", 0}, {"2", 1}, {"3", 2}, {"4", 3}, {"5", 4},
	}
	for _, tt := range tests {
		t.Run("key "+tt.key, func(t *testing.T) {
			m := sendKeys(t, newTestModel(newFakeEditor()), tt.key)
			if m.secIdx != tt.idx {
				t.Errorf("secIdx = %d, want %d", m.secIdx, tt.idx)
			}
			if m.focus != PaneEntries {
				t.Errorf("focus = %v, want entries", m.focus)
			}
		})
	}
}

func TestBracketCyclesSections(t *testing.T) {
	m := sendKeys(t, newTestModel(newFakeEditor()), "]")
	if m.secIdx != 1 {
		t.Errorf("secIdx = %d, want 1", m.secIdx)
	}
	m = sendKeys(t, m, "[")
	if m.secIdx != 0 {
		t.Errorf("secIdx = %d, want 0", m.secIdx)
	}
	m = sendKeys(t, m, "[")
	if m.secIdx != 4 {
		t.Errorf("wrapped secIdx = %d, want 4", m.secIdx)
	}
	m = sendKeys(t, m, "]")
	if m.secIdx != 0 {
		t.Errorf("wrapped secIdx = %d, want 0", m.secIdx)
	}
}

func TestMoveBounds(t *testing.T) {
	m := newTestModel(newFakeEditor())
	m.focus = PaneEntries
	m = sendKeys(t, m, "k", "k")
	if m.entryIdx != 0 {
		t.Errorf("entryIdx = %d, want 0", m.entryIdx)
	}
	for i := 0; i < 10; i++ {
		m = sendKeys(t, m, "j")
	}
	if m.entryIdx != len(m.entries)-1 {
		t.Errorf("entryIdx = %d, want last %d", m.entryIdx, len(m.entries)-1)
	}
}

func TestEditScalarFlow(t *testing.T) {
	f := newFakeEditor()
	m := newTestModel(f)
	m.focus = PaneEntries
	m = sendKeys(t, m, "e")
	if m.overlay != OverlayEdit {
		t.Fatalf("overlay = %v, want edit", m.overlay)
	}
	if m.editBuf != "v1" || m.editPath != "/models/k1" {
		t.Errorf("edit target = %q %q", m.editPath, m.editBuf)
	}
	m = sendKeys(t, m, "x", "enter")
	if m.overlay != OverlayNone {
		t.Errorf("overlay = %v, want none after enter", m.overlay)
	}
	if len(f.sets) != 1 || f.sets[0] != [2]string{"/models/k1", "v1x"} {
		t.Errorf("sets = %v", f.sets)
	}
	if !strings.Contains(m.status, "set /models/k1") {
		t.Errorf("status = %q", m.status)
	}
}

func TestEditCancelKeepsValue(t *testing.T) {
	f := newFakeEditor()
	m := newTestModel(f)
	m.focus = PaneEntries
	m = sendKeys(t, m, "e", "x", "esc")
	if m.overlay != OverlayNone {
		t.Errorf("overlay = %v, want none", m.overlay)
	}
	if len(f.sets) != 0 {
		t.Errorf("sets = %v, want none", f.sets)
	}
}

func TestEditEmptyShowsInlineError(t *testing.T) {
	f := newFakeEditor()
	m := newTestModel(f)
	m.focus = PaneEntries
	m = sendKeys(t, m, "e", "backspace", "backspace", "enter")
	if m.overlay != OverlayEdit {
		t.Fatalf("overlay = %v, want edit kept open", m.overlay)
	}
	if m.editErr == "" {
		t.Error("expected inline validation error")
	}
	if len(f.sets) != 0 {
		t.Errorf("sets = %v, want none", f.sets)
	}
}

func TestEditBoolHintsSpace(t *testing.T) {
	m := newTestModel(newFakeEditor())
	m.focus = PaneEntries
	m.entryIdx = 1
	m.refresh()
	m = sendKeys(t, m, "e")
	if m.overlay != OverlayNone {
		t.Errorf("overlay = %v, want none for bool row", m.overlay)
	}
	if !strings.Contains(m.status, "space") {
		t.Errorf("status = %q, want space hint", m.status)
	}
}

func TestToggleBoolFlow(t *testing.T) {
	f := newFakeEditor()
	m := newTestModel(f)
	m.focus = PaneEntries
	m.entryIdx = 1
	m.refresh()
	m = sendKeys(t, m, " ")
	if len(f.toggles) != 1 || f.toggles[0] != "/models/b1" {
		t.Errorf("toggles = %v", f.toggles)
	}
	if !strings.Contains(m.status, "toggled /models/b1") {
		t.Errorf("status = %q", m.status)
	}
	if len(f.DirtyPaths()) == 0 {
		t.Error("expected dirty paths after toggle")
	}
}

func TestToggleNonBoolFlashes(t *testing.T) {
	f := newFakeEditor()
	m := newTestModel(f)
	m.focus = PaneEntries
	m = sendKeys(t, m, " ")
	if len(f.toggles) != 0 {
		t.Errorf("toggles = %v, want none", f.toggles)
	}
	if m.status != "not a bool" {
		t.Errorf("status = %q, want not a bool", m.status)
	}
}

func TestAddFlow(t *testing.T) {
	f := newFakeEditor()
	m := newTestModel(f)
	m.focus = PaneEntries
	m = sendKeys(t, m, "a")
	if m.overlay != OverlayAdd {
		t.Fatalf("overlay = %v, want add", m.overlay)
	}
	m = sendKeys(t, m, "n", "k", "enter")
	if m.overlay != OverlayNone {
		t.Errorf("overlay = %v, want none", m.overlay)
	}
	if len(f.adds) != 1 || f.adds[0] != [2]string{"models", "nk"} {
		t.Errorf("adds = %v", f.adds)
	}
	if m.entries[m.entryIdx].Key != "nk" {
		t.Errorf("new row not selected: %+v", m.entries[m.entryIdx])
	}
}

func TestAddDuplicateStaysOpen(t *testing.T) {
	f := newFakeEditor()
	m := newTestModel(f)
	m.focus = PaneEntries
	m = sendKeys(t, m, "a", "k", "1", "enter")
	if m.overlay != OverlayAdd {
		t.Fatalf("overlay = %v, want add kept open", m.overlay)
	}
	if m.addErr == "" {
		t.Error("expected inline duplicate error")
	}
}

func TestAddCancel(t *testing.T) {
	f := newFakeEditor()
	m := newTestModel(f)
	m.focus = PaneEntries
	m = sendKeys(t, m, "a", "esc")
	if m.overlay != OverlayNone || len(f.adds) != 0 {
		t.Errorf("overlay = %v adds = %v", m.overlay, f.adds)
	}
}

func TestDeleteFlow(t *testing.T) {
	f := newFakeEditor()
	m := newTestModel(f)
	m.focus = PaneEntries
	m = sendKeys(t, m, "d")
	if m.overlay != OverlayConfirm || m.confirmKind != confirmDelete {
		t.Fatalf("overlay = %v kind = %v", m.overlay, m.confirmKind)
	}
	if !strings.Contains(m.confirmText, "/models/k1") {
		t.Errorf("confirm = %q, want full path", m.confirmText)
	}
	m = sendKeys(t, m, "enter")
	if len(f.removes) != 1 || f.removes[0] != [2]string{"models", "k1"} {
		t.Errorf("removes = %v", f.removes)
	}
	if !strings.Contains(m.status, "deleted /models/k1") {
		t.Errorf("status = %q", m.status)
	}
}

func TestDeleteCancelKeepsEntry(t *testing.T) {
	f := newFakeEditor()
	m := newTestModel(f)
	m.focus = PaneEntries
	m = sendKeys(t, m, "d", "esc")
	if len(f.removes) != 0 {
		t.Errorf("removes = %v, want none", f.removes)
	}
	if len(m.entries) != 4 {
		t.Errorf("entries = %d, want 4", len(m.entries))
	}
}

func TestSaveCleanFlashes(t *testing.T) {
	f := newFakeEditor()
	m := sendKeys(t, newTestModel(f), "s")
	if m.overlay != OverlayNone {
		t.Errorf("overlay = %v, want none when clean", m.overlay)
	}
	if m.status != "already saved" {
		t.Errorf("status = %q", m.status)
	}
	if f.saved {
		t.Error("fake should not be saved when clean")
	}
}

func TestSaveDirtyConfirmReportsBackup(t *testing.T) {
	f := newFakeEditor()
	f.dirty = []string{"/models/k1"}
	m := sendKeys(t, newTestModel(f), "s")
	if m.overlay != OverlayConfirm || m.confirmKind != confirmSave {
		t.Fatalf("overlay = %v kind = %v", m.overlay, m.confirmKind)
	}
	m = sendKeys(t, m, "esc")
	if f.saved {
		t.Error("esc should cancel save")
	}
	m = sendKeys(t, m, "s", "enter")
	if !f.saved {
		t.Error("expected save on confirm")
	}
	if !strings.Contains(m.status, "saved + backup "+f.backup) {
		t.Errorf("status = %q", m.status)
	}
}

func TestSaveErrorOpensPopup(t *testing.T) {
	f := newFakeEditor()
	f.dirty = []string{"/models/k1"}
	f.saveErr = errSave("disk full")
	m := sendKeys(t, newTestModel(f), "s", "enter")
	if m.overlay != OverlayError {
		t.Fatalf("overlay = %v, want error", m.overlay)
	}
	if !strings.Contains(m.errorText, "disk full") {
		t.Errorf("errorText = %q", m.errorText)
	}
	if len(f.DirtyPaths()) == 0 {
		t.Error("dirty marks should survive failed save")
	}
	m = sendKeys(t, m, "enter")
	if m.overlay != OverlayNone {
		t.Errorf("overlay = %v, want closed", m.overlay)
	}
}

type errSave string

func (e errSave) Error() string { return string(e) }

func TestReloadClean(t *testing.T) {
	f := newFakeEditor()
	m := sendKeys(t, newTestModel(f), "r")
	if !f.reloaded {
		t.Error("expected reload when clean")
	}
	if m.status != "reloaded" {
		t.Errorf("status = %q", m.status)
	}
}

func TestReloadDirtyConfirms(t *testing.T) {
	f := newFakeEditor()
	f.dirty = []string{"/models/k1"}
	m := sendKeys(t, newTestModel(f), "r")
	if m.overlay != OverlayConfirm || m.confirmKind != confirmReload {
		t.Fatalf("overlay = %v kind = %v", m.overlay, m.confirmKind)
	}
	m = sendKeys(t, m, "esc")
	if f.reloaded {
		t.Error("esc should cancel reload")
	}
	m = sendKeys(t, m, "r", "enter")
	if !f.reloaded {
		t.Error("expected reload on confirm")
	}
	if len(f.DirtyPaths()) != 0 {
		t.Errorf("dirty = %v, want cleared", f.DirtyPaths())
	}
}

func TestQuitClean(t *testing.T) {
	m := newTestModel(newFakeEditor())
	model, cmd := m.Update(keyMsg("q"))
	_ = model
	if cmd == nil {
		t.Fatal("expected quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("quit cmd returned %T", cmd())
	}
}

func TestQuitDirtyConfirms(t *testing.T) {
	f := newFakeEditor()
	f.dirty = []string{"/models/k1"}
	m := sendKeys(t, newTestModel(f), "q")
	if m.overlay != OverlayConfirm || m.confirmKind != confirmQuit {
		t.Fatalf("overlay = %v kind = %v", m.overlay, m.confirmKind)
	}
	if !strings.Contains(m.confirmText, "Quit without saving?") {
		t.Errorf("confirm = %q", m.confirmText)
	}
	model, cmd := m.Update(keyMsg("esc"))
	m = model.(*Model)
	if m.overlay != OverlayNone {
		t.Errorf("overlay = %v, want closed", m.overlay)
	}
	if cmd != nil {
		t.Error("esc should not quit")
	}
	model, cmd = m.Update(keyMsg("q"))
	m = model.(*Model)
	model, cmd = m.Update(keyMsg("enter"))
	_ = model
	if cmd == nil {
		t.Fatal("expected quit command on confirm")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("quit cmd returned %T", cmd())
	}
}

func TestReadOnlySectionBlocked(t *testing.T) {
	f := newFakeEditor()
	m := newTestModel(f)
	m = sendKeys(t, m, "5")
	if m.focus != PaneEntries || m.secIdx != 4 {
		t.Fatalf("telemetry not selected: %v %d", m.focus, m.secIdx)
	}
	for _, k := range []string{"e", "a", "d", " "} {
		m = sendKeys(t, m, k)
		if m.status != "read-only section" {
			t.Errorf("key %q status = %q, want read-only section", k, m.status)
		}
		if m.overlay != OverlayNone {
			t.Errorf("key %q opened overlay %v", k, m.overlay)
		}
	}
	if len(f.sets)+len(f.toggles)+len(f.adds)+len(f.removes) != 0 {
		t.Error("read-only section must not mutate")
	}
}

func TestDirtyRendering(t *testing.T) {
	f := newFakeEditor()
	f.dirty = []string{"/models/k1", "/models/b1"}
	m := newTestModel(f)
	view := m.View()
	if !strings.Contains(view, "dirty (2)") {
		t.Errorf("view missing dirty count:\n%s", view)
	}
	if !strings.Contains(view, "●") {
		t.Errorf("view missing dirty dot:\n%s", view)
	}
	if !strings.Contains(view, "clean") {
		f.dirty = nil
		m.refresh()
		if !strings.Contains(m.View(), "clean") {
			t.Errorf("clean view missing clean marker:\n%s", m.View())
		}
	}
}

func TestFilterFlow(t *testing.T) {
	m := newTestModel(newFakeEditor())
	m.focus = PaneEntries
	m = sendKeys(t, m, "/", "k")
	if !m.filterMode || m.filter != "k" {
		t.Fatalf("filter = %q mode = %v", m.filter, m.filterMode)
	}
	if len(m.entries) != 1 || m.entries[0].Key != "k1" {
		t.Errorf("filtered entries = %+v", m.entries)
	}
	m = sendKeys(t, m, "enter")
	if m.filterMode {
		t.Error("enter should leave filter mode")
	}
	if m.filter != "k" {
		t.Errorf("filter = %q, want kept", m.filter)
	}
	// esc clears the filter AND exits filter input mode in one step
	// (matches the "esc clear/close" keybar hint).
	m = sendKeys(t, m, "/", "esc")
	if m.filter != "" {
		t.Errorf("esc should clear filter, got %q", m.filter)
	}
	if m.filterMode {
		t.Error("esc should exit filter mode")
	}
}

func TestFilterEscapeExitsMode(t *testing.T) {
	m := newTestModel(newFakeEditor())
	m.focus = PaneEntries
	// Enter filter mode, type text, then Escape: must clear AND exit.
	m = sendKeys(t, m, "/", "m", "o")
	if !m.filterMode {
		t.Fatal("expected filter mode after /")
	}
	m = sendKeys(t, m, "esc")
	if m.filterMode {
		t.Fatal("esc must exit filter input mode")
	}
	if m.filter != "" {
		t.Fatalf("esc must clear filter, got %q", m.filter)
	}
	// The next section-jump key must act as a jump, not filter text.
	m = sendKeys(t, m, "5")
	if m.secIdx != 4 {
		t.Fatalf("secIdx = %d, want 4 (Telemetry jump)", m.secIdx)
	}
	if m.filterMode || m.filter != "" {
		t.Fatalf("jump key leaked into filter: mode=%v filter=%q", m.filterMode, m.filter)
	}
	if m.focus != PaneEntries {
		t.Fatalf("focus = %v, want entries after jump", m.focus)
	}
	// Esc on an empty filter in filter mode also exits.
	m2 := newTestModel(newFakeEditor())
	m2.focus = PaneEntries
	m2 = sendKeys(t, m2, "/", "esc")
	if m2.filterMode {
		t.Error("esc on empty filter should leave filter mode")
	}
	m2 = sendKeys(t, m2, "s")
	if m2.status != "already saved" {
		t.Errorf("key after esc acted as filter text, status = %q", m2.status)
	}
}

func TestOverlayEscapeClosesAll(t *testing.T) {
	// Help closes on esc.
	m := sendKeys(t, newTestModel(newFakeEditor()), "?")
	m = sendKeys(t, m, "esc")
	if m.overlay != OverlayNone {
		t.Errorf("help esc: overlay = %v, want none", m.overlay)
	}
	// Confirm (save) closes on esc.
	f := newFakeEditor()
	f.dirty = []string{"/models/k1"}
	m = sendKeys(t, newTestModel(f), "s", "esc")
	if m.overlay != OverlayNone {
		t.Errorf("confirm esc: overlay = %v, want none", m.overlay)
	}
	// Edit closes on esc.
	m = newTestModel(newFakeEditor())
	m.focus = PaneEntries
	m = sendKeys(t, m, "e", "esc")
	if m.overlay != OverlayNone {
		t.Errorf("edit esc: overlay = %v, want none", m.overlay)
	}
	// Add closes on esc.
	m = newTestModel(newFakeEditor())
	m.focus = PaneEntries
	m = sendKeys(t, m, "a", "esc")
	if m.overlay != OverlayNone {
		t.Errorf("add esc: overlay = %v, want none", m.overlay)
	}
	// Delete confirm closes on esc.
	m = newTestModel(newFakeEditor())
	m.focus = PaneEntries
	m = sendKeys(t, m, "d", "esc")
	if m.overlay != OverlayNone {
		t.Errorf("delete esc: overlay = %v, want none", m.overlay)
	}
	// Error popup closes on esc.
	fe := newFakeEditor()
	fe.dirty = []string{"/models/k1"}
	fe.saveErr = errSave("disk full")
	m = sendKeys(t, newTestModel(fe), "s", "enter")
	if m.overlay != OverlayError {
		t.Fatalf("expected error popup, got %v", m.overlay)
	}
	m = sendKeys(t, m, "esc")
	if m.overlay != OverlayNone {
		t.Errorf("error esc: overlay = %v, want none", m.overlay)
	}
}

func TestHelpOverlay(t *testing.T) {
	m := sendKeys(t, newTestModel(newFakeEditor()), "?")
	if m.overlay != OverlayHelp {
		t.Fatalf("overlay = %v, want help", m.overlay)
	}
	if !strings.Contains(m.View(), "lazyomo keys") {
		t.Error("help view missing title")
	}
	m = sendKeys(t, m, "j", "k", "?")
	if m.overlay != OverlayNone {
		t.Errorf("overlay = %v, want closed", m.overlay)
	}
}

func TestDetailEditAndToggle(t *testing.T) {
	f := newFakeEditor()
	m := newTestModel(f)
	m.focus = PaneEntries
	m.entryIdx = 3
	m.refresh()
	m = sendKeys(t, m, "enter")
	if m.focus != PaneDetail {
		t.Fatalf("focus = %v, want detail", m.focus)
	}
	if len(m.detail.Lines) != 2 {
		t.Fatalf("detail lines = %+v", m.detail.Lines)
	}
	m = sendKeys(t, m, "e")
	if m.overlay != OverlayEdit || m.editPath != "/models/al/model" {
		t.Fatalf("edit = %v %q", m.overlay, m.editPath)
	}
	m = sendKeys(t, m, "esc")

	m2 := newTestModel(f)
	m2.focus = PaneEntries
	m2.entryIdx = 1
	m2.refresh()
	m2 = sendKeys(t, m2, "enter", " ")
	if len(f.toggles) == 0 {
		t.Error("expected detail bool toggle")
	}
}

func TestTerminalTooSmall(t *testing.T) {
	m := newTestModel(newFakeEditor())
	m.width = 40
	m.height = 10
	if !strings.Contains(m.View(), "Terminal too small") {
		t.Error("expected resize message")
	}
}

func TestWindowSizeMsg(t *testing.T) {
	m := New(newFakeEditor())
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = model.(*Model)
	if m.width != 100 || m.height != 30 {
		t.Errorf("size = %dx%d", m.width, m.height)
	}
	view := m.View()
	for _, want := range []string{"Models", "Telemetry", "k1", "/tmp/omo.jsonc"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}

func TestCtrlCQuits(t *testing.T) {
	m := newTestModel(newFakeEditor())
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit command on ctrl+c")
	}
}
