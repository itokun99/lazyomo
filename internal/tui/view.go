package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/itokun99/lazyomo/internal/editor"
)

// Wireframe-A layout (P1-P6): every pane is a hand-painted 1-line rounded
// border box with its title inside the top border; exactly one pane carries
// the focus frame (green border plus the ● title marker, so color is never
// the sole cue); the keybar renders the binding table's bar subset for the
// active context; the status line is exactly one line. Painting by hand
// keeps every line exactly its pane width for the density contract.

// View implements tea.Model.
func (m *Model) View() string {
	if m.width < MinWidth || m.height < MinHeight {
		return fmt.Sprintf(
			"Terminal too small (need %dx%d, got %dx%d)\n\nPlease resize your terminal.",
			MinWidth, MinHeight, m.width, m.height,
		)
	}
	if m.overlay != OverlayNone {
		box := m.renderOverlayBox()
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box,
			lipgloss.WithWhitespaceChars(" "))
	}
	return m.renderBase()
}

// renderBase paints exactly m.height lines, each exactly m.width columns:
// the three panes, then the keybar, then the single status line.
func (m *Model) renderBase() string {
	contentH := m.height - 2
	leftW := m.leftW()
	rightW := m.width - leftW
	entriesH := m.entriesH()
	detailH := m.detailH()

	left := m.paintPane("Sections", m.focus == PaneSections, leftW, contentH, m.sideLines(leftW-2))
	entriesTitle := m.entriesTitle()
	right := append(
		m.paintPane(entriesTitle, m.focus == PaneEntries, rightW, entriesH, m.entryLines(rightW-2)),
		m.paintPane(m.detailTitle(), m.focus == PaneDetail, rightW, detailH, m.detailLines(rightW-2))...,
	)
	lines := make([]string, 0, m.height)
	for i := 0; i < contentH; i++ {
		lines = append(lines, left[i]+right[i])
	}
	lines = append(lines, m.renderKeybar(), m.renderStatus())
	return strings.Join(lines, "\n")
}

func (m *Model) leftW() int {
	w := m.width * 30 / 100
	if w < 24 {
		w = 24
	}
	if w > m.width-20 {
		w = m.width - 20
	}
	return w
}

// entriesTitle names the entries pane after the selected side row.
func (m *Model) entriesTitle() string {
	if row := m.selSide(); row != nil {
		if row.mcp {
			if m.mcpPath != "" {
				return "MCP servers · " + m.mcpPath
			}
			return "MCP servers"
		}
		if m.sideProviders() {
			return "Connections · derived"
		}
		if m.sideCatalog() {
			return "Catalog · derived"
		}
		if m.sideTools() {
			return "Tools · derived"
		}
		if row.hasFuture {
			return row.label
		}
		if sec := m.curSection(); sec != nil {
			return sec.Title
		}
		return row.label
	}
	return "Entries"
}

// detailTitle names the detail pane after the entry, flagged read-only.
func (m *Model) detailTitle() string {
	if row := m.selSide(); row != nil && row.mcp {
		return "Detail"
	}
	if m.sideDerived() {
		return "Detail · derived"
	}
	ro := ""
	if sec := m.curSection(); sec != nil && sec.ReadOnly {
		ro = " (read-only)"
	} else if m.sideReadOnly() {
		ro = " (read-only)"
	}
	if m.detail.Title != "" {
		return m.detail.Title + ro
	}
	if e := m.selEntry(); e != nil {
		return e.Key + ro
	}
	if row := m.selSide(); row != nil {
		return row.label + ro
	}
	return "Detail" + ro
}

