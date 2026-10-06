package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/itokun99/lazyomo/internal/editor"
)

// View implements tea.Model.
func (m *Model) View() string {
	if m.width < MinWidth || m.height < MinHeight {
		return fmt.Sprintf(
			"Terminal too small (need %dx%d, got %dx%d)\n\nPlease resize your terminal.",
			MinWidth, MinHeight, m.width, m.height,
		)
	}
	base := m.renderBase()
	if m.overlay == OverlayNone {
		return base
	}
	box := m.renderOverlay()
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box,
		lipgloss.WithWhitespaceChars(" "))
}

func (m *Model) renderBase() string {
	contentH := m.height - 2
	leftW := m.width * 30 / 100
	if leftW < 24 {
		leftW = 24
	}
	if leftW > m.width-20 {
		leftW = m.width - 20
	}
	rightW := m.width - leftW
	entriesH := contentH * 60 / 100
	detailH := contentH - entriesH

	sections := m.renderSections(leftW, contentH)
	entries := m.renderEntries(rightW, entriesH)
	detail := m.renderDetail(rightW, detailH)
	right := lipgloss.JoinVertical(lipgloss.Left, entries, detail)
	top := lipgloss.JoinHorizontal(lipgloss.Top, sections, right)
	return top + "\n" + m.renderKeybar() + "\n" + m.renderStatus()
}

func (m *Model) paneStyle(focused bool, w, h int) lipgloss.Style {
	st := m.styles.BlurBorder
	if focused {
		st = m.styles.FocusBorder
	}
	return st.Width(w - 2).Height(h - 2)
}

func (m *Model) renderSections(w, h int) string {
	view := max(1, h-2)
	var b strings.Builder
	end := min(m.secOffset+view, len(m.sections))
	for real := m.secOffset; real < end; real++ {
		sec := m.sections[real]
		line := fmt.Sprintf("%s (%d)", sec.Title, len(sec.Entries))
		if real == m.secIdx && m.focus == PaneSections {
			line = m.styles.Selected.Render("❯ " + truncatePlain(line, w-6))
		} else if real == m.secIdx {
			line = "❯ " + truncatePlain(line, w-6)
		} else {
			line = m.styles.Normal.Render("  " + truncatePlain(line, w-6))
		}
		b.WriteString(line + "\n")
	}
	body := strings.TrimSuffix(b.String(), "\n")
	return m.paneStyle(m.focus == PaneSections, w, h).Render(body)
}

func (m *Model) renderEntries(w, h int) string {
	view := max(1, h-2)
	var b strings.Builder
	end := min(m.entryOffset+view, len(m.entries))
	for i := m.entryOffset; i < end; i++ {
		e := m.entries[i]
		line := e.Key + " [" + kindHint(e.Kind) + "]"
		if e.Value != "" {
			line += " " + e.Value
		}
		if e.Dirty {
			line += " " + m.styles.DirtyDot.Render("●")
		}
		if m.filterMode && m.focus == PaneEntries {
			line = truncatePlain(line, w-4)
		}
		if i == m.entryIdx && m.focus == PaneEntries {
			line = m.styles.Selected.Render("❯ " + truncatePlain(line, w-6))
		} else if i == m.entryIdx {
			line = "❯ " + truncatePlain(line, w-6)
		} else {
			line = m.styles.Normal.Render("  " + truncatePlain(line, w-6))
		}
		b.WriteString(line + "\n")
	}
	if len(m.entries) == 0 {
		b.WriteString(m.styles.Help.Render("  (no entries)") + "\n")
	}
	body := strings.TrimSuffix(b.String(), "\n")
	return m.paneStyle(m.focus == PaneEntries, w, h).Render(body)
}

func (m *Model) renderDetail(w, h int) string {
	var b strings.Builder
	ro := ""
	if sec := m.curSection(); sec != nil && sec.ReadOnly {
		ro = " (read-only)"
	}
	title := "Detail"
	if m.detail.Title != "" {
		title = m.detail.Title + ro
	}
	b.WriteString(m.styles.PaneTitle.Render(truncatePlain(title, w-4)) + "\n")
	view := max(1, h-3)
	end := min(m.detailOff+view, len(m.detail.Lines))
	for i := m.detailOff; i < end; i++ {
		l := m.detail.Lines[i]
		line := l.Label + ": " + l.Value
		if l.Bool {
			line += " [bool]"
		}
		if i == m.detailLn && m.focus == PaneDetail {
			line = m.styles.Selected.Render("❯ " + truncatePlain(line, w-6))
		} else if i == m.detailLn {
			line = "❯ " + truncatePlain(line, w-6)
		} else {
			line = m.styles.Normal.Render("  " + truncatePlain(line, w-6))
		}
		b.WriteString(line + "\n")
	}
	if len(m.detail.Lines) == 0 {
		b.WriteString(m.styles.Help.Render("  (nothing selected)") + "\n")
	}
	body := strings.TrimSuffix(b.String(), "\n")
	return m.paneStyle(m.focus == PaneDetail, w, h).Render(body)
}

