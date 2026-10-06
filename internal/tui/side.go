package tui

import "github.com/itokun99/lazyomo/internal/editor"

// Grouped side panel (wireframe A): CONFIG holds the editor's config
// sections in contract order plus any extra present top-level keys as
// read-only rows; the MCP Servers row is the live surface (todo 9) and the
// Tools row the read-only mcp-cache view (todo 11); the PROVIDERS rows are
// read-only derived views (todo 10). Headers are display only; cursor, [/],
// and digit keys move over the flat selectable rows.

// futureRow identifies a placeholder side row a later wave wires.
type futureRow string

const (
	futureMCPTools  futureRow = "mcp-tools"
	futureProviders futureRow = "providers"
	futureCatalog   futureRow = "catalog"
)

// sideRow is one selectable side-panel row: bound to an editor section, to
// the MCP servers surface, or to a future placeholder (read-only-empty
// until wired).
type sideRow struct {
	label     string
	section   editor.SectionID
	future    futureRow
	hasFuture bool
	mcp       bool
	ro        bool
}

// sideGroup is one headed block of the side panel.
type sideGroup struct {
	title string
	ro    bool
	rows  []sideRow
}

// configSideOrder is the binding CONFIG order; extras append after it.
var configSideOrder = []editor.SectionID{
	editor.SectionModels,
	editor.SectionModelProfiles,
	editor.SectionAgents,
	editor.SectionCategories,
	editor.SectionTelemetry,
	editor.SectionGitMaster,
}

// buildSideGroups maps editor sections onto the grouped panel.
func buildSideGroups(sections []editor.Section) []sideGroup {
	byID := map[editor.SectionID]editor.Section{}
	for _, sec := range sections {
		byID[sec.ID] = sec
	}
	known := map[editor.SectionID]bool{}
	config := []sideRow{}
	for _, id := range configSideOrder {
		sec, ok := byID[id]
		if !ok {
			continue
		}
		known[id] = true
		config = append(config, sideRow{label: sec.Title, section: id, ro: sec.ReadOnly})
	}
	for _, sec := range sections {
		if known[sec.ID] {
			continue
		}
		config = append(config, sideRow{label: sec.Title, section: sec.ID, ro: true})
	}
	return []sideGroup{
		{title: "CONFIG", rows: config},
		{title: "MCP", rows: []sideRow{
			{label: "Servers", mcp: true},
			{label: "Tools", future: futureMCPTools, hasFuture: true, ro: true},
		}},
		{title: "PROVIDERS", ro: true, rows: []sideRow{
			{label: "Connections", future: futureProviders, hasFuture: true, ro: true},
			{label: "Catalog", future: futureCatalog, hasFuture: true, ro: true},
		}},
	}
}

// flattenSide returns the selectable rows in display order.
func flattenSide(groups []sideGroup) []sideRow {
	var out []sideRow
	for _, g := range groups {
		out = append(out, g.rows...)
	}
	return out
}

// sideTotalLines counts every painted side-panel line, headers included.
func sideTotalLines(groups []sideGroup) int {
	n := 0
	for _, g := range groups {
		n += 1 + len(g.rows)
	}
	return n
}

// sideCursorLine maps a flat selectable index to its painted line.
func sideCursorLine(groups []sideGroup, flatIdx int) int {
	line, idx := 0, 0
	for _, g := range groups {
		line++
		for range g.rows {
			if idx == flatIdx {
				return line
			}
			idx++
			line++
		}
	}
	return -1
}
