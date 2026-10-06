// Package editor implements the editable configuration surface of the
// lazyomo TUI on top of the comment-preserving JSONC engine in
// internal/omodit.
//
// The editable surface is defined by docs/spec-editor-v1.md: the models,
// model_profiles, model_profile, agents, categories, telemetry, and
// git_master keys of ~/.omo/omo.jsonc. Load parses the file once; every
// mutation validates its path and value before touching the document, so
// invalid input leaves the in-memory document (and the file on disk)
// untouched. Edits accumulate in a dirty path set until Save writes the
// document atomically with a timestamped backup, or Reload discards them.
//
// Entries created by AddEntry start as an empty string placeholder; the
// first field write (SetScalar, ToggleBool, or a chain write) replaces the
// placeholder with an object holding that field. Chain fields ("models"
// arrays on profiles, agents, and categories) accept a JSON array or a
// comma-separated string when written whole; MoveChain reorders them.
package editor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/itokun99/lazyomo/internal/omodit"
)

// SectionID names one section of the configuration: an editable section
// or one read-only section per remaining top-level key.
type SectionID string

// The six sections of the editable surface, in display order: the five
// v1 sections plus git_master. Read-only sections for every other
// present top-level key are appended after these, sorted by key.
const (
	SectionModels        SectionID = "models"
	SectionModelProfiles SectionID = "model_profiles"
	SectionAgents        SectionID = "agents"
	SectionCategories    SectionID = "categories"
	SectionTelemetry     SectionID = "telemetry"
	SectionGitMaster     SectionID = "git_master"
)

// EntryKind classifies an entry row of a section.
type EntryKind int

// Entry kinds. Sections list aliases as KindAlias and the telemetry switch
// as KindBool; KindScalar and KindChain complete the contract for scalar
// and chain rows.
const (
	KindScalar EntryKind = iota
	KindBool
	KindChain
	KindAlias
)

// Entry is one selectable row of a section. Value is the display string
// (the model of an alias, the display name of a profile, or true/false for
// the telemetry switch). Dirty reports whether the entry or one of its
// fields changed since the last Save or Reload.
type Entry struct {
	Key      string
	Path     string
	Kind     EntryKind
	Value    string
	Dirty    bool
	ReadOnly bool
}

// Section is one editable block of the configuration. Missing or
// non-object blocks render as sections without entries; the telemetry
// section always shows its enabled switch so the first space-toggle can
// create the block.
type Section struct {
	ID       SectionID
	Title    string
	Entries  []Entry
	ReadOnly bool
}

// DetailLine is one field of an entry detail view. Bool marks boolean
// values; Editable reports whether the v1 surface allows writing the
// field. Chain values are rendered as compact JSON arrays.
type DetailLine struct {
	Label    string
	Path     string
	Value    string
	Bool     bool
	Editable bool
}

// Detail is the detail view of one entry.
type Detail struct {
	Title string
	Lines []DetailLine
}

// Editor owns one loaded JSONC document and its unsaved change set.
// Methods are not safe for concurrent use.
type Editor struct {
	path  string
	doc   *omodit.Document
	dirty map[string]struct{}
}

// Load reads and parses the JSONC file at path.
func Load(path string) (*Editor, error) {
	doc, err := omodit.Load(path)
	if err != nil {
		return nil, err
	}
	return &Editor{path: path, doc: doc, dirty: make(map[string]struct{})}, nil
}

// LoadDefault loads ~/.omo/omo.jsonc via os.UserHomeDir.
func LoadDefault() (*Editor, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolving home directory: %w", err)
	}
	return Load(filepath.Join(home, ".omo", "omo.jsonc"))
}

// Path returns the file the editor was loaded from.
func (e *Editor) Path() string {
	return e.path
}

var sectionOrder = []SectionID{
	SectionModels,
	SectionModelProfiles,
	SectionAgents,
	SectionCategories,
	SectionTelemetry,
	SectionGitMaster,
}

