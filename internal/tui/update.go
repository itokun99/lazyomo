package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/itokun99/lazyomo/internal/editor"
	"github.com/itokun99/lazyomo/internal/workspace"
)

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.refresh()
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if m.overlay != OverlayNone {
		return m.handleOverlayKey(msg)
	}
	if m.filterMode {
		return m.handleFilterKey(msg)
	}
	return m.handleNormalKey(msg)
}

func (m *Model) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "1", "2", "3", "4", "5":
		n := int(msg.String()[0] - '1')
		if n < len(m.sections) {
			m.secIdx = n
			m.entryIdx = 0
			m.focus = PaneEntries
			m.refresh()
		}
		return m, nil
	case "0":
		if m.focus == PaneEntries {
			m.focus = PaneDetail
		} else {
			m.focus = PaneEntries
		}
		return m, nil
	case "j", "down":
		m.moveDown()
		return m, nil
	case "k", "up":
		m.moveUp()
		return m, nil
	case "h", "left":
		if m.focus == PaneEntries {
			m.focus = PaneSections
		} else if m.focus == PaneDetail {
			m.focus = PaneEntries
		}
		return m, nil
	case "l", "right":
		if m.focus == PaneSections {
			m.focus = PaneEntries
		} else if m.focus == PaneEntries {
			m.focus = PaneDetail
		}
		return m, nil
	case "tab":
		m.focus = (m.focus + 1) % 3
		return m, nil
	case "shift+tab":
		m.focus = (m.focus + 2) % 3
		return m, nil
	case "[":
		if len(m.sections) > 0 {
			m.secIdx = (m.secIdx + len(m.sections) - 1) % len(m.sections)
			m.entryIdx = 0
			m.refresh()
		}
		return m, nil
	case "]":
		if len(m.sections) > 0 {
			m.secIdx = (m.secIdx + 1) % len(m.sections)
			m.entryIdx = 0
			m.refresh()
		}
		return m, nil
	case "enter":
		if m.focus == PaneSections {
			m.focus = PaneEntries
		} else if m.focus == PaneEntries {
			m.focus = PaneDetail
		}
		return m, nil
	case "esc":
		if m.focus == PaneDetail {
			m.focus = PaneEntries
		} else if m.focus == PaneEntries {
			m.focus = PaneSections
		}
		return m, nil
	case "e":
		m.openEdit()
		return m, nil
	case "a":
		m.openAdd()
		return m, nil
	case "d":
		m.openDelete()
		return m, nil
	case " ":
		m.toggle()
		return m, nil
	case "s":
		m.save()
		return m, nil
	case "r":
		m.reload()
		return m, nil
	case "?":
		m.overlay = OverlayHelp
		m.helpScroll = 0
		return m, nil
	case "q":
		if m.dirtyCount() > 0 {
			m.overlay = OverlayConfirm
			m.confirmKind = confirmQuit
			m.confirmText = "Quit without saving?"
		} else {
			return m, tea.Quit
		}
		return m, nil
	case "/":
		if m.focus == PaneEntries {
			m.filterMode = true
			m.refresh()
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) moveDown() {
	switch m.focus {
	case PaneSections:
		if m.secIdx < len(m.sections)-1 {
			m.secIdx++
			m.entryIdx = 0
			m.refresh()
		}
	case PaneEntries:
		if m.entryIdx < len(m.entries)-1 {
			m.entryIdx++
			m.refresh()
		}
	case PaneDetail:
		if m.detailLn < len(m.detail.Lines)-1 {
			m.detailLn++
			m.detailOff = adjustOffset(m.detailOff, m.detailLn, m.detailViewport())
		}
	}
}

func (m *Model) moveUp() {
	switch m.focus {
	case PaneSections:
		if m.secIdx > 0 {
			m.secIdx--
			m.entryIdx = 0
			m.refresh()
		}
	case PaneEntries:
		if m.entryIdx > 0 {
			m.entryIdx--
			m.refresh()
		}
	case PaneDetail:
		if m.detailLn > 0 {
			m.detailLn--
			m.detailOff = adjustOffset(m.detailOff, m.detailLn, m.detailViewport())
		}
	}
}

func (m *Model) readOnlySelected() bool {
	if sec := m.curSection(); sec != nil && sec.ReadOnly {
		return true
	}
	if e := m.selEntry(); e != nil && e.ReadOnly {
		return true
	}
	return false
}

