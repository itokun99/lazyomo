package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/itokun99/lazyomo/internal/derived"
	"github.com/itokun99/lazyomo/internal/mcpfile"
)

type providerRow struct {
	key     string
	name    string
	api     string
	baseURL string
	count   int
}

type providerDetailLine struct {
	Label string
	Value string
}

type catalogRow struct {
	provider string
	id       string
	name     string
}

type catalogDetailLine struct {
	Label string
	Value string
}

func (m *Model) setDerivedPaths(modelsPath, storePath string) {
	m.modelsPath = modelsPath
	m.storePath = storePath
}

func (m *Model) bindDerived() {
	if m.modelsPath != "" && m.storePath != "" {
		return
	}
	agentDir, err := mcpfile.ResolveAgentDir()
	if err != nil {
		return
	}
	modelsPath, storePath := derived.Paths(agentDir)
	if m.modelsPath == "" {
		m.modelsPath = modelsPath
	}
	if m.storePath == "" {
		m.storePath = storePath
	}
}

func (m *Model) sideProviders() bool {
	row := m.selSide()
	return row != nil && row.hasFuture && row.future == futureProviders
}

func (m *Model) sideCatalog() bool {
	row := m.selSide()
	return row != nil && row.hasFuture && row.future == futureCatalog
}

func (m *Model) sideDerived() bool {
	return m.sideProviders() || m.sideCatalog()
}

func (m *Model) refreshProviders() {
	m.entries = nil
	m.detailLn = 0
	if strings.TrimSpace(m.modelsPath) == "" {
		m.providersErr = "models.json unavailable (no path)"
		m.providerRows = nil
		m.providerDetail = nil
		return
	}
	snap, err := derived.LoadProviders(m.modelsPath)
	if err != nil {
		m.providersErr = "models.json unavailable (unreadable or invalid)"
		m.providerRows = nil
		m.providerDetail = nil
		m.entryIdx = clamp(m.entryIdx, 0)
		return
	}
	m.providers = snap
	m.providersErr = ""
	rows := make([]providerRow, 0, len(snap.Providers))
	for _, p := range snap.Providers {
		if m.filter != "" && !containsFold(p.Key, m.filter) && !containsFold(p.Name, m.filter) {
			continue
		}
		rows = append(rows, providerRow{key: p.Key, name: p.Name, api: p.API, baseURL: p.BaseURL, count: p.ModelCount})
	}
	m.providerRows = rows
	m.entryIdx = clamp(m.entryIdx, len(m.providerRows))
	m.entryOffset = adjustOffset(m.entryOffset, m.entryIdx, m.entriesViewport())
	m.providerDetail = m.buildProviderDetail()
	m.detailLn = clamp(m.detailLn, len(m.providerDetail))
	m.detailOff = adjustOffset(m.detailOff, m.detailLn, m.detailListViewport())
}

func (m *Model) buildProviderDetail() []providerDetailLine {
	if len(m.providerRows) == 0 || m.entryIdx < 0 || m.entryIdx >= len(m.providerRows) {
		return nil
	}
	row := m.providerRows[m.entryIdx]
	refreshed := m.providers.RefreshedAt.Format(time.RFC3339)
	return []providerDetailLine{
		{Label: "name", Value: row.name},
		{Label: "api", Value: orUnknown(row.api)},
		{Label: "baseUrl", Value: orUnknown(row.baseURL)},
		{Label: "models", Value: itoa(row.count) + " models"},
		{Label: "refreshed", Value: refreshed},
		{Label: "source", Value: m.providers.Path},
	}
}

func (m *Model) refreshCatalog() {
	m.entries = nil
	m.detailLn = 0
	if strings.TrimSpace(m.storePath) == "" {
		m.catalogErr = "models-store.json unavailable (no path)"
		m.catalogRows = nil
		m.catalogDetail = nil
		return
	}
	snap, err := derived.LoadCatalog(m.storePath)
	if err != nil {
		m.catalogErr = "models-store.json unavailable (unreadable or invalid)"
		m.catalogRows = nil
		m.catalogDetail = nil
		m.entryIdx = clamp(m.entryIdx, 0)
		return
	}
	m.catalog = snap
	m.catalogErr = ""
	var rows []catalogRow
	for _, p := range snap.Providers {
		for _, mod := range p.Models {
			value := p.Name + "/" + mod.ID
			if m.filter != "" && !containsFold(value, m.filter) && !containsFold(mod.Name, m.filter) {
				continue
			}
			rows = append(rows, catalogRow{provider: p.Name, id: mod.ID, name: mod.Name})
		}
	}
	m.catalogRows = rows
	m.entryIdx = clamp(m.entryIdx, len(m.catalogRows))
	m.entryOffset = adjustOffset(m.entryOffset, m.entryIdx, m.entriesViewport())
	m.catalogDetail = m.buildCatalogDetail()
	m.detailLn = clamp(m.detailLn, len(m.catalogDetail))
	m.detailOff = adjustOffset(m.detailOff, m.detailLn, m.detailListViewport())
}