var sectionTitles = map[SectionID]string{
	SectionModels:        "Models",
	SectionModelProfiles: "Model Profiles",
	SectionAgents:        "Agents",
	SectionCategories:    "Categories",
	SectionTelemetry:     "Telemetry",
	SectionGitMaster:     "Git",
}

var sectionRoots = map[SectionID]string{
	SectionModels:        "/models",
	SectionModelProfiles: "/model_profiles",
	SectionAgents:        "/agents",
	SectionCategories:    "/categories",
	SectionTelemetry:     "/telemetry",
	SectionGitMaster:     "/git_master",
}

// editableSections reports whether a top-level key has an editable section.
// Every other present top-level key renders as its own read-only section.
var editableSections = map[string]struct{}{
	"models":         {},
	"model_profiles": {},
	"agents":         {},
	"categories":     {},
	"telemetry":      {},
	"git_master":     {},
}

// gitMasterKeys are the fixed toggle rows of the Git section.
var gitMasterKeys = []string{"commit_footer", "include_co_authored_by"}

// Sections returns the editable sections in display order, rebuilt from
// the current document, followed by one read-only section per remaining
// present top-level key (sorted by key).
func (e *Editor) Sections() []Section {
	sections := make([]Section, 0, len(sectionOrder))
	for _, id := range sectionOrder {
		section := Section{ID: id, Title: sectionTitles[id]}
		switch id {
		case SectionTelemetry:
			section.Entries = []Entry{e.telemetryEntry()}
		case SectionGitMaster:
			section.Entries = e.gitMasterEntries()
		default:
			section.Entries = e.blockEntries(id)
		}
		sections = append(sections, section)
	}
	sections = append(sections, e.readOnlySections()...)
	return sections
}

// blockEntries lists the members of one section block in key order.
func (e *Editor) blockEntries(section SectionID) []Entry {
	raw, found := e.doc.Get(sectionRoots[section])
	if !found {
		return []Entry{}
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return []Entry{}
	}
	entries := make([]Entry, 0, len(obj))
	for _, key := range sortedKeys(obj) {
		path := sectionRoots[section] + "/" + escapeToken(key)
		entries = append(entries, Entry{
			Key:   key,
			Path:  path,
			Kind:  KindAlias,
			Value: entryDisplayValue(section, obj[key]),
			Dirty: e.isDirtyAt(path),
		})
	}
	return entries
}

// entryDisplayValue is the display string of one section entry.
func entryDisplayValue(section SectionID, raw any) string {
	obj, ok := raw.(map[string]any)
	if !ok {
		if s, ok := raw.(string); ok {
			return s
		}
		return ""
	}
	if section == SectionModelProfiles {
		return stringField(obj, "display_name")
	}
	return stringField(obj, "model")
}

// telemetryEntry is the single telemetry switch; it is shown even when the
// block is missing, defaulting to false.
func (e *Editor) telemetryEntry() Entry {
	value := "false"
	if raw, found := e.doc.Get("/telemetry/enabled"); found {
		if b, ok := raw.(bool); ok {
			value = strconv.FormatBool(b)
		}
	}
	return Entry{
		Key:   "enabled",
		Path:  "/telemetry/enabled",
		Kind:  KindBool,
		Value: value,
		Dirty: e.isDirtyAt("/telemetry/enabled"),
	}
}

// gitMasterEntries are the two git_master switches; they are shown even
// when the block is missing, defaulting to false, so the first
// space-toggle can create the block.
func (e *Editor) gitMasterEntries() []Entry {
	entries := make([]Entry, 0, len(gitMasterKeys))
	for _, key := range gitMasterKeys {
		path := "/git_master/" + key
		value := "false"
		if raw, found := e.doc.Get(path); found {
			if b, ok := raw.(bool); ok {
				value = strconv.FormatBool(b)
			}
		}
		entries = append(entries, Entry{
			Key:   key,
			Path:  path,
			Kind:  KindBool,
			Value: value,
			Dirty: e.isDirtyAt(path),
		})
	}
	return entries
}

