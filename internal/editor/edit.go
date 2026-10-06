package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/itokun99/lazyomo/internal/omodit"
)

// SetScalar writes value to an editable scalar or chain path:
//
//	/models/<alias>/model                 <provider>/<model-id>, non-empty
//	/models/<alias>/reasoning             low|medium|high|max, or "" to unset
//	/model_profiles/<name>/display_name   non-empty, at most 64 characters
//	/model_profiles/<name>/models         whole chain
//	/model_profiles/<name>/models/<i>     one chain element
//	/model_profile                        existing model_profiles key or <provider>/<model-id> pin, or "" to clear
//	/agents/<name>/model                  existing alias or <provider>/<model-id>
//	/agents/<name>/models[/<i>]           chain or element
//	/agents/<name>/reasoning              low|medium|high|max, or "" to unset
//	/categories/<name>/model              existing alias or <provider>/<model-id>
//	/categories/<name>/models[/<i>]       chain or element
//
// A whole chain is either a JSON array of strings or a comma-separated
// list. Invalid paths and values return an inline-displayable error and
// leave the document untouched. Boolean paths are written with ToggleBool.
func (e *Editor) SetScalar(path, value string) error {
	tokens, err := parsePointer(path)
	if err != nil {
		return fmt.Errorf("invalid path %q: %w", path, err)
	}
	switch {
	case len(tokens) == 3 && tokens[0] == "models" && tokens[2] == "model":
		return e.setModel(tokens[1], value)
	case len(tokens) == 3 && tokens[0] == "models" && tokens[2] == "reasoning":
		return e.setReasoning(SectionModels, tokens[1], value)
	case len(tokens) == 3 && tokens[0] == "model_profiles" && tokens[2] == "display_name":
		return e.setDisplayName(tokens[1], value)
	case len(tokens) == 3 && tokens[0] == "model_profiles" && tokens[2] == "models":
		return e.setChain(SectionModelProfiles, tokens[1], value)
	case len(tokens) == 4 && tokens[0] == "model_profiles" && tokens[2] == "models":
		return e.setChainElement(SectionModelProfiles, tokens[1], tokens[3], value)
	case len(tokens) == 1 && tokens[0] == "model_profile":
		return e.setModelProfile(value)
	case len(tokens) == 3 && tokens[0] == "agents" && tokens[2] == "model":
		return e.setModelReference(SectionAgents, tokens[1], value)
	case len(tokens) == 3 && tokens[0] == "agents" && tokens[2] == "models":
		return e.setChain(SectionAgents, tokens[1], value)
	case len(tokens) == 4 && tokens[0] == "agents" && tokens[2] == "models":
		return e.setChainElement(SectionAgents, tokens[1], tokens[3], value)
	case len(tokens) == 3 && tokens[0] == "agents" && tokens[2] == "reasoning":
		return e.setReasoning(SectionAgents, tokens[1], value)
	case len(tokens) == 3 && tokens[0] == "categories" && tokens[2] == "model":
		return e.setModelReference(SectionCategories, tokens[1], value)
	case len(tokens) == 3 && tokens[0] == "categories" && tokens[2] == "models":
		return e.setChain(SectionCategories, tokens[1], value)
	case len(tokens) == 4 && tokens[0] == "categories" && tokens[2] == "models":
		return e.setChainElement(SectionCategories, tokens[1], tokens[3], value)
	}
	return fmt.Errorf("path %q is not editable", path)
}

func (e *Editor) setModel(key, value string) error {
	if !validModelRef(value) {
		return fmt.Errorf("invalid model %q: want <provider>/<model-id>", value)
	}
	return e.setEntryField(SectionModels, key, "model", value)
}

func (e *Editor) setModelReference(section SectionID, key, value string) error {
	if !validModelRef(value) && !e.aliasExists(value) {
		return fmt.Errorf("invalid model %q: want an existing alias or <provider>/<model-id>", value)
	}
	return e.setEntryField(section, key, "model", value)
}

func (e *Editor) aliasExists(key string) bool {
	_, ok := e.getObject("/models/" + escapeToken(key))
	return ok
}

