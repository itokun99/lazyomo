// Package omodit loads, edits, and saves JSONC configuration files
// (for example ~/.omo/omo.jsonc) while preserving comments, key order,
// and formatting outside the edited subtree.
//
// It is built on github.com/tailscale/hujson (JWCC format): the file is
// parsed once into an exact syntax tree where comments and whitespace are
// kept as trivia, mutations are applied as RFC 6902 patches, and the tree
// is packed back to bytes that are identical to the input wherever nothing
// changed.
package omodit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tailscale/hujson"
)

// backupTimeLayout formats backup timestamps in UTC as
// <path>.bak.<YYYY-MM-DDTHH-mm-ss-SSSZ>, for example
// omo.jsonc.bak.2026-10-05T12-34-56-789Z.
const backupTimeLayout = "2006-01-02T15-04-05-000Z"

// Document is a parsed JSONC configuration file held in memory.
// The zero value is not usable; construct with Load.
type Document struct {
	path string
	root hujson.Value
}

// Load reads the file at path and parses it as JSONC.
// Comments (// and /* */) and trailing commas are preserved in memory.
// Errors are wrapped as "loading config <path>: <cause>".
func Load(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading config %s: %w", path, err)
	}
	root, err := hujson.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("loading config %s: %w", path, err)
	}
	return &Document{path: path, root: root}, nil
}

// Bytes returns the current serialized JSONC content of the document.
// Untouched regions are byte-identical to the loaded file.
func (d *Document) Bytes() []byte {
	return d.root.Pack()
}

// Get returns the value at the RFC 6901 JSON Pointer (for example
// "/models/k3/reasoning"). The returned value is decoded via json.Unmarshal
// of the packed subtree. The second result reports whether the pointer was
// found; an invalid pointer reports found=false.
func (d *Document) Get(pointer string) (any, bool) {
	if _, err := parsePointer(pointer); err != nil {
		return nil, false
	}
	sub := d.root.Find(pointer)
	if sub == nil {
		return nil, false
	}
	clone := sub.Clone()
	clone.Standardize()
	var out any
	if err := json.Unmarshal(clone.Pack(), &out); err != nil {
		return nil, false
	}
	return out, true
}

// Set replaces the existing value at the RFC 6901 JSON Pointer with value.
// It returns an error if the pointer is absent or invalid.
func (d *Document) Set(pointer string, value any) error {
	if _, err := parsePointer(pointer); err != nil {
		return fmt.Errorf("setting value at %q: %w", pointer, err)
	}
	if pointer == "" {
		valueJSON, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("setting value at %q: %w", pointer, err)
		}
		parsed, err := hujson.Parse(valueJSON)
		if err != nil {
			return fmt.Errorf("setting value at %q: %w", pointer, err)
		}
		d.root.Value = parsed.Value
		return nil
	}
	if d.root.Find(pointer) == nil {
		return fmt.Errorf("setting value at %q: pointer not found", pointer)
	}
	if err := d.patch("replace", pointer, value); err != nil {
		return fmt.Errorf("setting value at %q: %w", pointer, err)
	}
	return nil
}

// Add inserts value at the RFC 6901 JSON Pointer as a new object member or
// array element. It returns an error if the pointer already exists or the
// pointer (or its parent) is invalid. The "-" array token and an index
// equal to the array length both append. The new member or element is
// indented to match its siblings and keeps the trailing-comma style of
// the array or object it joins.
func (d *Document) Add(pointer string, value any) error {
	tokens, err := parsePointer(pointer)
	if err != nil {
		return fmt.Errorf("adding value at %q: %w", pointer, err)
	}
	if len(tokens) == 0 {
		return fmt.Errorf("adding value at %q: value already exists", pointer)
	}
	if d.root.Find(pointer) != nil {
		return fmt.Errorf("adding value at %q: value already exists", pointer)
	}
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("adding value at %q: %w", pointer, err)
	}
	parsed, err := hujson.Parse(valueJSON)
	if err != nil {
		return fmt.Errorf("adding value at %q: %w", pointer, err)
	}
	parentPointer := ""
	if len(tokens) > 1 {
		escaped := make([]string, len(tokens)-1)
		for i, tok := range tokens[:len(tokens)-1] {
			escaped[i] = escapeToken(tok)
		}
		parentPointer = "/" + strings.Join(escaped, "/")
	}
	parent := d.root.Find(parentPointer)
	if parent == nil {
		return fmt.Errorf("adding value at %q: parent %q not found", pointer, parentPointer)
	}
	lastToken := tokens[len(tokens)-1]
	switch composite := parent.Value.(type) {
	case *hujson.Object:
		for _, m := range composite.Members {
			if memberNameEquals(m.Name, lastToken) {
				return fmt.Errorf("adding value at %q: value already exists", pointer)
			}
		}
		appendObjectMember(composite, lastToken, &parsed)
		return nil
	case *hujson.Array:
		if lastToken != "-" {
			n, err := arrayIndex(lastToken)
			if err != nil {
				return fmt.Errorf("adding value at %q: %w", pointer, err)
			}
			if n < len(composite.Elements) {
				return fmt.Errorf("adding value at %q: value already exists", pointer)
			}
			if n > len(composite.Elements) {
				return fmt.Errorf("adding value at %q: array index out of range", pointer)
			}
		}
		appendArrayElement(composite, &parsed)
		return nil
	default:
		return fmt.Errorf("adding value at %q: parent is not an object or array", pointer)
	}
}