// readOnlySections builds one read-only section per present top-level key
// outside the editable surface, in sorted key order. Each section carries
// a single entry previewing the key's value; Detail expands it into
// read-only lines.
func (e *Editor) readOnlySections() []Section {
	raw, found := e.doc.Get("")
	if !found {
		return nil
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	var sections []Section
	for _, key := range sortedKeys(obj) {
		if _, editable := editableSections[key]; editable {
			continue
		}
		path := "/" + escapeToken(key)
		value := obj[key]
		kind := KindScalar
		if isBool(value) {
			kind = KindBool
		}
		sections = append(sections, Section{
			ID:    SectionID(key),
			Title: key,
			Entries: []Entry{{
				Key:      key,
				Path:     path,
				Kind:     kind,
				Value:    displayValue(value),
				Dirty:    e.isDirtyAt(path),
				ReadOnly: true,
			}},
			ReadOnly: true,
		})
	}
	return sections
}

// fieldSpec describes one detail line of a section entry.
type fieldSpec struct {
	label   string
	boolean bool
	chain   bool
}

var detailFields = map[SectionID][]fieldSpec{
	SectionModels: {
		{label: "model"},
		{label: "reasoning"},
	},
	SectionModelProfiles: {
		{label: "display_name"},
		{label: "models", chain: true},
	},
	SectionAgents: {
		{label: "model"},
		{label: "models", chain: true},
		{label: "reasoning"},
		{label: "disable", boolean: true},
	},
	SectionCategories: {
		{label: "model"},
		{label: "models", chain: true},
	},
}

// Detail returns the field lines of one entry. Unknown sections and
// missing entries are errors; fields outside the editable surface are
// listed as read-only lines.
func (e *Editor) Detail(section SectionID, key string) (Detail, error) {
	root, ok := sectionRoots[section]
	if !ok {
		return e.readOnlyDetail(string(section), key)
	}
	if section == SectionTelemetry {
		if key != "enabled" {
			return Detail{}, fmt.Errorf("no telemetry entry %q", key)
		}
		value := "false"
		if raw, found := e.doc.Get("/telemetry/enabled"); found {
			if b, ok := raw.(bool); ok {
				value = strconv.FormatBool(b)
			}
		}
		return Detail{
			Title: "enabled",
			Lines: []DetailLine{{
				Label:    "enabled",
				Path:     "/telemetry/enabled",
				Value:    value,
				Bool:     true,
				Editable: true,
			}},
		}, nil
	}
	if section == SectionGitMaster {
		return e.gitMasterDetail(key)
	}
	path := root + "/" + escapeToken(key)
	raw, found := e.doc.Get(path)
	if !found {
		return Detail{}, fmt.Errorf("no %s entry %q", section, key)
	}
	obj, isObject := raw.(map[string]any)
	if !isObject {
		if s, ok := raw.(string); !ok || s != "" {
			return Detail{
				Title: key,
				Lines: []DetailLine{{Label: key, Path: path, Value: displayValue(raw)}},
			}, nil
		}
		obj = map[string]any{}
	}
	specs := detailFields[section]
	lines := make([]DetailLine, 0, len(specs)+len(obj))
	seen := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		seen[spec.label] = struct{}{}
		line := DetailLine{
			Label:    spec.label,
			Path:     path + "/" + escapeToken(spec.label),
			Editable: true,
			Bool:     spec.boolean,
		}
		value, present := obj[spec.label]
		switch {
		case spec.boolean:
			if b, ok := value.(bool); ok {
				line.Value = strconv.FormatBool(b)
			} else {
				line.Value = "false"
			}
		case spec.chain:
			if present {
				line.Value = displayValue(value)
			}
		default:
			if s, ok := value.(string); ok {
				line.Value = s
			} else if present {
				line.Value = displayValue(value)
			}
		}
		lines = append(lines, line)
	}
	for _, name := range sortedKeys(obj) {
		if _, ok := seen[name]; ok {
			continue
		}
		value := obj[name]
		lines = append(lines, DetailLine{
			Label: name,
			Path:  path + "/" + escapeToken(name),
			Value: displayValue(value),
			Bool:  isBool(value),
		})
	}
	return Detail{Title: key, Lines: lines}, nil
}

