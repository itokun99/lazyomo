package tui

import (
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/itokun99/lazyomo/internal/editor"
	"github.com/itokun99/lazyomo/internal/fuzzy"
	"github.com/itokun99/lazyomo/internal/workspace"
)

// OverlayPicker is the fuzzy model picker overlay (todo 13). It opens via
// `e` on scalar model fields matching ^/(models|agents|categories)/<name>/model$
// and writes the selected id verbatim through ConfigSurface.SetScalar.
const pickerMaxQuery = 128

// pickerFooter is the binding footer inside the picker box (P3: the base
// keybar stays hidden while an overlay is open, so the box carries it).
const pickerFooter = "type to filter  enter select  tab preview  ctrl+e raw  esc cancel"

var pickerPathRe = regexp.MustCompile(`^/(models|agents|categories)/[^/]+/model$`)

func isPickerPath(path string) bool {
	return pickerPathRe.MatchString(path)
}

func isModelsPickerPath(path string) bool {
	if !isPickerPath(path) {
		return false
	}
	return strings.HasPrefix(path, "/models/")
}

// pickerState owns one open picker: the target pointer, its value at open,
// the filter query (capped at 128 runes), the rebuilt candidate set, the
// ranked view over it, and the cursor/scroll plus preview toggle.
type pickerState struct {
	path    string
	current string
	query   []rune
	cands   []workspace.Candidate
	results []fuzzy.Result
	cursor  int
	offset  int
	preview bool
}

// pickerCandidatesFor rebuilds the candidate set for one open: fresh
// workspace.BuildCandidates over the live catalog files plus the document,
// filtered to refs-only for the models-section field (aliases stay for
// alias-accepting agents/categories fields). Tests inject pickerBuild to
// supply synthetic fixtures; production uses the bound *editor.Editor.
func (m *Model) pickerCandidatesFor(targetPath string) []workspace.Candidate {
	var cands []workspace.Candidate
	if m.pickerBuild != nil {
		cands = m.pickerBuild(targetPath)
	} else if ed, ok := m.config.(*editor.Editor); ok {
		cands = workspace.BuildCandidates(m.storePath, m.modelsPath, ed)
	}
	if isModelsPickerPath(targetPath) {
		kept := make([]workspace.Candidate, 0, len(cands))
		for _, c := range cands {
			if c.Group == fuzzy.GroupAlias {
				continue
			}
			kept = append(kept, c)
		}
		return kept
	}
	return cands
}

// openPicker rebuilds candidates and opens the overlay. An empty candidate
// set falls back to the raw-text edit overlay so `e` always edits.
func (m *Model) openPicker(targetPath, current string) {
	cands := m.pickerCandidatesFor(targetPath)
	if len(cands) == 0 {
		m.overlay = OverlayEdit
		m.editPath = targetPath
		m.editTitle = "Edit " + targetPath
		m.editBuf = current
		m.editErr = ""
		return
	}
	m.picker = &pickerState{
		path:    targetPath,
		current: current,
		preview: m.width >= 100,
		cands:   cands,
	}
	m.refilterPicker()
	m.overlay = OverlayPicker
}

// refilterPicker re-ranks the rebuilt list on every keystroke.
func (m *Model) refilterPicker() {
	p := m.picker
	if p == nil {
		return
	}
	p.results = fuzzy.Filter(workspace.ToFuzzy(p.cands), string(p.query))
	p.cursor = clamp(p.cursor, len(p.results))
	if len(p.results) == 0 {
		p.cursor = 0
		p.offset = 0
		return
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor >= len(p.results) {
		p.cursor = len(p.results) - 1
	}
	rows := pickerResultRows(m.width, m.height)
	p.offset = adjustOffset(p.offset, p.cursor, max(1, rows))
}

// pickerSelected maps the cursor back to the workspace candidate for
// preview and write-back.
func (m *Model) pickerSelected() *workspace.Candidate {
	p := m.picker
	if p == nil || len(p.results) == 0 {
		return nil
	}
	if p.cursor < 0 || p.cursor >= len(p.results) {
		return nil
	}
	idx := p.results[p.cursor].Index
	if idx < 0 || idx >= len(p.cands) {
		return nil
	}
	return &p.cands[idx]
}

// handlePickerKey owns every key while the picker is open. Printable runes
// (including q/space/digits) extend the query; navigation moves the cursor;
// tab toggles preview; enter selects; esc cancels with zero mutation;
// ctrl+e escapes to raw text; ctrl+c quits (the top-level handleKey already
// quits first, this is a backstop for direct dispatch).
func (m *Model) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	p := m.picker
	if p == nil {
		m.overlay = OverlayNone
		return m, nil
	}
	rows := max(1, pickerResultRows(m.width, m.height))
	switch msg.String() {
	case "enter":
		m.pickerSelect()
		return m, nil
	case "esc":
		m.overlay = OverlayNone
		m.picker = nil
		return m, nil
	case "tab":
		p.preview = !p.preview
		return m, nil
	case "ctrl+e":
		m.pickerToRaw()
		return m, nil
	case "ctrl+u":
		if len(p.query) > 0 {
			p.query = nil
			p.cursor = 0
			p.offset = 0
			m.refilterPicker()
		}
		return m, nil
	case "backspace":
		if len(p.query) > 0 {
			p.query = p.query[:len(p.query)-1]
			p.cursor = 0
			p.offset = 0
			m.refilterPicker()
		}
		return m, nil
	case "up", "ctrl+p":
		m.pickerMove(-1, rows)
		return m, nil
	case "down", "ctrl+n":
		m.pickerMove(1, rows)
		return m, nil
	case "pgup":
		m.pickerMove(-rows, rows)
		return m, nil
	case "pgdown", "pgdn":
		m.pickerMove(rows, rows)
		return m, nil
	case "home":
		if len(p.results) > 0 {
			p.cursor = 0
			p.offset = adjustOffset(p.offset, p.cursor, rows)
		}
		return m, nil
	case "end":
		if len(p.results) > 0 {
			p.cursor = len(p.results) - 1
			p.offset = adjustOffset(p.offset, p.cursor, rows)
		}
		return m, nil
	}
	if msg.Type == tea.KeySpace {
		m.pickerAppendRunes([]rune{' '})
		return m, nil
	}
	if msg.Type == tea.KeyRunes {
		m.pickerAppendRunes(msg.Runes)
		return m, nil
	}
	return m, nil
}