// paintPane draws one titled pane: exactly h lines of exactly w columns.
// The focused pane gets the green frame plus the ● title marker.
func (m *Model) paintPane(title string, focused bool, w, h int, body []string) []string {
	inner := max(1, w-2)
	frame := m.styles.FrameBlur
	marker := ""
	if focused {
		frame = m.styles.FrameFocus
		marker = "● "
	}
	titleFit := fitWidth(title, max(1, inner-2-lipgloss.Width(marker)))
	top := "╭─ " + marker + titleFit + " " + strings.Repeat("─", max(1, w-5-lipgloss.Width(marker)-lipgloss.Width(titleFit))) + "╮"
	top = fitWidth(top, w)
	lines := []string{frame.Render(top)}
	for i := 0; i < h-2; i++ {
		content := ""
		if i < len(body) {
			content = body[i]
		}
		lines = append(lines, frame.Render("│")+padRight(content, inner)+frame.Render("│"))
	}
	lines = append(lines, frame.Render("╰"+strings.Repeat("─", inner)+"╯"))
	return lines
}

// sideLines renders the grouped side panel rows for the inner width.
func (m *Model) sideLines(inner int) []string {
	focused := m.focus == PaneSections
	var all []string
	sel := m.selSide()
	for _, g := range m.groups {
		head := g.title
		if g.ro {
			head += " (ro)"
		}
		all = append(all, m.styles.Help.Render(fitWidth(head, inner)))
		for _, row := range g.rows {
			text := fitWidth(sideRowLabel(row), max(0, inner-2))
			selected := sel != nil && *sel == row
			line := "  " + text
			if selected {
				line = "❯ " + text
			}
			line = padRight(line, inner)
			if selected && focused {
				line = m.styles.Selected.Render(line)
			} else if !selected {
				line = m.styles.Normal.Render(line)
			}
			all = append(all, line)
		}
	}
	return windowOf(all, m.secOffset, max(1, m.secViewport()), inner)
}

func sideRowLabel(row sideRow) string {
	if row.ro {
		return row.label + " (ro)"
	}
	return row.label
}