func (m *Model) buildCatalogDetail() []catalogDetailLine {
	if len(m.catalogRows) == 0 || m.entryIdx < 0 || m.entryIdx >= len(m.catalogRows) {
		return nil
	}
	row := m.catalogRows[m.entryIdx]
	var mod derived.CatalogModel
	var prov derived.CatalogProvider
	for _, p := range m.catalog.Providers {
		if p.Name != row.provider {
			continue
		}
		prov = p
		for _, candidate := range p.Models {
			if candidate.ID == row.id {
				mod = candidate
				break
			}
		}
		break
	}
	lines := []catalogDetailLine{
		{Label: "id", Value: mod.ID},
		{Label: "name", Value: mod.Name},
		{Label: "provider", Value: row.provider},
		{Label: "cost", Value: orUnknown(mod.Cost)},
		{Label: "contextWindow", Value: formatContextWindow(mod.ContextWindow)},
		{Label: "checkedAt", Value: orUnknown(prov.CheckedAt)},
		{Label: "etag", Value: orUnknown(prov.ETag)},
		{Label: "refreshed", Value: m.catalog.RefreshedAt.Format(time.RFC3339)},
		{Label: "source", Value: m.catalog.Path},
	}
	return lines
}

func orUnknown(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(unknown)"
	}
	return s
}

func formatContextWindow(n int) string {
	if n <= 0 {
		return "(unknown)"
	}
	return itoa(n)
}

func (m *Model) selProviderRow() *providerRow {
	if len(m.providerRows) == 0 || m.entryIdx < 0 || m.entryIdx >= len(m.providerRows) {
		return nil
	}
	return &m.providerRows[m.entryIdx]
}

func (m *Model) selCatalogRow() *catalogRow {
	if len(m.catalogRows) == 0 || m.entryIdx < 0 || m.entryIdx >= len(m.catalogRows) {
		return nil
	}
	return &m.catalogRows[m.entryIdx]
}

func (m *Model) providerEntryLines(inner int) []string {
	if m.providersErr != "" {
		return []string{m.styles.Help.Render(padRight(fitWidth("  "+m.providersErr, inner), inner))}
	}
	if len(m.providerRows) == 0 {
		text := "  (no providers)"
		if m.filter != "" {
			text = "  (no matches)"
		}
		return []string{m.styles.Help.Render(padRight(fitWidth(text, inner), inner))}
	}
	focused := m.focus == PaneEntries
	lines := make([]string, 0, len(m.providerRows))
	end := min(m.entryOffset+max(1, m.entriesViewport()), len(m.providerRows))
	for i := m.entryOffset; i < end; i++ {
		row := m.providerRows[i]
		countWord := "models"
		if row.count == 1 {
			countWord = "model"
		}
		text := row.key + "  " + row.name + "  " + orUnknown(row.api) + "  " + itoa(row.count) + " " + countWord
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

func (m *Model) providerDetailLines(inner int) []string {
	if m.providersErr != "" {
		return []string{m.styles.Help.Render(padRight(fitWidth("  "+m.providersErr, inner), inner))}
	}
	if len(m.providerDetail) == 0 {
		return []string{m.styles.Help.Render(padRight("  (no provider selected)", inner))}
	}
	row := m.selProviderRow()
	header := ""
	if row != nil {
		header = row.key + " · derived · " + m.providers.Path
	} else {
		header = "Connections · derived · " + m.providers.Path
	}
	lines := []string{m.styles.Help.Render(padRight(fitWidth("  "+header, inner), inner))}
	labelW := m.providerLabelWidth()
	focused := m.focus == PaneDetail
	end := min(m.detailOff+max(1, m.detailListViewport()), len(m.providerDetail))
	for i := m.detailOff; i < end; i++ {
		entry := m.providerDetail[i]
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

func (m *Model) providerLabelWidth() int {
	w := 4
	for _, entry := range m.providerDetail {
		if n := lipgloss.Width(entry.Label); n > w {
			w = n
		}
	}
	if w > 20 {
		w = 20
	}
	return w
}

func (m *Model) catalogEntryLines(inner int) []string {
	if m.catalogErr != "" {
		return []string{m.styles.Help.Render(padRight(fitWidth("  "+m.catalogErr, inner), inner))}
	}
	if len(m.catalogRows) == 0 {
		text := "  (no models)"
		if m.filter != "" {
			text = "  (no matches)"
		}
		return []string{m.styles.Help.Render(padRight(fitWidth(text, inner), inner))}
	}
	focused := m.focus == PaneEntries
	lines := make([]string, 0, len(m.catalogRows))
	end := min(m.entryOffset+max(1, m.entriesViewport()), len(m.catalogRows))
	for i := m.entryOffset; i < end; i++ {
		row := m.catalogRows[i]
		text := row.provider + "/" + row.id + "  " + row.name
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

func (m *Model) catalogDetailLines(inner int) []string {
	if m.catalogErr != "" {
		return []string{m.styles.Help.Render(padRight(fitWidth("  "+m.catalogErr, inner), inner))}
	}
	if len(m.catalogDetail) == 0 {
		return []string{m.styles.Help.Render(padRight("  (no model selected)", inner))}
	}
	row := m.selCatalogRow()
	header := ""
	if row != nil {
		header = row.provider + "/" + row.id + " · derived · " + m.catalog.Path
	} else {
		header = "Catalog · derived · " + m.catalog.Path
	}
	lines := []string{m.styles.Help.Render(padRight(fitWidth("  "+header, inner), inner))}
	labelW := m.catalogLabelWidth()
	focused := m.focus == PaneDetail
	end := min(m.detailOff+max(1, m.detailListViewport()), len(m.catalogDetail))
	for i := m.detailOff; i < end; i++ {
		entry := m.catalogDetail[i]
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

func (m *Model) catalogLabelWidth() int {
	w := 4
	for _, entry := range m.catalogDetail {
		if n := lipgloss.Width(entry.Label); n > w {
			w = n
		}
	}
	if w > 20 {
		w = 20
	}
	return w
}
