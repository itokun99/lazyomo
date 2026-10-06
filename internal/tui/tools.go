package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/itokun99/lazyomo/internal/derived"
)

type toolsRow struct {
	name  string
	count int
}

type toolsDetailLine struct {
	Label string
	Value string
}

func (m *Model) sideTools() bool {
	row := m.selSide()
	return row != nil && row.hasFuture && row.future == futureMCPTools
}

func (m *Model) refreshTools() {
	m.entries = nil
	m.detailLn = 0
	if strings.TrimSpace(m.toolsPath) == "" {
		m.toolsErr = "mcp-cache.json unavailable (no path)"
		m.toolsRows = nil
		m.toolsDetail = nil
		return
	}
	snap, err := derived.LoadTools(m.toolsPath)
	if err != nil {
		m.toolsErr = "mcp-cache.json unavailable (unreadable or invalid)"
		m.toolsRows = nil
		m.toolsDetail = nil
		m.entryIdx = clamp(m.entryIdx, 0)
		return
	}
	m.tools = snap
	m.toolsErr = ""
	rows := make([]toolsRow, 0, len(snap.Servers))
	for _, s := range snap.Servers {
		if m.filter != "" && !containsFold(s.Name, m.filter) {
			continue
		}
		rows = append(rows, toolsRow{name: s.Name, count: s.ToolCount})
	}
	m.toolsRows = rows
	m.entryIdx = clamp(m.entryIdx, len(m.toolsRows))
	m.entryOffset = adjustOffset(m.entryOffset, m.entryIdx, m.entriesViewport())
	m.toolsDetail = m.buildToolsDetail()
	m.detailLn = clamp(m.detailLn, len(m.toolsDetail))
	m.detailOff = adjustOffset(m.detailOff, m.detailLn, m.detailListViewport())
}

func (m *Model) buildToolsDetail() []toolsDetailLine {
	if len(m.toolsRows) == 0 || m.entryIdx < 0 || m.entryIdx >= len(m.toolsRows) {
		return nil
	}
	row := m.toolsRows[m.entryIdx]
	var server *derived.ToolsServer
	for i := range m.tools.Servers {
		if m.tools.Servers[i].Name == row.name {
			server = &m.tools.Servers[i]
			break
		}
	}
	if server == nil {
		return nil
	}
	refreshed := m.tools.RefreshedAt.Format(time.RFC3339)
	lines := []toolsDetailLine{
		{Label: "server", Value: server.Name},
		{Label: "tools", Value: itoa(server.ToolCount) + " tools"},
		{Label: "fetchedAt", Value: orUnknown(server.FetchedAt)},
		{Label: "refreshed", Value: refreshed},
		{Label: "source", Value: m.tools.Path},
	}
	for _, t := range server.Tools {
		lines = append(lines, toolsDetailLine{Label: t.Name, Value: orUnknown(t.Description)})
	}
	return lines
}

func (m *Model) selToolsRow() *toolsRow {
	if len(m.toolsRows) == 0 || m.entryIdx < 0 || m.entryIdx >= len(m.toolsRows) {
		return nil
	}
	return &m.toolsRows[m.entryIdx]
}

func (m *Model) toolsEntryLines(inner int) []string {
	if m.toolsErr != "" {
		return []string{m.styles.Help.Render(padRight(fitWidth("  "+m.toolsErr, inner), inner))}
	}
	if len(m.toolsRows) == 0 {
		text := "  (no servers)"
		if m.filter != "" {
			text = "  (no matches)"
		}
		return []string{m.styles.Help.Render(padRight(fitWidth(text, inner), inner))}
	}
	focused := m.focus == PaneEntries
	lines := make([]string, 0, len(m.toolsRows))
	end := min(m.entryOffset+max(1, m.entriesViewport()), len(m.toolsRows))
	for i := m.entryOffset; i < end; i++ {
		row := m.toolsRows[i]
		countWord := "tools"
		if row.count == 1 {
			countWord = "tool"
		}
		text := row.name + "  " + itoa(row.count) + " " + countWord
		text = fitWidth(text, max(0, inner-2))
		line := padRight("  "+text, inner)
		if i == m.entryIdx {
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

func (m *Model) toolsDetailLines(inner int) []string {
	if m.toolsErr != "" {
		return []string{m.styles.Help.Render(padRight(fitWidth("  "+m.toolsErr, inner), inner))}
	}
	if len(m.toolsDetail) == 0 {
		return []string{m.styles.Help.Render(padRight("  (no server selected)", inner))}
	}
	row := m.selToolsRow()
	header := ""
	if row != nil {
		header = row.name + " · derived · " + m.tools.Path
	} else {
		header = "Tools · derived · " + m.tools.Path
	}
	lines := []string{m.styles.Help.Render(padRight(fitWidth("  "+header, inner), inner))}
	labelW := m.toolsLabelWidth()
	focused := m.focus == PaneDetail
	end := min(m.detailOff+max(1, m.detailListViewport()), len(m.toolsDetail))
	for i := m.detailOff; i < end; i++ {
		entry := m.toolsDetail[i]
		text := padRight(entry.Label, labelW) + " " + entry.Value
		text = fitWidth(text, max(0, inner-2))
		painted := padRight("  "+text, inner)
		if i == m.detailLn {
			plain := "❯ " + text
			if focused {
				painted = m.styles.Selected.Render(padRight(plain, inner))
			} else {
				painted = padRight(plain, inner)
			}
		} else {
			painted = m.styles.Normal.Render(painted)
		}
		lines = append(lines, painted)
	}
	return lines
}

func (m *Model) toolsLabelWidth() int {
	w := 4
	for _, entry := range m.toolsDetail {
		if n := lipgloss.Width(entry.Label); n > w {
			w = n
		}
	}
	if w > 20 {
		w = 20
	}
	return w
}
