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

// contextName resolves the binding-table context for the current state:
// the topmost overlay wins, then filter input, then the focused pane.
func (m *Model) contextName() string {
	if m.overlay != OverlayNone {
		switch m.overlay {
		case OverlayConfirm:
			return ctxConfirm
		case OverlayEdit:
			return ctxEdit
		case OverlayAdd:
			return ctxAdd
		case OverlayError:
			return ctxError
		case OverlayHelp:
			return ctxHelp
		case OverlayMCPAdd:
			return ctxMCPAdd
		case OverlayMCPReveal:
			return ctxMCPReveal
		case OverlayPicker:
			return ctxPicker
		}
	}
	if m.filterMode {
		return ctxFilter
	}
	if row := m.selSide(); row != nil && row.mcp && m.focus != PaneSections {
		if m.focus == PaneDetail {
			return ctxMCPDetail
		}
		return ctxMCPList
	}
	switch m.focus {
	case PaneSections:
		return ctxSections
	case PaneEntries:
		return ctxEntries
	default:
		return ctxDetail
	}
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

// handleNormalKey dispatches purely through the binding table.
func (m *Model) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if b, ok := lookupAction(tableFor(m.contextName()), msg.String()); ok {
		return m.doAction(b.Act, b.Arg)
	}
	return m, nil
}