func (m *Model) renderKeybar() string {
	var keys string
	switch m.focus {
	case PaneSections:
		keys = "1-5 jump  enter entries  l/tab next  [/] section  s save  r reload  ? help  q quit"
	case PaneEntries:
		if m.filterMode {
			keys = "type to filter  enter done  esc clear/close"
		} else {
			keys = "e edit  a add  d del  space toggle  / filter  enter detail  s save  r reload  ? help"
		}
	default:
		keys = "e edit  space toggle  esc back  s save  r reload  ? help  q quit"
	}
	return truncatePlain(m.styles.Keybar.Render(keys), m.width)
}

func (m *Model) renderStatus() string {
	n := m.dirtyCount()
	state := "clean"
	if n > 0 {
		state = fmt.Sprintf("%s dirty (%d)", m.styles.DirtyDot.Render("●"), n)
	}
	left := m.activePath() + "  " + state
	right := m.status
	if right == "" {
		right = "ready"
	}
	if m.filter != "" {
		right = "filter: " + m.filter + "  " + right
	}
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + right
	return m.styles.Status.Width(m.width - 2).Render(truncatePlain(line, m.width-2))
}

func (m *Model) renderOverlay() string {
	switch m.overlay {
	case OverlayHelp:
		return m.styles.OverlayBox.Width(min(m.width-4, 66)).Render(m.helpText())
	case OverlayConfirm:
		body := m.confirmText + "\n\n[enter] confirm   [esc] cancel"
		return m.styles.OverlayBox.Render(body)
	case OverlayEdit:
		body := m.editTitle + "\n\n> " + m.editBuf + "▌"
		if m.editErr != "" {
			body += "\n" + m.styles.ErrorMsg.Render(m.editErr)
		} else {
			body += "\n[enter] apply   [esc] cancel"
		}
		return m.styles.OverlayBox.Width(min(m.width-4, 60)).Render(body)
	case OverlayAdd:
		sec := ""
		if s := m.curSection(); s != nil {
			sec = string(s.ID)
		}
		body := "Add entry to " + sec + "\n\n> " + m.addBuf + "▌"
		if m.addErr != "" {
			body += "\n" + m.styles.ErrorMsg.Render(m.addErr)
		} else {
			body += "\n[enter] add   [esc] cancel"
		}
		return m.styles.OverlayBox.Width(min(m.width-4, 60)).Render(body)
	case OverlayError:
		body := m.styles.ErrorMsg.Render("Error") + "\n\n" + m.errorText + "\n\n[enter/esc] close"
		return m.styles.OverlayBox.Width(min(m.width-4, 60)).Render(body)
	default:
		return ""
	}
}

func (m *Model) helpText() string {
	rows := [][]string{
		{"1-5", "Select section, focus Entries"},
		{"0", "Focus Entries/Detail toggle"},
		{"j/k", "Move selection"},
		{"h/l", "Move focus left/right"},
		{"tab/shift+tab", "Next/previous pane"},
		{"[ / ]", "Previous/next section"},
		{"enter", "Drill down / confirm"},
		{"esc", "Climb back / close popup"},
		{"e", "Edit scalar value"},
		{"a", "Add entry"},
		{"d", "Delete entry (confirm)"},
		{"space", "Toggle bool"},
		{"s", "Save (confirm when dirty)"},
		{"r", "Reload (confirm when dirty)"},
		{"?", "Open/close help"},
		{"q", "Quit (confirm when dirty)"},
		{"/", "Filter entries"},
	}
	var b strings.Builder
	b.WriteString(m.styles.PaneTitle.Render("lazyomo keys") + "\n\n")
	lines := make([]string, 0, len(rows))
	for _, r := range rows {
		lines = append(lines, fmt.Sprintf("%-14s %s", r[0], r[1]))
	}
	start := min(m.helpScroll, max(0, len(lines)-1))
	end := min(start+14, len(lines))
	for _, l := range lines[start:end] {
		b.WriteString(l + "\n")
	}
	b.WriteString("\n[?/esc] close")
	return strings.TrimSuffix(b.String(), "\n")
}

// kindHint derives the entry-row type hint locally from EntryKind.
// The editor API is frozen and exposes no hint helper; this stays a
// TUI-side rendering concern.
func kindHint(k editor.EntryKind) string {
	switch k {
	case editor.KindScalar:
		return "scalar"
	case editor.KindBool:
		return "bool"
	case editor.KindChain:
		return "chain"
	case editor.KindAlias:
		return "alias"
	default:
		return "?"
	}
}

func truncatePlain(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

func truncate(s string, n int) string {
	return truncatePlain(s, n)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