func (e *Editor) setReasoning(section SectionID, key, value string) error {
	if value != "" && !validReasoning(value) {
		return fmt.Errorf("invalid reasoning %q: want one of low, medium, high, max", value)
	}
	entryPath := e.entryPath(section, key)
	if _, found := e.doc.Get(entryPath); !found {
		return fmt.Errorf("no %s entry %q", section, key)
	}
	if value == "" {
		fieldPath := entryPath + "/reasoning"
		if _, found := e.doc.Get(fieldPath); found {
			if err := e.doc.Remove(fieldPath); err != nil {
				return fmt.Errorf("unsetting %q: %w", fieldPath, err)
			}
			e.markDirty(fieldPath)
		}
		return nil
	}
	return e.setEntryField(section, key, "reasoning", value)
}

func (e *Editor) setDisplayName(key, value string) error {
	if err := validateDisplayName(value); err != nil {
		return err
	}
	return e.setEntryField(SectionModelProfiles, key, "display_name", value)
}

func (e *Editor) setModelProfile(value string) error {
	if value == "" {
		profiles, ok := e.getObject("/model_profiles")
		if ok && len(profiles) > 0 {
			return fmt.Errorf("clearing model_profile is blocked while %d model profiles exist", len(profiles))
		}
		if _, found := e.doc.Get("/model_profile"); found {
			if err := e.doc.Remove("/model_profile"); err != nil {
				return fmt.Errorf("clearing model_profile: %w", err)
			}
			e.markDirty("/model_profile")
		}
		return nil
	}
	if profiles, ok := e.getObject("/model_profiles"); ok {
		if _, ok := profiles[value]; ok {
			return e.writeModelProfile(value)
		}
	}
	// Literal provider/model pins never name a profile key: accept any
	// value matching modelRefPattern (which always contains a slash).
	if validModelRef(value) {
		return e.writeModelProfile(value)
	}
	return fmt.Errorf("unknown model profile %q", value)
}

// writeModelProfile stores value at /model_profile, adding the key when
// it is absent.
func (e *Editor) writeModelProfile(value string) error {
	if _, found := e.doc.Get("/model_profile"); found {
		if err := e.doc.Set("/model_profile", value); err != nil {
			return fmt.Errorf("setting model_profile: %w", err)
		}
	} else {
		if err := e.doc.Add("/model_profile", value); err != nil {
			return fmt.Errorf("setting model_profile: %w", err)
		}
	}
	e.markDirty("/model_profile")
	return nil
}

func (e *Editor) setChain(section SectionID, key, value string) error {
	elements, err := parseChain(value)
	if err != nil {
		return err
	}
	return e.setEntryField(section, key, "models", elements)
}

func (e *Editor) setChainElement(section SectionID, key, indexToken, value string) error {
	index, err := strconv.Atoi(indexToken)
	if err != nil || index < 0 {
		return fmt.Errorf("invalid chain index %q", indexToken)
	}
	if !validChainElement(value) {
		return fmt.Errorf("invalid chain entry %q: want <provider>/<model-id>[:suffix] or <alias>[:suffix]", value)
	}
	fieldPath := e.entryPath(section, key) + "/models"
	raw, found := e.doc.Get(fieldPath)
	if !found {
		return fmt.Errorf("no chain at %q", fieldPath)
	}
	arr, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("value at %q is not a chain", fieldPath)
	}
	if index >= len(arr) {
		return fmt.Errorf("chain index %d out of range for %q (length %d)", index, fieldPath, len(arr))
	}
	updated := make([]any, len(arr))
	copy(updated, arr)
	updated[index] = value
	if err := e.doc.Set(fieldPath, updated); err != nil {
		return fmt.Errorf("setting %q: %w", fieldPath, err)
	}
	e.markDirty(fieldPath)
	return nil
}

// entryPath is the section root plus the escaped key.
func (e *Editor) entryPath(section SectionID, key string) string {
	return sectionRoots[section] + "/" + escapeToken(key)
}

// getObject returns the object at pointer, reporting whether it exists and
// decodes as an object.
func (e *Editor) getObject(pointer string) (map[string]any, bool) {
	raw, found := e.doc.Get(pointer)
	if !found {
		return nil, false
	}
	obj, ok := raw.(map[string]any)
	return obj, ok
}

