package workspace

import "github.com/itokun99/lazyomo/internal/editor"

// Provenance classifies how a surface row may be changed.
type Provenance string

// Row provenances. Editable rows can be written through their source's
// session; read-only rows are displayed but never written in this wave;
// derived rows are computed from external data files and are never written.
const (
	ProvenanceEditable Provenance = "editable"
	ProvenanceReadOnly Provenance = "read-only"
	ProvenanceDerived  Provenance = "derived"
)

// Row is one addressable line of the unified multi-source surface: an RFC
// 6901 pointer into a source's document plus the provenance that decides
// whether the row can be written. Editor sections, mcpfile surfaces (a later
// wave), and derived readers all map into rows so renderers badge them
// uniformly.
type Row struct {
	SourceID   string
	Pointer    string
	Provenance Provenance
}

// EditorRows maps the entries of an editor's sections onto surface rows.
// Sections and entries flagged read-only map to ProvenanceReadOnly.
func EditorRows(sourceID string, sections []editor.Section) []Row {
	var rows []Row
	for _, section := range sections {
		for _, entry := range section.Entries {
			provenance := ProvenanceEditable
			if section.ReadOnly || entry.ReadOnly {
				provenance = ProvenanceReadOnly
			}
			rows = append(rows, Row{SourceID: sourceID, Pointer: entry.Path, Provenance: provenance})
		}
	}
	return rows
}

// DetailRows maps the lines of one editor detail view onto surface rows.
func DetailRows(sourceID string, detail editor.Detail) []Row {
	var rows []Row
	for _, line := range detail.Lines {
		provenance := ProvenanceReadOnly
		if line.Editable {
			provenance = ProvenanceEditable
		}
		rows = append(rows, Row{SourceID: sourceID, Pointer: line.Path, Provenance: provenance})
	}
	return rows
}

// DerivedRow returns the row for a computed value backed by an external data
// file (for example a provider entry read from models.json).
func DerivedRow(sourceID, pointer string) Row {
	return Row{SourceID: sourceID, Pointer: pointer, Provenance: ProvenanceDerived}
}