// pickerAppendRunes appends printable runes up to the 128 cap and re-ranks.
func (m *Model) pickerAppendRunes(runes []rune) {
	p := m.picker
	if p == nil || len(runes) == 0 {
		return
	}
	for _, r := range runes {
		if len(p.query) >= pickerMaxQuery {
			break
		}
		if r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		p.query = append(p.query, r)
	}
	p.cursor = 0
	p.offset = 0
	m.refilterPicker()
}

// pickerMove steps the cursor and keeps it in the scrolled window.
func (m *Model) pickerMove(delta, rows int) {
	p := m.picker
	if p == nil || len(p.results) == 0 {
		return
	}
	next := p.cursor + delta
	if next < 0 {
		next = 0
	}
	if next >= len(p.results) {
		next = len(p.results) - 1
	}
	p.cursor = next
	p.offset = adjustOffset(p.offset, p.cursor, rows)
}

// pickerSelect writes the cursor candidate verbatim. No match keeps the
// picker open (Enter inert); selecting the current value closes as
// unchanged with no write and no dirty.
func (m *Model) pickerSelect() {
	p := m.picker
	if p == nil {
		return
	}
	sel := m.pickerSelected()
	if sel == nil {
		return
	}
	if sel.Value == p.current {
		path := p.path
		m.overlay = OverlayNone
		m.picker = nil
		m.status = "unchanged " + path
		return
	}
	path := p.path
	value := sel.Value
	if m.config == nil {
		m.status = "no config surface"
		return
	}
	if err := m.config.SetScalar(path, value); err != nil {
		m.overlay = OverlayNone
		m.picker = nil
		m.status = "set failed: " + err.Error()
		m.refresh()
		return
	}
	m.overlay = OverlayNone
	m.picker = nil
	m.status = "set " + path
	m.refresh()
}

// pickerToRaw escapes to the existing raw-text edit overlay on the same
// pointer, prefilled with the value at picker open.
func (m *Model) pickerToRaw() {
	p := m.picker
	if p == nil {
		return
	}
	m.overlay = OverlayEdit
	m.editPath = p.path
	m.editTitle = "Edit " + p.path
	m.editBuf = p.current
	m.editErr = ""
	m.picker = nil
}

// pickerQueryDisplay tail-shows the query when it overflows the box: the
// newest (rightmost) input stays visible.
func pickerQueryDisplay(query []rune, n int) string {
	if n <= 0 {
		return ""
	}
	q := string(query)
	if lipgloss.Width(q) <= n {
		return q
	}
	if n == 1 {
		return "…"
	}
	// Keep the tail that fits n-1 columns, prefixed with the marker.
	rs := []rune(q)
	lo := len(rs)
	w := 0
	for lo > 0 {
		rw := lipgloss.Width(string(rs[lo-1]))
		if w+rw > n-1 {
			break
		}
		w += rw
		lo--
	}
	return "…" + string(rs[lo:])
}

// middleTruncate keeps head and tail around a … marker so long ids fit
// without wrapping.
func middleTruncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	headBudget := (n - 1) / 2
	tailBudget := n - 1 - headBudget
	rs := []rune(s)
	// Head.
	headW := 0
	headN := 0
	for _, r := range rs {
		rw := lipgloss.Width(string(r))
		if headW+rw > headBudget {
			break
		}
		headW += rw
		headN++
	}
	// Tail.
	tailW := 0
	tailN := 0
	for i := len(rs) - 1; i >= 0; i-- {
		rw := lipgloss.Width(string(rs[i]))
		if tailW+rw > tailBudget {
			break
		}
		tailW += rw
		tailN++
	}
	return string(rs[:headN]) + "…" + string(rs[len(rs)-tailN:])
}