// setEntryField writes one object field of a section entry, materializing
// the object when the entry is still an AddEntry placeholder.
func (e *Editor) setEntryField(section SectionID, key, field string, value any) error {
	entryPath := e.entryPath(section, key)
	raw, found := e.doc.Get(entryPath)
	if !found {
		return fmt.Errorf("no %s entry %q", section, key)
	}
	fieldPath := entryPath + "/" + escapeToken(field)
	if _, isObject := raw.(map[string]any); !isObject {
		placeholder, isString := raw.(string)
		if !isString || placeholder != "" {
			return fmt.Errorf("entry %q is not an editable object", key)
		}
		if err := e.doc.Set(entryPath, map[string]any{field: value}); err != nil {
			return fmt.Errorf("setting %q: %w", fieldPath, err)
		}
		e.markDirty(fieldPath)
		return nil
	}
	if _, found := e.doc.Get(fieldPath); found {
		if err := e.doc.Set(fieldPath, value); err != nil {
			return fmt.Errorf("setting %q: %w", fieldPath, err)
		}
	} else {
		if err := e.doc.Add(fieldPath, value); err != nil {
			return fmt.Errorf("adding %q: %w", fieldPath, err)
		}
	}
	e.markDirty(fieldPath)
	return nil
}

// ToggleBool flips a boolean at a known boolean path:
//
//	/telemetry/enabled
//	/agents/<name>/disable
//	/git_master/commit_footer
//	/git_master/include_co_authored_by
//
// A missing telemetry or git_master block is created with the toggled key
// set; a missing agent disable is added to the agent entry. Unknown paths
// and non-boolean values are errors.
func (e *Editor) ToggleBool(path string) error {
	tokens, err := parsePointer(path)
	if err != nil {
		return fmt.Errorf("invalid path %q: %w", path, err)
	}
	switch {
	case len(tokens) == 2 && tokens[0] == "telemetry" && tokens[1] == "enabled":
		return e.toggleField("/telemetry/enabled", "/telemetry", "enabled")
	case len(tokens) == 3 && tokens[0] == "agents" && tokens[2] == "disable":
		return e.toggleEntryBool(SectionAgents, tokens[1])
	case len(tokens) == 2 && tokens[0] == "git_master" &&
		(tokens[1] == "commit_footer" || tokens[1] == "include_co_authored_by"):
		return e.toggleField(path, "/git_master", tokens[1])
	}
	return fmt.Errorf("unknown boolean path %q", path)
}

func (e *Editor) toggleField(fieldPath, blockPath, blockKey string) error {
	raw, found := e.doc.Get(fieldPath)
	if found {
		current, ok := raw.(bool)
		if !ok {
			return fmt.Errorf("value at %q is not a boolean", fieldPath)
		}
		if err := e.doc.Set(fieldPath, !current); err != nil {
			return fmt.Errorf("toggling %q: %w", fieldPath, err)
		}
		e.markDirty(fieldPath)
		return nil
	}
	if _, found := e.doc.Get(blockPath); !found {
		if err := e.doc.Add(blockPath, map[string]any{blockKey: true}); err != nil {
			return fmt.Errorf("creating %s: %w", blockPath, err)
		}
	} else {
		if err := e.doc.Add(fieldPath, true); err != nil {
			return fmt.Errorf("adding %q: %w", fieldPath, err)
		}
	}
	e.markDirty(fieldPath)
	return nil
}

func (e *Editor) toggleEntryBool(section SectionID, key string) error {
	entryPath := e.entryPath(section, key)
	raw, found := e.doc.Get(entryPath)
	if !found {
		return fmt.Errorf("no %s entry %q", section, key)
	}
	fieldPath := entryPath + "/disable"
	if current, found := e.doc.Get(fieldPath); found {
		b, ok := current.(bool)
		if !ok {
			return fmt.Errorf("value at %q is not a boolean", fieldPath)
		}
		if err := e.doc.Set(fieldPath, !b); err != nil {
			return fmt.Errorf("toggling %q: %w", fieldPath, err)
		}
		e.markDirty(fieldPath)
		return nil
	}
	if _, isObject := raw.(map[string]any); !isObject {
		placeholder, isString := raw.(string)
		if !isString || placeholder != "" {
			return fmt.Errorf("entry %q is not an editable object", key)
		}
		if err := e.doc.Set(entryPath, map[string]any{"disable": true}); err != nil {
			return fmt.Errorf("toggling %q: %w", fieldPath, err)
		}
	} else {
		if err := e.doc.Add(fieldPath, true); err != nil {
			return fmt.Errorf("adding %q: %w", fieldPath, err)
		}
	}
	e.markDirty(fieldPath)
	return nil
}