func (m *Model) openEdit() {
	if m.focus != PaneEntries && m.focus != PaneDetail {
		return
	}
	if m.readOnlySelected() {
		m.status = "read-only section"
		return
	}
	if m.focus == PaneEntries {
		e := m.selEntry()
		if e == nil {
			return
		}
		switch e.Kind {
		case editor.KindScalar:
			m.overlay = OverlayEdit
			m.editPath = e.Path
			m.editTitle = "Edit " + e.Path
			m.editBuf = e.Value
			m.editErr = ""
		case editor.KindBool:
			m.status = "use space to toggle bools"
		default:
			m.status = "select a detail row to edit"
		}
		return
	}
	l := m.selLine()
	if l == nil {
		return
	}
	if !l.Editable {
		m.status = "not editable"
		return
	}
	if l.Bool {
		m.status = "use space to toggle bools"
		return
	}
	m.overlay = OverlayEdit
	m.editPath = l.Path
	m.editTitle = "Edit " + l.Path
	m.editBuf = l.Value
	m.editErr = ""
}

func (m *Model) openAdd() {
	if m.focus != PaneEntries {
		return
	}
	sec := m.curSection()
	if sec == nil {
		return
	}
	if sec.ReadOnly || m.readOnlySelected() {
		m.status = "read-only section"
		return
	}
	m.overlay = OverlayAdd
	m.addBuf = ""
	m.addErr = ""
}

func (m *Model) openDelete() {
	if m.focus != PaneEntries {
		return
	}
	sec := m.curSection()
	e := m.selEntry()
	if sec == nil || e == nil {
		return
	}
	if sec.ReadOnly || e.ReadOnly {
		m.status = "read-only section"
		return
	}
	m.overlay = OverlayConfirm
	m.confirmKind = confirmDelete
	m.confirmText = "Delete " + e.Path + "?"
	m.delSec = sec.ID
	m.delKey = e.Key
	m.delPath = e.Path
}

func (m *Model) toggle() {
	if m.focus != PaneEntries && m.focus != PaneDetail {
		return
	}
	if m.readOnlySelected() {
		m.status = "read-only section"
		return
	}
	if m.focus == PaneEntries {
		e := m.selEntry()
		if e == nil {
			return
		}
		if e.Kind != editor.KindBool {
			m.status = "not a bool"
			return
		}
		if err := m.config.ToggleBool(e.Path); err != nil {
			m.status = "toggle failed: " + err.Error()
			return
		}
		m.status = "toggled " + e.Path
		m.refresh()
		return
	}
	l := m.selLine()
	if l == nil {
		return
	}
	if !l.Bool {
		m.status = "not a bool"
		return
	}
	if !l.Editable {
		m.status = "not editable"
		return
	}
	if err := m.config.ToggleBool(l.Path); err != nil {
		m.status = "toggle failed: " + err.Error()
		return
	}
	m.status = "toggled " + l.Path
	m.refresh()
}

func (m *Model) save() {
	n := m.dirtyCount()
	if n == 0 {
		m.status = "already saved"
		return
	}
	m.overlay = OverlayConfirm
	m.confirmKind = confirmSave
	m.confirmText = fmt.Sprintf("Save %d changes?", n)
}

func (m *Model) reload() {
	n := m.dirtyCount()
	if n == 0 {
		if err := m.ws.Reload(); err != nil {
			m.overlay = OverlayError
			m.errorText = "reload failed: " + err.Error()
			return
		}
		m.entryIdx = 0
		m.refresh()
		m.status = "reloaded"
		return
	}
	m.overlay = OverlayConfirm
	m.confirmKind = confirmReload
	m.confirmText = fmt.Sprintf("Reload and lose %d changes?", n)
}