// doAction executes one binding-table action. actQuit is the only action
// that can end the program; every other action mutates the model.
func (m *Model) doAction(act Action, arg int) (tea.Model, tea.Cmd) {
	switch act {
	case actMoveUp:
		m.moveUp()
	case actMoveDown:
		m.moveDown()
	case actFocusLeft:
		if m.focus == PaneEntries {
			m.focus = PaneSections
		} else if m.focus == PaneDetail {
			m.focus = PaneEntries
		}
	case actFocusRight:
		if m.focus == PaneSections {
			m.focus = PaneEntries
		} else if m.focus == PaneEntries {
			m.focus = PaneDetail
		}
	case actNextPane:
		m.focus = (m.focus + 1) % 3
	case actPrevPane:
		m.focus = (m.focus + 2) % 3
	case actPrevSection:
		m.cycleSection(-1)
	case actNextSection:
		m.cycleSection(1)
	case actJumpVisible:
		if arg >= 0 && arg < len(m.flat) {
			m.secIdx = arg
			m.entryIdx = 0
			m.focus = PaneEntries
			m.refresh()
		}
	case actDrillDown:
		if m.focus == PaneSections {
			m.focus = PaneEntries
		} else if m.focus == PaneEntries {
			m.focus = PaneDetail
		}
	case actClimbUp:
		if m.focus == PaneDetail {
			m.focus = PaneEntries
		} else if m.focus == PaneEntries {
			m.focus = PaneSections
		}
	case actOpenEdit:
		m.openEdit()
	case actOpenAdd:
		m.openAdd()
	case actOpenDelete:
		m.openDelete()
	case actToggle:
		m.toggle()
	case actReveal:
		m.mcpReveal()
	case actSave:
		m.save()
	case actReload:
		m.reload()
	case actHelp:
		m.overlay = OverlayHelp
		m.helpScroll = 0
	case actFilter:
		if m.focus == PaneEntries {
			m.filterMode = true
			m.refresh()
		}
	case actQuit:
		if m.dirtyCount() > 0 {
			m.overlay = OverlayConfirm
			m.confirmKind = confirmQuit
			m.confirmText = confirmLines("Quit without saving?", m.dirtyFiles())
		} else {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *Model) cycleSection(dir int) {
	if len(m.flat) == 0 {
		return
	}
	m.secIdx = (m.secIdx + dir + len(m.flat)) % len(m.flat)
	m.entryIdx = 0
	m.refresh()
}

func (m *Model) moveDown() {
	switch m.focus {
	case PaneSections:
		if m.secIdx < len(m.flat)-1 {
			m.secIdx++
			m.entryIdx = 0
			m.refresh()
		}
	case PaneEntries:
		if m.entryIdx < m.listLen()-1 {
			m.entryIdx++
			m.refresh()
		}
	case PaneDetail:
		if m.detailLn < m.detailListLen()-1 {
			m.detailLn++
			m.detailOff = adjustOffset(m.detailOff, m.detailLn, m.detailListViewport())
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
			m.detailOff = adjustOffset(m.detailOff, m.detailLn, m.detailListViewport())
		}
	}
}

// sideReadOnly reports whether the selected side row rejects edits: future
// placeholder rows always do, and the MCP servers row does while no
// editable session is bound.
func (m *Model) sideReadOnly() bool {
	row := m.selSide()
	if row == nil {
		return false
	}
	if row.mcp {
		return m.mcp == nil
	}
	return row.hasFuture
}

func (m *Model) readOnlySelected() bool {
	if m.sideReadOnly() {
		return true
	}
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
	if m.sideMCP() {
		m.mcpOpenEdit()
		return
	}
	if m.curSection() == nil || m.readOnlySelected() {
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
	if isPickerPath(l.Path) {
		m.openPicker(l.Path, l.Value)
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
	if m.sideMCP() {
		m.mcpOpenAdd()
		return
	}
	sec := m.curSection()
	if sec == nil || sec.ReadOnly || m.readOnlySelected() {
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
	if m.sideMCP() {
		m.mcpOpenDelete()
		return
	}
	sec := m.curSection()
	e := m.selEntry()
	if sec == nil || e == nil {
		m.status = "read-only section"
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
	if m.sideMCP() {
		m.mcpToggle()
		return
	}
	if m.curSection() == nil || m.readOnlySelected() {
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
	files := m.dirtyFiles()
	if len(files) == 0 {
		m.status = "already saved"
		return
	}
	m.overlay = OverlayConfirm
	m.confirmKind = confirmSave
	m.confirmText = confirmLines(fmt.Sprintf("Save %d changes?", m.dirtyCount()), files)
}

func (m *Model) reload() {
	files := m.dirtyFiles()
	if len(files) == 0 {
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
	m.confirmText = confirmLines(fmt.Sprintf("Reload and lose %d changes?", m.dirtyCount()), files)
}

// handleOverlayKey dispatches overlay keys through the binding table;
// printable runes fall through into the edit/add buffers.
func (m *Model) handleOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.overlay == OverlayPicker {
		return m.handlePickerKey(msg)
	}
	if b, ok := lookupAction(tableFor(m.contextName()), msg.String()); ok {
		switch b.Act {
		case actConfirm:
			return m.confirm()
		case actCancel:
			m.closeOverlay()
		case actSubmit:
			if m.overlay == OverlayEdit {
				m.submitEdit()
			} else if m.overlay == OverlayAdd {
				m.submitAdd()
			} else if m.overlay == OverlayMCPAdd {
				m.mcpAddSubmit()
			}
		case actBackspace:
			if m.overlay == OverlayEdit && len(m.editBuf) > 0 {
				m.editBuf = m.editBuf[:len(m.editBuf)-1]
			} else if m.overlay == OverlayAdd && len(m.addBuf) > 0 {
				m.addBuf = m.addBuf[:len(m.addBuf)-1]
			} else if m.overlay == OverlayMCPAdd && m.mcpAddStage == 0 && len(m.addBuf) > 0 {
				m.addBuf = m.addBuf[:len(m.addBuf)-1]
			}
		case actMoveUp:
			if m.overlay == OverlayMCPAdd {
				m.mcpAddMove(-1)
			}
		case actMoveDown:
			if m.overlay == OverlayMCPAdd {
				m.mcpAddMove(1)
			}
		case actHelpDown:
			m.helpScroll++
		case actHelpUp:
			if m.helpScroll > 0 {
				m.helpScroll--
			}
		}
		return m, nil
	}
	if msg.Type == tea.KeyRunes {
		switch m.overlay {
		case OverlayEdit:
			m.editBuf += string(msg.Runes)
		case OverlayAdd:
			m.addBuf += string(msg.Runes)
		case OverlayMCPAdd:
			if m.mcpAddStage == 0 {
				m.addBuf += string(msg.Runes)
			}
		}
	}
	return m, nil
}

// closeOverlay cancels the topmost overlay (P5: esc cancels topmost
// everywhere) and drops its transient errors.
func (m *Model) closeOverlay() {
	m.overlay = OverlayNone
	m.confirmKind = confirmNone
	m.editErr = ""
	m.addErr = ""
	m.editMCP = false
	m.editRename = false
	m.revealText = ""
	m.pendingMCP = ""
	m.renameFrom = ""
	m.renameTo = ""
	m.picker = nil
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
			m.status = "save failed"
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
	case confirmMCPRemove:
		if m.mcp == nil {
			m.status = "mcp.json unavailable"
			return m, nil
		}
		if err := m.mcp.RemoveServer(m.pendingMCP); err != nil {
			m.status = "delete failed: " + err.Error()
			m.refresh()
			return m, nil
		}
		m.status = "removed " + m.pendingMCP
		m.pendingMCP = ""
		m.refresh()
	case confirmRename:
		if m.mcp == nil {
			m.status = "mcp.json unavailable"
			return m, nil
		}
		result, err := m.mcp.RenameServer(m.renameFrom, m.renameTo)
		if err != nil {
			m.status = "rename failed: " + err.Error()
			m.refresh()
			return m, nil
		}
		m.status = "renamed " + result.OldName + " → " + result.NewName + "; " + result.Warning
		m.refresh()
	}
	return m, nil
}

func (m *Model) submitEdit() {
	if m.editMCP {
		if m.editRename {
			m.submitRename()
			return
		}
		m.submitMCPField()
		return
	}
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

// handleFilterKey dispatches filter keys through the binding table;
// printable runes extend the query.
func (m *Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if b, ok := lookupAction(tableFor(ctxFilter), msg.String()); ok {
		switch b.Act {
		case actFilterDone:
			m.filterMode = false
			m.refresh()
		case actFilterClear:
			m.filter = ""
			m.filterMode = false
			m.refresh()
		case actFilterBackspace:
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
				m.entryIdx = 0
				m.refresh()
			}
		}
		return m, nil
	}
	if msg.Type == tea.KeyRunes {
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