// AddEntry adds a new entry to a section with an empty string placeholder
// value. Keys follow the alias rules and must be unique in the section;
// categories are limited to the fixed set of ten names. Telemetry holds a
// single fixed switch and does not support adding entries.
func (e *Editor) AddEntry(section SectionID, key string) error {
	root, ok := sectionRoots[section]
	if !ok {
		return fmt.Errorf("unknown section %q", section)
	}
	if section == SectionTelemetry || section == SectionGitMaster {
		return fmt.Errorf("section %s does not support adding entries", section)
	}
	if err := validateKey(key); err != nil {
		return err
	}
	if section == SectionCategories {
		if _, ok := knownCategories[key]; !ok {
			return fmt.Errorf("unknown category %q: the category set is fixed", key)
		}
	}
	entryPath := root + "/" + escapeToken(key)
	if _, found := e.doc.Get(entryPath); found {
		return fmt.Errorf("entry %q already exists in %s", key, section)
	}
	if _, found := e.doc.Get(root); !found {
		if err := e.doc.Add(root, map[string]any{key: ""}); err != nil {
			return fmt.Errorf("adding entry %q to %s: %w", key, section, err)
		}
	} else {
		if err := e.doc.Add(entryPath, ""); err != nil {
			return fmt.Errorf("adding entry %q to %s: %w", key, section, err)
		}
	}
	e.markDirty(entryPath)
	return nil
}

// RemoveEntry removes a section entry after the reference-blocking rules
// of spec section 1: a models alias still referenced by agents, categories,
// or model_profiles chains cannot be removed, and a model_profiles profile
// selected by /model_profile cannot be removed. The error names the
// referrer.
func (e *Editor) RemoveEntry(section SectionID, key string) error {
	root, ok := sectionRoots[section]
	if !ok {
		return fmt.Errorf("unknown section %q", section)
	}
	if section == SectionTelemetry {
		if key == "enabled" {
			return fmt.Errorf("removing the telemetry enabled toggle is not supported")
		}
		return fmt.Errorf("no telemetry entry %q", key)
	}
	if section == SectionGitMaster {
		for _, name := range gitMasterKeys {
			if key == name {
				return fmt.Errorf("removing the git_master %s toggle is not supported", key)
			}
		}
		return fmt.Errorf("no git_master entry %q", key)
	}
	entryPath := root + "/" + escapeToken(key)
	if _, found := e.doc.Get(entryPath); !found {
		return fmt.Errorf("no %s entry %q", section, key)
	}
	switch section {
	case SectionModels:
		if referrer, ok := e.aliasReferrer(key); ok {
			return fmt.Errorf("cannot remove model alias %q: still referenced by %s", key, referrer)
		}
	case SectionModelProfiles:
		if selected, found := e.doc.Get("/model_profile"); found {
			if name, ok := selected.(string); ok && name == key {
				return fmt.Errorf("cannot remove model profile %q: selected by /model_profile", key)
			}
		}
	}
	if err := e.doc.Remove(entryPath); err != nil {
		return fmt.Errorf("removing %s entry %q: %w", section, key, err)
	}
	e.dropDirtyDescendants(entryPath)
	e.markDirty(entryPath)
	return nil
}

// aliasReferrer returns the first path referencing alias, scanning agents,
// categories, and model_profiles in that order.
func (e *Editor) aliasReferrer(alias string) (string, bool) {
	for _, section := range []SectionID{SectionAgents, SectionCategories, SectionModelProfiles} {
		root := sectionRoots[section]
		obj, ok := e.getObject(root)
		if !ok {
			continue
		}
		for _, name := range sortedKeys(obj) {
			entry, ok := obj[name].(map[string]any)
			if !ok {
				continue
			}
			if section != SectionModelProfiles {
				if model, ok := entry["model"].(string); ok && model == alias {
					return root + "/" + escapeToken(name) + "/model", true
				}
			}
			if list, ok := entry["models"].([]any); ok {
				for i, element := range list {
					if modelRefPart(element) == alias {
						return fmt.Sprintf("%s/%s/models/%d", root, escapeToken(name), i), true
					}
				}
			}
		}
	}
	return "", false
}

// modelRefPart is the model reference of a chain element, without its
// optional :suffix.
func modelRefPart(element any) string {
	s, ok := element.(string)
	if !ok {
		return ""
	}
	return strings.SplitN(s, ":", 2)[0]
}