func (m *Model) handleOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.overlay {
	case OverlayHelp:
		switch msg.String() {
		case "?", "esc":
			m.overlay = OverlayNone
		case "j", "down":
			m.helpScroll++
		case "k", "up":
			if m.helpScroll > 0 {
				m.helpScroll--
			}
		}
		return m, nil
	case OverlayError:
		switch msg.String() {
		case "enter", "esc":
			m.overlay = OverlayNone
		}
		return m, nil
	case OverlayConfirm:
		switch msg.String() {
		case "enter":
			return m.confirm()
		case "esc":
			m.overlay = OverlayNone
			m.confirmKind = confirmNone
		}
		return m, nil
	case OverlayEdit:
		switch msg.Type {
		case tea.KeyEsc:
			m.overlay = OverlayNone
			m.editErr = ""
		case tea.KeyEnter:
			m.submitEdit()
		case tea.KeyBackspace:
			if len(m.editBuf) > 0 {
				m.editBuf = m.editBuf[:len(m.editBuf)-1]
			}
		case tea.KeyRunes:
			m.editBuf += string(msg.Runes)
		}
		return m, nil
	case OverlayAdd:
		switch msg.Type {
		case tea.KeyEsc:
			m.overlay = OverlayNone
			m.addErr = ""
		case tea.KeyEnter:
			m.submitAdd()
		case tea.KeyBackspace:
			if len(m.addBuf) > 0 {
				m.addBuf = m.addBuf[:len(m.addBuf)-1]
			}
		case tea.KeyRunes:
			m.addBuf += string(msg.Runes)
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) confirm() (tea.Model, tea.Cmd) {
	kind := m.confirmKind
	m.overlay = OverlayNone
	m.confirmKind = confirmNone
	switch kind {
	case confirmSave:
		result := m.ws.SaveAll()
		m.refresh()
		if failed := saveFailure(result); failed != "" {
			m.overlay = OverlayError
			m.errorText = "save failed: " + failed
			return m, nil
		}
		m.status = saveStatus(result)
	case confirmReload:
		if err := m.ws.Reload(); err != nil {
			m.overlay = OverlayError
			m.errorText = "reload failed: " + err.Error()
			return m, nil
		}
		m.entryIdx = 0
		m.filter = ""
		m.refresh()
		m.status = "reloaded"
	case confirmQuit:
		return m, tea.Quit
	case confirmDelete:
		if err := m.config.RemoveEntry(m.delSec, m.delKey); err != nil {
			m.status = "delete failed: " + err.Error()
			return m, nil
		}
		m.status = "deleted " + m.delPath
		m.refresh()
	}
	return m, nil
}

func (m *Model) submitEdit() {
	if strings.TrimSpace(m.editBuf) == "" {
		m.editErr = "value must not be empty"
		return
	}
	if err := m.config.SetScalar(m.editPath, m.editBuf); err != nil {
		m.editErr = err.Error()
		return
	}
	m.overlay = OverlayNone
	m.editErr = ""
	m.status = "set " + m.editPath
	m.refresh()
}

func (m *Model) submitAdd() {
	sec := m.curSection()
	if sec == nil {
		m.overlay = OverlayNone
		return
	}
	if strings.TrimSpace(m.addBuf) == "" {
		m.addErr = "name must not be empty"
		return
	}
	if err := m.config.AddEntry(sec.ID, m.addBuf); err != nil {
		m.addErr = err.Error()
		return
	}
	key := m.addBuf
	m.overlay = OverlayNone
	m.addErr = ""
	m.status = "added /" + string(sec.ID) + "/" + key
	m.refresh()
	for i, e := range m.entries {
		if e.Key == key {
			m.entryIdx = i
			break
		}
	}
	m.refresh()
}

func (m *Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		// esc clears the filter AND exits filter input mode, returning
		// normal key handling (matches the "esc clear/close" hint).
		m.filter = ""
		m.filterMode = false
		m.refresh()
	case tea.KeyEnter:
		m.filterMode = false
		m.refresh()
	case tea.KeyBackspace:
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
			m.entryIdx = 0
			m.refresh()
		}
	case tea.KeyRunes:
		m.filter += string(msg.Runes)
		m.entryIdx = 0
		m.refresh()
	}
	return m, nil
}

// saveFailure summarizes the failed files of a workspace save: stale
// refusals and save errors, one entry each. It returns "" when every
// attempted file saved cleanly.
func saveFailure(result workspace.SaveResult) string {
	parts := make([]string, 0, len(result.Files))
	for _, file := range result.Files {
		switch {
		case file.Stale:
			parts = append(parts, fmt.Sprintf("%s is stale (changed on disk since load); unsaved edits kept", file.Path))
		case file.Err != nil:
			parts = append(parts, file.Err.Error())
		}
	}
	return strings.Join(parts, "; ")
}

// saveStatus summarizes a clean workspace save: the backup path of a single
// file, or the file count plus every backup for several files.
func saveStatus(result workspace.SaveResult) string {
	backups := make([]string, 0, len(result.Files))
	for _, file := range result.Files {
		if file.Backup != "" {
			backups = append(backups, file.Backup)
		}
	}
	switch {
	case len(result.Files) == 1 && len(backups) == 1:
		return "saved + backup " + backups[0]
	case len(result.Files) == 1:
		return "saved"
	case len(backups) > 0:
		return fmt.Sprintf("saved %d files (backups: %s)", len(result.Files), strings.Join(backups, ", "))
	default:
		return fmt.Sprintf("saved %d files", len(result.Files))
	}
}