// pickerGroupLabel renders the candidate group without color dependence.
func pickerGroupLabel(group int) string {
	if group == fuzzy.GroupAlias {
		return "alias"
	}
	return "ref"
}

// pickerPreviewLines builds the preview column for the cursor candidate:
// value/group/provider/used-in/write-path, truncated to the column width.
func (m *Model) pickerPreviewLines(width int) []string {
	sel := m.pickerSelected()
	if sel == nil {
		return []string{"(no match)"}
	}
	provider := sel.Provider
	if provider == "" {
		if sel.Group == fuzzy.GroupAlias {
			provider = "(alias)"
		} else {
			provider = "(unknown)"
		}
	}
	used := "(none)"
	if len(sel.Usages) > 0 {
		used = strings.Join(sel.Usages, ", ")
	}
	lines := []string{
		"value: " + sel.Value,
		"group: " + pickerGroupLabel(sel.Group),
		"provider: " + provider,
		"used-in: " + used,
		"write-path: " + m.picker.path,
	}
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, middleTruncate(l, max(1, width)))
	}
	return out
}

// renderPicker paints the picker box at min(110,w-4)x min(30,h-4) with at
// least 14 result rows at 80x24 (24 at 120x40). Result rows are
// middleTruncated; the query tail-shows; the footer carries the binding
// contract; the cursor uses the ❯ non-color cue.
func (m *Model) renderPicker() string {
	p := m.picker
	if p == nil {
		return ""
	}
	boxW, boxH := pickerBox(m.width, m.height)
	inner := max(1, boxW-2)
	rows := pickerResultRows(m.width, m.height)
	frame := m.styles.FrameFocus
	title := middleTruncate("Pick model", max(1, inner-4))
	top := "╭─ " + title + " " + strings.Repeat("─", max(1, boxW-5-lipgloss.Width(title))) + "╮"
	top = fitWidth(top, boxW)
	out := []string{frame.Render(top)}
	contentW := max(1, inner-2)
	// Query line.
	qdisp := pickerQueryDisplay(p.query, max(1, contentW-4))
	qline := "> " + qdisp + "▌"
	out = append(out, frame.Render("│")+padRight("  "+fitWidth(qline, contentW), inner)+frame.Render("│"))
	// Count/header line.
	header := ""
	if len(p.results) == 0 {
		if len(p.query) == 0 {
			header = "  " + itoa(len(p.cands)) + " models"
		} else {
			header = "  no matches"
		}
	} else {
		header = "  " + itoa(len(p.results)) + "/" + itoa(len(p.cands))
	}
	out = append(out, frame.Render("│")+padRight(fitWidth(header, inner), inner)+frame.Render("│"))
	// Result rows, split into columns when previewing.
	previewOn := p.preview
	var leftW, rightW int
	if previewOn {
		leftW = inner * 55 / 100
		if leftW < 20 {
			leftW = 20
		}
		if leftW > inner-12 {
			leftW = inner - 12
		}
		rightW = inner - leftW - 1
		if rightW < 10 {
			previewOn = false
		}
	}
	var previewLines []string
	if previewOn {
		previewLines = m.pickerPreviewLines(max(1, rightW-2))
	}
	for i := 0; i < rows; i++ {
		idx := p.offset + i
		if !previewOn {
			line := ""
			if idx < len(p.results) {
				cand := p.cands[p.results[idx].Index]
				text := middleTruncate(cand.Value, max(1, contentW-2))
				if idx == p.cursor {
					line = m.styles.Selected.Render(padRight("❯ "+text, inner))
				} else {
					line = m.styles.Normal.Render(padRight("  "+text, inner))
				}
			} else {
				line = padRight("", inner)
			}
			out = append(out, frame.Render("│")+line+frame.Render("│"))
			continue
		}
		left := ""
		if idx < len(p.results) {
			cand := p.cands[p.results[idx].Index]
			text := middleTruncate(cand.Value, max(1, leftW-4))
			if idx == p.cursor {
				left = m.styles.Selected.Render(padRight("❯ "+text, leftW))
			} else {
				left = m.styles.Normal.Render(padRight("  "+text, leftW))
			}
		} else {
			left = padRight("", leftW)
		}
		right := ""
		if i < len(previewLines) {
			right = m.styles.Help.Render(padRight(fitWidth(previewLines[i], rightW), rightW))
		} else {
			right = padRight("", rightW)
		}
		out = append(out, frame.Render("│")+left+frame.Render("│")+right+frame.Render("│"))
	}
	// Spacer then footer.
	out = append(out, frame.Render("│")+padRight("", inner)+frame.Render("│"))
	foot := fitWidth(pickerFooter, contentW)
	out = append(out, frame.Render("│")+padRight("  "+foot, inner)+frame.Render("│"))
	out = append(out, frame.Render("╰"+strings.Repeat("─", inner)+"╯"))
	// Pad or trim to exactly boxH lines so geometry holds.
	for len(out) < boxH {
		out = append(out, frame.Render("│")+padRight("", inner)+frame.Render("│"))
	}
	if len(out) > boxH {
		out = out[:boxH]
	}
	return strings.Join(out, "\n")
}