// MoveChain moves the element at index from to index to in the chain at
// path (a "models" array on a profile, agent, or category). Bounds are
// validated before the splice; equal indexes are a no-op.
func (e *Editor) MoveChain(path string, from, to int) error {
	tokens, err := parsePointer(path)
	if err != nil {
		return fmt.Errorf("invalid path %q: %w", path, err)
	}
	if len(tokens) != 3 || tokens[2] != "models" ||
		(tokens[0] != "model_profiles" && tokens[0] != "agents" && tokens[0] != "categories") {
		return fmt.Errorf("path %q is not an editable chain", path)
	}
	raw, found := e.doc.Get(path)
	if !found {
		return fmt.Errorf("no chain at %q", path)
	}
	arr, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("value at %q is not a chain", path)
	}
	if from < 0 || from >= len(arr) {
		return fmt.Errorf("chain index %d out of range for %q (length %d)", from, path, len(arr))
	}
	if to < 0 || to >= len(arr) {
		return fmt.Errorf("chain index %d out of range for %q (length %d)", to, path, len(arr))
	}
	if from == to {
		return nil
	}
	updated := make([]any, 0, len(arr))
	updated = append(updated, arr[:from]...)
	updated = append(updated, arr[from+1:]...)
	updated = append(updated, nil)
	copy(updated[to+1:], updated[to:])
	updated[to] = arr[from]
	if err := e.doc.Set(path, updated); err != nil {
		return fmt.Errorf("setting %q: %w", path, err)
	}
	e.markDirty(path)
	return nil
}

// DirtyPaths returns the deduplicated paths changed since the last Save or
// Reload, in sorted order.
func (e *Editor) DirtyPaths() []string {
	paths := make([]string, 0, len(e.dirty))
	for path := range e.dirty {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// Save writes the document atomically through omodit and returns the path
// of the timestamped backup it created, or "" when the file did not exist
// before. Dirty state is cleared only on success. Saving is refused while
// a dirty models alias has no model.
func (e *Editor) Save() (string, error) {
	if err := e.checkDirtyModels(); err != nil {
		return "", err
	}
	before := e.backupNames()
	if err := e.doc.Save(); err != nil {
		return "", err
	}
	backup := newBackupPath(before, e.backupNames())
	e.dirty = make(map[string]struct{})
	return backup, nil
}

// checkDirtyModels refuses to save while a models alias touched since the
// last save has no model ("Refuse empty model on save", spec section 1).
func (e *Editor) checkDirtyModels() error {
	aliases := make(map[string]struct{})
	for path := range e.dirty {
		tokens, err := parsePointer(path)
		if err != nil || len(tokens) < 2 || tokens[0] != "models" {
			continue
		}
		aliases[tokens[1]] = struct{}{}
	}
	for _, key := range sortedKeys(aliases) {
		raw, found := e.doc.Get("/models/" + escapeToken(key))
		if !found {
			continue
		}
		obj, ok := raw.(map[string]any)
		if !ok || stringField(obj, "model") == "" {
			return fmt.Errorf("refusing to save: model alias %q has no model", key)
		}
	}
	return nil
}

// backupNames lists the backup files of the edited path.
func (e *Editor) backupNames() map[string]struct{} {
	dir := filepath.Dir(e.path)
	prefix := filepath.Base(e.path) + ".bak."
	names := make(map[string]struct{})
	entries, err := os.ReadDir(dir)
	if err != nil {
		return names
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
			names[filepath.Join(dir, entry.Name())] = struct{}{}
		}
	}
	return names
}

// newBackupPath is the backup name that appeared between two listings.
func newBackupPath(before, after map[string]struct{}) string {
	newest := ""
	for name := range after {
		if _, existed := before[name]; existed {
			continue
		}
		if name > newest {
			newest = name
		}
	}
	return newest
}

// Reload re-reads the file from disk and discards unsaved edits.
func (e *Editor) Reload() error {
	doc, err := omodit.Load(e.path)
	if err != nil {
		return err
	}
	e.doc = doc
	e.dirty = make(map[string]struct{})
	return nil
}

func (e *Editor) markDirty(path string) {
	e.dirty[path] = struct{}{}
}

// dropDirtyDescendants forgets dirty paths below path; they no longer
// exist once path is removed.
func (e *Editor) dropDirtyDescendants(path string) {
	prefix := path + "/"
	for p := range e.dirty {
		if strings.HasPrefix(p, prefix) {
			delete(e.dirty, p)
		}
	}
}

// isDirtyAt reports whether path itself or one of its descendants changed.
func (e *Editor) isDirtyAt(path string) bool {
	if _, ok := e.dirty[path]; ok {
		return true
	}
	prefix := path + "/"
	for p := range e.dirty {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}