// gitMasterDetail is the detail view of one Git toggle row. Unknown keys
// are errors; the value defaults to false when the block or key is
// absent.
func (e *Editor) gitMasterDetail(key string) (Detail, error) {
	known := false
	for _, name := range gitMasterKeys {
		if key == name {
			known = true
			break
		}
	}
	if !known {
		return Detail{}, fmt.Errorf("no git_master entry %q", key)
	}
	path := "/git_master/" + key
	value := "false"
	if raw, found := e.doc.Get(path); found {
		if b, ok := raw.(bool); ok {
			value = strconv.FormatBool(b)
		}
	}
	return Detail{
		Title: key,
		Lines: []DetailLine{{
			Label:    key,
			Path:     path,
			Value:    value,
			Bool:     true,
			Editable: true,
		}},
	}, nil
}

// readOnlyDetail is the detail view of one read-only top-level key.
// Object values expand to one read-only line per member in sorted key
// order; every other JSON type renders as a single read-only line
// previewing the value. Unknown keys are errors.
func (e *Editor) readOnlyDetail(topKey, key string) (Detail, error) {
	if _, editable := editableSections[topKey]; editable {
		return Detail{}, fmt.Errorf("unknown section %q", SectionID(topKey))
	}
	path := "/" + escapeToken(topKey)
	raw, found := e.doc.Get(path)
	if !found {
		return Detail{}, fmt.Errorf("unknown section %q", SectionID(topKey))
	}
	if key != topKey {
		return Detail{}, fmt.Errorf("no %s entry %q", topKey, key)
	}
	if obj, ok := raw.(map[string]any); ok {
		lines := make([]DetailLine, 0, len(obj))
		for _, name := range sortedKeys(obj) {
			value := obj[name]
			lines = append(lines, DetailLine{
				Label: name,
				Path:  path + "/" + escapeToken(name),
				Value: displayValue(value),
				Bool:  isBool(value),
			})
		}
		return Detail{Title: key, Lines: lines}, nil
	}
	return Detail{
		Title: key,
		Lines: []DetailLine{{
				Label: key,
				Path:  path,
				Value: displayValue(raw),
				Bool:  isBool(raw),
			}},
	}, nil
}

// stringField returns the named string member of obj, or "".
func stringField(obj map[string]any, name string) string {
	s, _ := obj[name].(string)
	return s
}

// displayValue renders a decoded JSON value as a display string.
func displayValue(raw any) string {
	switch value := raw.(type) {
	case nil:
		return ""
	case string:
		return value
	case bool:
		return strconv.FormatBool(value)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	default:
		encoded, err := json.Marshal(raw)
		if err != nil {
			return ""
		}
		return string(encoded)
	}
}

func isBool(raw any) bool {
	_, ok := raw.(bool)
	return ok
}

// sortedKeys returns the keys of m in sorted order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// parsePointer splits an RFC 6901 JSON Pointer into unescaped tokens.
// The empty string denotes the whole document.
func parsePointer(pointer string) ([]string, error) {
	if pointer == "" {
		return nil, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("must be empty or start with '/'")
	}
	parts := strings.Split(pointer[1:], "/")
	for i, part := range parts {
		unescaped, err := unescapeToken(part)
		if err != nil {
			return nil, err
		}
		parts[i] = unescaped
	}
	return parts, nil
}

// unescapeToken decodes ~1 to '/' and ~0 to '~'.
func unescapeToken(token string) (string, error) {
	for i := 0; i < len(token); i++ {
		if token[i] == '~' {
			if i+1 >= len(token) || (token[i+1] != '0' && token[i+1] != '1') {
				return "", fmt.Errorf("bad escape in %q", token)
			}
			i++
		}
	}
	token = strings.ReplaceAll(token, "~1", "/")
	return strings.ReplaceAll(token, "~0", "~"), nil
}

// escapeToken encodes one pointer token per RFC 6901.
func escapeToken(token string) string {
	token = strings.ReplaceAll(token, "~", "~0")
	return strings.ReplaceAll(token, "/", "~1")
}
