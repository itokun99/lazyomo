package tui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/itokun99/lazyomo/internal/editor"
	"github.com/itokun99/lazyomo/internal/mcpfile"
	"github.com/itokun99/lazyomo/internal/workspace"
)

var (
	_ workspace.Session = (*fakeEditor)(nil)
	_ ConfigSurface     = (*fakeEditor)(nil)
	_ Workspace         = (*fakeWorkspace)(nil)
	_ workspace.Session = (*stubSession)(nil)
	_ Workspace         = (*workspace.Registry)(nil)
)

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
			ID: editor.SectionTelemetry, Title: "Telemetry",
			Entries: []editor.Entry{
				{Key: "enabled", Path: "/telemetry/enabled", Kind: editor.KindBool, Value: "false"},
			},
		},
		{
			ID: editor.SectionGitMaster, Title: "Git",
			Entries: []editor.Entry{
				{Key: "commit_footer", Path: "/git_master/commit_footer", Kind: editor.KindBool, Value: "false"},
				{Key: "include_co_authored_by", Path: "/git_master/include_co_authored_by", Kind: editor.KindBool, Value: "false"},
			},
		},
		{
			ID: editor.SectionID("$schema"), Title: "$schema", ReadOnly: true,
			Entries: []editor.Entry{
				{Key: "$schema", Path: "/$schema", Kind: editor.KindScalar, Value: "https://example.invalid/omo.schema.json", ReadOnly: true},
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
	f.details["telemetry\x00enabled"] = editor.Detail{Title: "enabled", Lines: []editor.DetailLine{
		{Label: "enabled", Path: "/telemetry/enabled", Value: "false", Bool: true, Editable: true},
	}}
	f.details["git_master\x00commit_footer"] = editor.Detail{Title: "commit_footer", Lines: []editor.DetailLine{
		{Label: "commit_footer", Path: "/git_master/commit_footer", Value: "false", Bool: true, Editable: true},
	}}
	f.details["$schema\x00$schema"] = editor.Detail{Title: "$schema", Lines: []editor.DetailLine{
		{Label: "$schema", Path: "/$schema", Value: "https://example.invalid/omo.schema.json"},
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

// fakeWorkspace is the workspace harness: sources in registry order plus a
// realistic save-all over their sessions (per-file results, stale refusals
// and scripted errors, continue-on-error) and a reload that refreshes every
// session.
type fakeWorkspace struct {
	sources  []workspace.Source
	stale    map[string]bool
	saveErrs map[string]error

	saveCalls   int
	reloadCalls int
	reloadErr   error
}

// newFakeWorkspace wraps one config session as the user source.
func newFakeWorkspace(ed *fakeEditor) *fakeWorkspace {
	return &fakeWorkspace{sources: []workspace.Source{{
		ID:       workspace.IDUser,
		Path:     ed.Path(),
		Kind:     workspace.KindUser,
		Schema:   workspace.SchemaOmo,
		Writable: true,
		Session:  ed,
	}}}
}

// addSource registers one more writable source after the user source.
func (f *fakeWorkspace) addSource(id, path string, kind workspace.Kind, schema workspace.Schema, session workspace.Session) {
	f.sources = append(f.sources, workspace.Source{
		ID: id, Path: path, Kind: kind, Schema: schema, Writable: true, Session: session,
	})
}

func (f *fakeWorkspace) Sources() []workspace.Source {
	return append([]workspace.Source(nil), f.sources...)
}

func (f *fakeWorkspace) SaveAll() workspace.SaveResult {
	f.saveCalls++
	var result workspace.SaveResult
	for _, source := range f.sources {
		if !source.Writable || source.Session == nil || len(source.Session.DirtyPaths()) == 0 {
			continue
		}
		if f.stale[source.ID] {
			result.Files = append(result.Files, workspace.FileResult{
				SourceID: source.ID, Path: source.Path, Stale: true,
				Err: fmt.Errorf("saving %s: refusing stale file changed on disk since load", source.Path),
			})
			continue
		}
		if err := f.saveErrs[source.ID]; err != nil {
			result.Files = append(result.Files, workspace.FileResult{
				SourceID: source.ID, Path: source.Path,
				Err: fmt.Errorf("saving %s: %w", source.Path, err),
			})
			continue
		}
		backup, err := source.Session.Save()
		if err != nil {
			result.Files = append(result.Files, workspace.FileResult{
				SourceID: source.ID, Path: source.Path,
				Err: fmt.Errorf("saving %s: %w", source.Path, err),
			})
			continue
		}
		result.Files = append(result.Files, workspace.FileResult{
			SourceID: source.ID, Path: source.Path, Backup: backup,
		})
	}
	return result
}

func (f *fakeWorkspace) Reload() error {
	f.reloadCalls++
	if f.reloadErr != nil {
		return f.reloadErr
	}
	var errs []error
	for _, source := range f.sources {
		if source.Session == nil {
			continue
		}
		if err := source.Session.Reload(); err != nil {
			errs = append(errs, fmt.Errorf("reloading %s: %w", source.Path, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("reloading workspace: %w", errors.Join(errs...))
	}
	return nil
}

// stubSession is a bare workspace.Session for sources other than the config
// document (a stand-in for the attached mcp.json session).
type stubSession struct {
	path      string
	dirty     []string
	backup    string
	saveErr   error
	reloadErr error

	saved    bool
	reloaded bool
}

func (s *stubSession) Path() string { return s.path }

func (s *stubSession) DirtyPaths() []string { return append([]string(nil), s.dirty...) }

func (s *stubSession) Save() (string, error) {
	if s.saveErr != nil {
		return "", s.saveErr
	}
	s.saved = true
	s.dirty = nil
	return s.backup, nil
}

func (s *stubSession) Reload() error {
	if s.reloadErr != nil {
		return s.reloadErr
	}
	s.reloaded = true
	s.dirty = nil
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

func newTestModel(ws Workspace) *Model {
	m := New(ws)
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
			m := sendKeys(t, newTestModel(newFakeWorkspace(newFakeEditor())), tt.keys...)
			if m.focus != tt.focus {
				t.Errorf("focus = %v, want %v", m.focus, tt.focus)
			}
		})
	}
}

func TestNumberJumpSelectsSection(t *testing.T) {
	// The harness mirrors the real surface: five jumpable sections plus
	// Git plus one read-only section, reached via [/] and j/k.
	m := newTestModel(newFakeWorkspace(newFakeEditor()))
	if len(m.sections) != 7 {
		t.Fatalf("sections = %d, want 7 (5 + Git + read-only)", len(m.sections))
	}
	if m.sections[5].ID != editor.SectionGitMaster || m.sections[5].ReadOnly {
		t.Errorf("sections[5] = %+v, want editable Git", m.sections[5])
	}
	if !m.sections[6].ReadOnly {
		t.Errorf("sections[6] = %+v, want read-only", m.sections[6])
	}
	tests := []struct {
		key string
		idx int
	}{
		{"1", 0}, {"2", 1}, {"3", 2}, {"4", 3}, {"5", 4},
	}
	for _, tt := range tests {
		t.Run("key "+tt.key, func(t *testing.T) {
			m := sendKeys(t, newTestModel(newFakeWorkspace(newFakeEditor())), tt.key)
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
	m := sendKeys(t, newTestModel(newFakeWorkspace(newFakeEditor())), "]")
	if m.secIdx != 1 {
		t.Errorf("secIdx = %d, want 1", m.secIdx)
	}
	m = sendKeys(t, m, "[")
	if m.secIdx != 0 {
		t.Errorf("secIdx = %d, want 0", m.secIdx)
	}
	m = sendKeys(t, m, "[")
	if m.secIdx != 6 {
		t.Errorf("wrapped secIdx = %d, want 6", m.secIdx)
	}
	m = sendKeys(t, m, "]")
	if m.secIdx != 0 {
		t.Errorf("wrapped secIdx = %d, want 0", m.secIdx)
	}
}

func TestMoveBounds(t *testing.T) {
	m := newTestModel(newFakeWorkspace(newFakeEditor()))
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
	m := newTestModel(newFakeWorkspace(f))
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
	m := newTestModel(newFakeWorkspace(f))
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
	m := newTestModel(newFakeWorkspace(f))
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
	m := newTestModel(newFakeWorkspace(newFakeEditor()))
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
	m := newTestModel(newFakeWorkspace(f))
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
	m := newTestModel(newFakeWorkspace(f))
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
	m := newTestModel(newFakeWorkspace(f))
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
	m := newTestModel(newFakeWorkspace(f))
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
	m := newTestModel(newFakeWorkspace(f))
	m.focus = PaneEntries
	m = sendKeys(t, m, "a", "esc")
	if m.overlay != OverlayNone || len(f.adds) != 0 {
		t.Errorf("overlay = %v adds = %v", m.overlay, f.adds)
	}
}

func TestDeleteFlow(t *testing.T) {
	f := newFakeEditor()
	m := newTestModel(newFakeWorkspace(f))
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
	m := newTestModel(newFakeWorkspace(f))
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
	m := sendKeys(t, newTestModel(newFakeWorkspace(f)), "s")
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
	m := sendKeys(t, newTestModel(newFakeWorkspace(f)), "s")
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
	m := sendKeys(t, newTestModel(newFakeWorkspace(f)), "s", "enter")
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
	m := sendKeys(t, newTestModel(newFakeWorkspace(f)), "r")
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
	m := sendKeys(t, newTestModel(newFakeWorkspace(f)), "r")
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
	m := newTestModel(newFakeWorkspace(newFakeEditor()))
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
	m := sendKeys(t, newTestModel(newFakeWorkspace(f)), "q")
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
	m := newTestModel(newFakeWorkspace(f))
	// "[" from the first section wraps to the last one: the $schema
	// read-only section. Telemetry and Git stay editable toggles.
	m = sendKeys(t, m, "[", "enter")
	if m.focus != PaneEntries || m.secIdx != 6 {
		t.Fatalf("read-only section not selected: %v %d", m.focus, m.secIdx)
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
	m := newTestModel(newFakeWorkspace(f))
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
	m := newTestModel(newFakeWorkspace(newFakeEditor()))
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
	m := newTestModel(newFakeWorkspace(newFakeEditor()))
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
	// Telemetry is still the fifth section; Git follows it.
	m = sendKeys(t, m, "5")
	if m.secIdx != 4 {
		t.Fatalf("secIdx = %d, want 4 (Telemetry jump)", m.secIdx)
	}
	m = sendKeys(t, m, "]")
	if m.secIdx != 5 || m.sections[5].ID != editor.SectionGitMaster {
		t.Fatalf("secIdx = %d (%q), want 5 (Git)", m.secIdx, m.sections[m.secIdx].ID)
	}
	if m.filterMode || m.filter != "" {
		t.Fatalf("jump key leaked into filter: mode=%v filter=%q", m.filterMode, m.filter)
	}
	if m.focus != PaneEntries {
		t.Fatalf("focus = %v, want entries after jump", m.focus)
	}
	// Esc on an empty filter in filter mode also exits.
	m2 := newTestModel(newFakeWorkspace(newFakeEditor()))
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
	m := sendKeys(t, newTestModel(newFakeWorkspace(newFakeEditor())), "?")
	m = sendKeys(t, m, "esc")
	if m.overlay != OverlayNone {
		t.Errorf("help esc: overlay = %v, want none", m.overlay)
	}
	// Confirm (save) closes on esc.
	f := newFakeEditor()
	f.dirty = []string{"/models/k1"}
	m = sendKeys(t, newTestModel(newFakeWorkspace(f)), "s", "esc")
	if m.overlay != OverlayNone {
		t.Errorf("confirm esc: overlay = %v, want none", m.overlay)
	}
	// Edit closes on esc.
	m = newTestModel(newFakeWorkspace(newFakeEditor()))
	m.focus = PaneEntries
	m = sendKeys(t, m, "e", "esc")
	if m.overlay != OverlayNone {
		t.Errorf("edit esc: overlay = %v, want none", m.overlay)
	}
	// Add closes on esc.
	m = newTestModel(newFakeWorkspace(newFakeEditor()))
	m.focus = PaneEntries
	m = sendKeys(t, m, "a", "esc")
	if m.overlay != OverlayNone {
		t.Errorf("add esc: overlay = %v, want none", m.overlay)
	}
	// Delete confirm closes on esc.
	m = newTestModel(newFakeWorkspace(newFakeEditor()))
	m.focus = PaneEntries
	m = sendKeys(t, m, "d", "esc")
	if m.overlay != OverlayNone {
		t.Errorf("delete esc: overlay = %v, want none", m.overlay)
	}
	// Error popup closes on esc.
	fe := newFakeEditor()
	fe.dirty = []string{"/models/k1"}
	fe.saveErr = errSave("disk full")
	m = sendKeys(t, newTestModel(newFakeWorkspace(fe)), "s", "enter")
	if m.overlay != OverlayError {
		t.Fatalf("expected error popup, got %v", m.overlay)
	}
	m = sendKeys(t, m, "esc")
	if m.overlay != OverlayNone {
		t.Errorf("error esc: overlay = %v, want none", m.overlay)
	}
}

func TestHelpOverlay(t *testing.T) {
	m := sendKeys(t, newTestModel(newFakeWorkspace(newFakeEditor())), "?")
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
	m := newTestModel(newFakeWorkspace(f))
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

	m2 := newTestModel(newFakeWorkspace(f))
	m2.focus = PaneEntries
	m2.entryIdx = 1
	m2.refresh()
	m2 = sendKeys(t, m2, "enter", " ")
	if len(f.toggles) == 0 {
		t.Error("expected detail bool toggle")
	}
}

func TestTerminalTooSmall(t *testing.T) {
	m := newTestModel(newFakeWorkspace(newFakeEditor()))
	m.width = 40
	m.height = 10
	if !strings.Contains(m.View(), "Terminal too small") {
		t.Error("expected resize message")
	}
}

func TestWindowSizeMsg(t *testing.T) {
	m := New(newFakeWorkspace(newFakeEditor()))
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
	m := newTestModel(newFakeWorkspace(newFakeEditor()))
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit command on ctrl+c")
	}
}

func TestDirtyCountAggregatesSources(t *testing.T) {
	f := newFakeEditor()
	f.dirty = []string{"/models/k1", "/models/b1"}
	ws := newFakeWorkspace(f)
	mcp := &stubSession{path: "/tmp/agent/mcp.json", dirty: []string{"/mcpServers/fetch"}}
	ws.addSource(workspace.IDMCP, mcp.path, workspace.KindExternal, workspace.SchemaMCPServers, mcp)
	m := newTestModel(ws)
	if got := m.dirtyCount(); got != 3 {
		t.Fatalf("dirtyCount() = %d, want 3", got)
	}
	if view := m.View(); !strings.Contains(view, "dirty (3)") {
		t.Errorf("view missing aggregate dirty count:\n%s", view)
	}
	m = sendKeys(t, m, "s")
	if !strings.Contains(m.confirmText, "Save 3 changes?") {
		t.Errorf("confirmText = %q, want all three changes", m.confirmText)
	}
}

func TestSaveAllReportsEveryFile(t *testing.T) {
	f := newFakeEditor()
	f.dirty = []string{"/models/k1"}
	ws := newFakeWorkspace(f)
	mcp := &stubSession{path: "/tmp/agent/mcp.json", dirty: []string{"/mcpServers/fetch"}, backup: "/tmp/agent/mcp.json.bak.1"}
	ws.addSource(workspace.IDMCP, mcp.path, workspace.KindExternal, workspace.SchemaMCPServers, mcp)
	m := newTestModel(ws)
	m = sendKeys(t, m, "s", "enter")
	if ws.saveCalls != 1 {
		t.Fatalf("saveCalls = %d, want 1", ws.saveCalls)
	}
	if !f.saved || !mcp.saved {
		t.Fatalf("saved flags: user=%v mcp=%v, want both saved", f.saved, mcp.saved)
	}
	if len(f.DirtyPaths()) != 0 || len(mcp.DirtyPaths()) != 0 {
		t.Errorf("dirty after save: %v %v, want none", f.DirtyPaths(), mcp.DirtyPaths())
	}
	if got := m.dirtyCount(); got != 0 {
		t.Errorf("dirtyCount() = %d, want 0", got)
	}
	for _, want := range []string{"saved 2 files", f.backup, mcp.backup} {
		if !strings.Contains(m.status, want) {
			t.Errorf("status = %q, want it to contain %q", m.status, want)
		}
	}
}

func TestStaleSaveSurfacesError(t *testing.T) {
	f := newFakeEditor()
	f.dirty = []string{"/models/k1"}
	ws := newFakeWorkspace(f)
	mcp := &stubSession{path: "/tmp/agent/mcp.json", dirty: []string{"/mcpServers/fetch"}}
	ws.addSource(workspace.IDMCP, mcp.path, workspace.KindExternal, workspace.SchemaMCPServers, mcp)
	ws.stale = map[string]bool{workspace.IDMCP: true}
	m := newTestModel(ws)
	m = sendKeys(t, m, "s", "enter")
	if m.overlay != OverlayError {
		t.Fatalf("overlay = %v, want error overlay", m.overlay)
	}
	if !strings.Contains(m.errorText, "stale") || !strings.Contains(m.errorText, mcp.path) {
		t.Errorf("errorText = %q, want the stale path", m.errorText)
	}
	if !strings.Contains(m.View(), "stale") {
		t.Errorf("view does not surface the stale error:\n%s", m.View())
	}
	if len(mcp.DirtyPaths()) == 0 {
		t.Error("stale refusal must keep the unsaved edits")
	}
	if !f.saved {
		t.Error("the sibling file must still save")
	}
}

func TestSavePartialFailureKeepsFailedDirty(t *testing.T) {
	f := newFakeEditor()
	f.dirty = []string{"/models/k1"}
	ws := newFakeWorkspace(f)
	ws.saveErrs = map[string]error{workspace.IDUser: errSave("disk full")}
	mcp := &stubSession{path: "/tmp/agent/mcp.json", dirty: []string{"/mcpServers/fetch"}, backup: "/tmp/agent/mcp.json.bak.1"}
	ws.addSource(workspace.IDMCP, mcp.path, workspace.KindExternal, workspace.SchemaMCPServers, mcp)
	m := newTestModel(ws)
	m = sendKeys(t, m, "s", "enter")
	if m.overlay != OverlayError || !strings.Contains(m.errorText, "disk full") {
		t.Fatalf("overlay = %v errorText = %q, want a disk-full error", m.overlay, m.errorText)
	}
	if len(f.DirtyPaths()) == 0 {
		t.Error("failed file must keep its dirty paths")
	}
	if !mcp.saved {
		t.Error("the sibling file must still save")
	}
}

func TestReloadErrorOpensPopup(t *testing.T) {
	ws := newFakeWorkspace(newFakeEditor())
	ws.reloadErr = errSave("reload boom")
	m := sendKeys(t, newTestModel(ws), "r")
	if m.overlay != OverlayError || !strings.Contains(m.errorText, "reload boom") {
		t.Fatalf("overlay = %v errorText = %q", m.overlay, m.errorText)
	}
}

// writeFixture writes one synthetic config file, creating its directory.
func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// realRegistryFixture builds the real workspace registry over a synthetic
// home (user omo.jsonc + mcp.json) and attaches the mcpfile session at the
// seam main.go wires, returning the registry, the mcp session, and both
// paths.
func realRegistryFixture(t *testing.T) (*workspace.Registry, *mcpfile.Session, string, string) {
	t.Helper()
	home := t.TempDir()
	agentDir := filepath.Join(home, ".omo", "agent")
	userPath := filepath.Join(home, ".omo", "omo.jsonc")
	mcpPath := filepath.Join(agentDir, "mcp.json")
	cwd := filepath.Join(home, "work")
	writeFixture(t, userPath, `{"models":{"k3":{"model":"acme/code-large"}},"telemetry":{"enabled":false}}`)
	writeFixture(t, mcpPath, `{"mcpServers":{"fetch":{"type":"stdio","command":"npx"}}}`)
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", cwd, err)
	}
	reg := workspace.NewRegistry(workspace.RegistryConfig{HomeDir: home, Cwd: cwd, AgentDir: agentDir})
	if _, ok := reg.Source(workspace.IDUser); !ok {
		t.Fatalf("user source missing; diagnostics: %v", reg.Diagnostics())
	}
	if _, ok := reg.Source(workspace.IDMCP); !ok {
		t.Fatalf("mcp source missing; diagnostics: %v", reg.Diagnostics())
	}
	session, err := mcpfile.Load(mcpPath)
	if err != nil {
		t.Fatalf("mcpfile.Load() error = %v", err)
	}
	if err := reg.AttachSession(workspace.IDMCP, session); err != nil {
		t.Fatalf("AttachSession() error = %v", err)
	}
	return reg, session, userPath, mcpPath
}

func TestRealRegistrySaveFlow(t *testing.T) {
	reg, _, userPath, _ := realRegistryFixture(t)
	m := newTestModel(reg)
	if got := m.activePath(); got != userPath {
		t.Errorf("activePath() = %q, want %q", got, userPath)
	}
	for _, want := range []string{"Models", "Telemetry"} {
		if !strings.Contains(m.View(), want) {
			t.Errorf("view missing %q:\n%s", want, m.View())
		}
	}
	m = sendKeys(t, m, "5", " ")
	if got := m.dirtyCount(); got != 1 {
		t.Fatalf("dirty count = %d, want 1", got)
	}
	m = sendKeys(t, m, "s", "enter")
	if m.overlay != OverlayNone {
		t.Fatalf("overlay = %v, errorText = %q", m.overlay, m.errorText)
	}
	if !strings.Contains(m.status, "saved + backup") {
		t.Errorf("status = %q, want the backup report", m.status)
	}
	if got := m.dirtyCount(); got != 0 {
		t.Errorf("dirty count after save = %d, want 0", got)
	}
	raw, err := os.ReadFile(userPath)
	if err != nil {
		t.Fatalf("reading saved config: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("saved config is not JSON: %v\n%s", err, raw)
	}
	telemetry, _ := doc["telemetry"].(map[string]any)
	if enabled, _ := telemetry["enabled"].(bool); !enabled {
		t.Errorf("telemetry.enabled = %v, want true\n%s", telemetry["enabled"], raw)
	}
	backups, err := filepath.Glob(userPath + ".bak.*")
	if err != nil {
		t.Fatalf("globbing backups: %v", err)
	}
	if len(backups) != 1 {
		t.Errorf("backup count = %d (%v), want 1", len(backups), backups)
	}
}

func TestRealRegistryStaleRefusal(t *testing.T) {
	reg, session, _, mcpPath := realRegistryFixture(t)
	if err := session.AddServer("late", "stdio"); err != nil {
		t.Fatalf("AddServer() error = %v", err)
	}
	if err := session.SetField("late", "command", "npx"); err != nil {
		t.Fatalf("SetField() error = %v", err)
	}
	tampered := []byte("{\"mcpServers\":{\"fetch\":{\"type\":\"stdio\",\"command\":\"npx\"}},\"settings\":{}}\n")
	if err := os.WriteFile(mcpPath, tampered, 0o644); err != nil {
		t.Fatalf("tampering mcp.json: %v", err)
	}
	m := newTestModel(reg)
	if got := m.dirtyCount(); got == 0 {
		t.Fatal("expected the edited mcp session to be dirty")
	}
	m = sendKeys(t, m, "s", "enter")
	if m.overlay != OverlayError {
		t.Fatalf("overlay = %v, want error overlay", m.overlay)
	}
	if !strings.Contains(m.errorText, "stale") || !strings.Contains(m.errorText, mcpPath) {
		t.Errorf("errorText = %q, want the stale path", m.errorText)
	}
	if !strings.Contains(m.View(), "stale") {
		t.Errorf("view does not surface the stale error:\n%s", m.View())
	}
	if len(session.DirtyPaths()) == 0 {
		t.Error("stale refusal must keep the unsaved edits")
	}
	after, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("reading tampered mcp.json: %v", err)
	}
	if !bytes.Equal(after, tampered) {
		t.Errorf("refused save touched the file:\ngot  %q\nwant %q", after, tampered)
	}
}