// Remove deletes the value at the RFC 6901 JSON Pointer.
// It returns an error if the pointer is absent or invalid.
func (d *Document) Remove(pointer string) error {
	if _, err := parsePointer(pointer); err != nil {
		return fmt.Errorf("removing value at %q: %w", pointer, err)
	}
	if pointer == "" {
		return fmt.Errorf("removing value at %q: cannot remove root value", pointer)
	}
	if d.root.Find(pointer) == nil {
		return fmt.Errorf("removing value at %q: pointer not found", pointer)
	}
	patchBytes, err := json.Marshal([]map[string]any{
		{"op": "remove", "path": pointer},
	})
	if err != nil {
		return fmt.Errorf("removing value at %q: %w", pointer, err)
	}
	if err := d.root.Patch(patchBytes); err != nil {
		return fmt.Errorf("removing value at %q: %w", pointer, err)
	}
	return nil
}

// Save writes the document back to the path it was loaded from atomically:
// a temp file is written in the same directory, fsynced, chmodded to the
// existing file mode, and renamed over the target. Before overwriting an
// existing target, the original bytes are copied to a backup named
// <path>.bak.<YYYY-MM-DDTHH-mm-ss-SSSZ> (UTC); if that name is taken,
// -2/-3 suffixes are appended until free. If the target does not exist,
// it is created with mode 0o644 and no backup is made.
func (d *Document) Save() error {
	data := d.root.Pack()
	dir := filepath.Dir(d.path)

	info, err := os.Stat(d.path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("saving config: %w", err)
	}
	if os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		if err := writeTempRename(dir, d.path, data, 0o644); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		return nil
	}

	mode := info.Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}
	original, err := os.ReadFile(d.path)
	if err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	backupPath, err := freeBackupPath(d.path)
	if err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	if err := os.WriteFile(backupPath, original, mode); err != nil {
		return fmt.Errorf("creating backup: %w", err)
	}
	if err := writeTempRename(dir, d.path, data, mode); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	return nil
}

// patch applies a single RFC 6902 add/replace operation carrying value.
func (d *Document) patch(op, pointer string, value any) error {
	patchBytes, err := json.Marshal([]map[string]any{
		{"op": op, "path": pointer, "value": value},
	})
	if err != nil {
		return err
	}
	return d.root.Patch(patchBytes)
}

// parsePointer splits an RFC 6901 JSON Pointer into unescaped tokens.
// The empty string denotes the whole document. Each "~1" becomes "/"
// and each "~0" becomes "~"; any other "~" sequence is invalid.
func parsePointer(pointer string) ([]string, error) {
	if pointer == "" {
		return nil, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("invalid JSON pointer %q: must be empty or start with '/'", pointer)
	}
	parts := strings.Split(pointer[1:], "/")
	for i, part := range parts {
		for j := 0; j < len(part); j++ {
			if part[j] == '~' {
				if j+1 >= len(part) || (part[j+1] != '0' && part[j+1] != '1') {
					return nil, fmt.Errorf("invalid JSON pointer %q: bad escape in %q", pointer, part)
				}
				j++
			}
		}
		part = strings.ReplaceAll(part, "~1", "/")
		part = strings.ReplaceAll(part, "~0", "~")
		parts[i] = part
	}
	return parts, nil
}