// entryLines renders the entries list rows for the inner width.
func (m *Model) entryLines(inner int) []string {
	if m.sideMCP() {
		return m.mcpEntryLines(inner)
	}
	if m.sideProviders() {
		return m.providerEntryLines(inner)
	}
	if m.sideCatalog() {
		return m.catalogEntryLines(inner)
	}
	if m.sideTools() {
		return m.toolsEntryLines(inner)
	}
	focused := m.focus == PaneEntries
	if len(m.entries) == 0 {
		return []string{m.styles.Help.Render(padRight("  (no entries)", inner))}
	}
	lines := make([]string, 0, len(m.entries))
	end := min(m.entryOffset+max(1, m.entriesViewport()), len(m.entries))
	for i := m.entryOffset; i < end; i++ {
		e := m.entries[i]
		text := e.Key + " [" + kindHint(e.Kind) + "]"
		if e.Value != "" {
			text += " " + e.Value
		}
		dot := ""
		if e.Dirty {
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

// detailLines renders the detail rows for the inner width.
func (m *Model) detailLines(inner int) []string {
	if m.sideMCP() {
		return m.mcpDetailLines(inner)
	}
	if m.sideProviders() {
		return m.providerDetailLines(inner)
	}
	if m.sideCatalog() {
		return m.catalogDetailLines(inner)
	}
	if m.sideTools() {
		return m.toolsDetailLines(inner)
	}
	focused := m.focus == PaneDetail
	if len(m.detail.Lines) == 0 {
		return []string{m.styles.Help.Render(padRight("  (nothing selected)", inner))}
	}
	lines := make([]string, 0, len(m.detail.Lines))
	end := min(m.detailOff+max(1, m.detailViewport()), len(m.detail.Lines))
	for i := m.detailOff; i < end; i++ {
		l := m.detail.Lines[i]
		text := l.Label + ": " + l.Value
		if l.Bool {
			text += " [bool]"
		}
		text = fitWidth(text, max(0, inner-2))
		line := padRight("  "+text, inner)
		if i == m.detailLn {
			plain := "❯ " + text
			if focused {
				line = m.styles.Selected.Render(padRight(plain, inner))
			} else {
				line = padRight(plain, inner)
			}
		} else {
			line = m.styles.Normal.Render(line)
		}
		lines = append(lines, line)
	}
	return lines
}

// renderKeybar shows exactly the binding table's bar subset for the active
// context. It is hidden while an overlay is open (View skips the base).
func (m *Model) renderKeybar() string {
	bar := fitWidth(keybarFor(m.contextName()), m.width)
	return m.styles.Keybar.Render(padRight(bar, m.width))
}

// renderStatus paints exactly one status line: the active source path with
// the total dirty count across sources on the left, the message (or ready)
// on the right.
func (m *Model) renderStatus() string {
	n := m.dirtyCount()
	plainState := "clean"
	state := "clean"
	if n > 0 {
		plainState = "● dirty(" + itoa(n) + ")"
		state = m.styles.DirtyDot.Render("●") + " dirty(" + itoa(n) + ")"
	}
	left := m.activePath() + " · " + state
	if lipgloss.Width(left) > m.width {
		left = fitWidth(m.activePath()+" · "+plainState, m.width)
		return m.styles.Status.Render(padRight(left, m.width))
	}
	right := m.status
	if right == "" {
		right = "ready"
	}
	if m.filter != "" {
		right = "filter: " + m.filter + "  " + right
	}
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		right = fitWidth(right, max(0, m.width-lipgloss.Width(left)-1))
		gap = m.width - lipgloss.Width(left) - lipgloss.Width(right)
	}
	if gap < 0 {
		gap = 0
	}
	return m.styles.Status.Render(left + strings.Repeat(" ", gap) + right)
}

// renderOverlayBox centers a titled box for the topmost overlay.
func (m *Model) renderOverlayBox() string {
	switch m.overlay {
	case OverlayHelp:
		lines := m.helpLines()
		maxLines := max(1, m.height-6)
		start := min(m.helpScroll, max(0, len(lines)-maxLines))
		win := lines[start:min(start+maxLines, len(lines))]
		return strings.Join(m.paintBox("Help", min(m.width-4, 66), win), "\n")
	case OverlayConfirm:
		lines := strings.Split(m.confirmText, "\n")
		lines = append(lines, "", "[enter] confirm   [esc] cancel")
		return strings.Join(m.paintBox("Confirm", min(m.width-4, 66), lines), "\n")
	case OverlayEdit:
		lines := []string{"> " + m.editBuf + "▌"}
		if m.editErr != "" {
			lines = append(lines, "", m.editErr)
		} else {
			lines = append(lines, "", "[enter] apply   [esc] cancel")
		}
		return strings.Join(m.paintBox(m.editTitle, min(m.width-4, 60), lines), "\n")
	case OverlayAdd:
		sec := ""
		if s := m.curSection(); s != nil {
			sec = string(s.ID)
		}
		lines := []string{"> " + m.addBuf + "▌"}
		if m.addErr != "" {
			lines = append(lines, "", m.addErr)
		} else {
			lines = append(lines, "", "[enter] add   [esc] cancel")
		}
		return strings.Join(m.paintBox("Add entry to "+sec, min(m.width-4, 60), lines), "\n")
	case OverlayError:
		lines := append(strings.Split(m.errorText, "\n"), "", "[enter/esc] close")
		return strings.Join(m.paintBox("Error", min(m.width-4, 60), lines), "\n")
	case OverlayMCPAdd:
		return strings.Join(m.paintBox(m.mcpAddTitle(), min(m.width-4, 60), m.mcpAddLines()), "\n")
	case OverlayMCPReveal:
		lines := append(strings.Split(m.revealText, "\n"), "", "[x/esc] close")
		return strings.Join(m.paintBox("Reveal secrets", min(m.width-4, 76), lines), "\n")
	default:
		return ""
	}
}

// paintBox draws a centered overlay box of exactly w columns.
func (m *Model) paintBox(title string, w int, lines []string) []string {
	inner := max(1, w-2)
	frame := m.styles.FrameFocus
	titleFit := fitWidth(title, max(1, inner-4))
	top := "╭─ " + titleFit + " " + strings.Repeat("─", max(1, w-5-lipgloss.Width(titleFit))) + "╮"
	out := []string{frame.Render(fitWidth(top, w)), frame.Render("│" + strings.Repeat(" ", inner) + "│")}
	width := max(1, inner-2)
	for _, l := range lines {
		for _, w := range wrapLine(stripANSI(l), width) {
			out = append(out, frame.Render("│")+padRight("  "+w, inner)+frame.Render("│"))
		}
	}
	out = append(out, frame.Render("│"+strings.Repeat(" ", inner)+"│"))
	out = append(out, frame.Render("╰"+strings.Repeat("─", inner)+"╯"))
	return out
}

// helpLines renders every binding table: one headed block per context with
// its label/desc pairs, so help always matches the dispatch table.
func (m *Model) helpLines() []string {
	lines := []string{m.styles.PaneTitle.Render("lazyomo keys"), ""}
	seenCtx := map[string]bool{}
	for _, table := range bindingTables {
		if seenCtx[table.Name] {
			continue
		}
		seenCtx[table.Name] = true
		lines = append(lines, table.Title)
		notes := map[string]string{
			ctxFilter: "type to filter",
			ctxEdit:   "type to edit",
			ctxAdd:    "type to add",
			ctxPicker: "type to filter",
			ctxMCPAdd: "type a name, pick a template",
		}
		if note, ok := notes[table.Name]; ok {
			lines = append(lines, "  "+note)
		}
		seen := map[string]bool{}
		for _, b := range table.Keys {
			pair := b.Label + " " + b.Desc
			if seen[pair] {
				continue
			}
			seen[pair] = true
			lines = append(lines, "  "+padRight(b.Label, 9)+" "+b.Desc)
		}
		lines = append(lines, "")
	}
	lines = append(lines, "[?/esc] close")
	return lines
}

// wrapLine word-wraps plain text to at most n columns; words longer
// than n hard-split so paths and errors never vanish off the box edge.
func wrapLine(s string, n int) []string {
	if n <= 0 {
		return []string{""}
	}
	if lipgloss.Width(s) <= n {
		return []string{s}
	}
	var out []string
	cur := ""
	curW := 0
	flush := func() {
		if cur != "" {
			out = append(out, cur)
			cur = ""
			curW = 0
		}
	}
	for _, word := range strings.Fields(s) {
		ww := lipgloss.Width(word)
		if ww > n {
			flush()
			runes := []rune(word)
			for len(runes) > 0 {
				take := len(runes)
				for take > 0 && lipgloss.Width(string(runes[:take])) > n {
					take--
				}
				if take == 0 {
					take = 1
				}
				out = append(out, string(runes[:take]))
				runes = runes[take:]
			}
			continue
		}
		if curW > 0 && curW+1+ww > n {
			flush()
		}
		if curW > 0 {
			cur += " "
			curW++
		}
		cur += word
		curW += ww
	}
	flush()
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

// kindHint derives the entry-row type hint locally from EntryKind.
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

// fitWidth truncates plain text to at most n columns.
func fitWidth(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= n {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > n {
		runes = runes[:len(runes)-1]
	}
	return string(runes)
}

// padRight pads text (which may hold ANSI codes) to exactly n columns.
func padRight(s string, n int) string {
	if w := lipgloss.Width(s); w < n {
		return s + strings.Repeat(" ", n-w)
	}
	return s
}

// stripANSI drops SGR escape sequences for width math on styled strings.
func stripANSI(s string) string {
	var b strings.Builder
	in := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			in = true
			i++
			continue
		}
		if in {
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				in = false
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// clampWindow keeps a cursor line inside a scrolled window with margin.
func clampWindow(offset, cursor, view, total int) int {
	if view <= 0 {
		return 0
	}
	if cursor >= 0 {
		if cursor < offset+scrollMargin {
			offset = cursor - scrollMargin
		}
		if cursor > offset+view-1-scrollMargin {
			offset = cursor - view + 1 + scrollMargin
		}
	}
	if offset > total-view {
		offset = total - view
	}
	if offset < 0 {
		offset = 0
	}
	return offset
}

// windowOf slices rows to a scrolled window; short windows pad blank.
func windowOf(rows []string, start, view int, inner int) []string {
	out := make([]string, 0, view)
	for i := 0; i < view; i++ {
		if start+i < len(rows) {
			out = append(out, rows[start+i])
		} else {
			out = append(out, padRight("", inner))
		}
	}
	return out
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