// escapeToken encodes one pointer token per RFC 6901 (~ becomes ~0,
// then / becomes ~1).
func escapeToken(token string) string {
	token = strings.ReplaceAll(token, "~", "~0")
	return strings.ReplaceAll(token, "/", "~1")
}

// memberNameEquals reports whether an object member name literal decodes
// to the given pointer token.
func memberNameEquals(name hujson.Value, token string) bool {
	lit, ok := name.Value.(hujson.Literal)
	if !ok {
		return false
	}
	var decoded string
	if err := json.Unmarshal(lit, &decoded); err != nil {
		return false
	}
	return decoded == token
}

// arrayIndex parses a decimal array index token.
func arrayIndex(token string) (int, error) {
	if token == "" {
		return 0, fmt.Errorf("invalid array index %q", token)
	}
	for i := 0; i < len(token); i++ {
		if token[i] < '0' || token[i] > '9' {
			return 0, fmt.Errorf("invalid array index %q", token)
		}
	}
	n, err := strconv.Atoi(token)
	if err != nil {
		return 0, fmt.Errorf("invalid array index %q", token)
	}
	return n, nil
}

// siblingIndent derives the trivia placed before a new sibling from a
// neighboring trivia block: the text from its last newline onward, so the
// new member or element starts on its own line at the same indent. When
// the reference holds no newline, whitespace-only trivia is reused as is
// and anything else yields no separator.
func siblingIndent(ref string) string {
	if i := strings.LastIndexByte(ref, '\n'); i >= 0 {
		return ref[i:]
	}
	if strings.TrimSpace(ref) == "" {
		return ref
	}
	return ""
}

// appendObjectMember appends a new member styled after existing members:
// the name starts on its own indented line, the value keeps the sibling
// colon spacing, and the trailing comma follows the previous style.
func appendObjectMember(obj *hujson.Object, name string, value *hujson.Value) {
	nameBefore := hujson.Extra("")
	valueBefore := hujson.Extra(" ")
	var valueAfter hujson.Extra
	if len(obj.Members) > 0 {
		last := &obj.Members[len(obj.Members)-1]
		nameBefore = hujson.Extra(siblingIndent(string(last.Name.BeforeExtra)))
		valueBefore = append(hujson.Extra(nil), last.Value.BeforeExtra...)
		if last.Value.AfterExtra != nil {
			valueAfter = hujson.Extra{}
		}
	} else if len(obj.AfterExtra) > 0 {
		nameBefore = hujson.Extra(siblingIndent(string(obj.AfterExtra)))
	}
	nameValue := hujson.Value{BeforeExtra: nameBefore, Value: hujson.String(name)}
	value.BeforeExtra = valueBefore
	value.AfterExtra = valueAfter
	obj.Members = append(obj.Members, hujson.ObjectMember{Name: nameValue, Value: *value})
}

// appendArrayElement appends a new element styled after existing elements.
func appendArrayElement(arr *hujson.Array, value *hujson.Value) {
	var before hujson.Extra
	var after hujson.Extra
	if len(arr.Elements) > 0 {
		last := &arr.Elements[len(arr.Elements)-1]
		before = hujson.Extra(siblingIndent(string(last.BeforeExtra)))
		if last.AfterExtra != nil {
			after = hujson.Extra{}
		}
	} else if len(arr.AfterExtra) > 0 {
		before = hujson.Extra(siblingIndent(string(arr.AfterExtra)))
	}
	value.BeforeExtra = before
	value.AfterExtra = after
	arr.Elements = append(arr.Elements, *value)
}

// freeBackupPath returns the backup name for path with a UTC timestamp,
// appending -2/-3 suffixes while the name is already taken.
func freeBackupPath(path string) (string, error) {
	stamp := time.Now().UTC().Format(backupTimeLayout)
	base := path + ".bak." + stamp
	candidate := base
	for i := 2; ; i++ {
		_, err := os.Stat(candidate)
		if os.IsNotExist(err) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
}

// writeTempRename writes data to a temp file in dir, fsyncs it, sets mode,
// and renames it over target. No temp file is left behind on success or
// failure.
func writeTempRename(dir, target string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(dir, ".omodit-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return err
	}
	if err := os.Rename(tmpName, target); err != nil {
		return err
	}
	return nil
}
