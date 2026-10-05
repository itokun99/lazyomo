package omodit_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/itokun99/lazyomo/internal/omodit"
)

const fixturePath = "testdata/sample.jsonc"

// copyFixture copies the fixture into a temp dir and returns its new path
// plus the original bytes.
func copyFixture(t *testing.T) (string, []byte) {
	t.Helper()
	original, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), "config.jsonc")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("copying fixture: %v", err)
	}
	return path, original
}

// loadCopy copies the fixture and loads the copy.
func loadCopy(t *testing.T) (*omodit.Document, string, []byte) {
	t.Helper()
	path, original := copyFixture(t)
	doc, err := omodit.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return doc, path, original
}

func assertContains(t *testing.T, haystack []byte, needles []string) {
	t.Helper()
	for _, needle := range needles {
		if !strings.Contains(string(haystack), needle) {
			t.Errorf("output missing %q\noutput:\n%s", needle, haystack)
		}
	}
}

func TestLoad_IdentityRoundTrip(t *testing.T) {
	original, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	doc, err := omodit.Load(fixturePath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := doc.Bytes(); !bytes.Equal(got, original) {
		t.Errorf("Bytes() differs from file content:\ngot:\n%s\nwant:\n%s", got, original)
	}
}

func TestDocument_Get(t *testing.T) {
	tests := []struct {
		name    string
		pointer string
		want    any
		found   bool
	}{
		{name: "nested bool", pointer: "/models/k3/reasoning", want: true, found: true},
		{name: "telemetry flag", pointer: "/telemetry/enabled", want: true, found: true},
		{name: "string with slashes", pointer: "/models/k3/name", want: "model // not a comment", found: true},
		{name: "array element field", pointer: "/servers/0/name", want: "a", found: true},
		{name: "second array element", pointer: "/servers/1/url", want: "http://b", found: true},
		{name: "escaped slash and tilde", pointer: "/a~1b/c~0d", want: "escaped", found: true},
		{name: "whole document", pointer: "", want: nil, found: true},
		{name: "missing member", pointer: "/does/not/exist", want: nil, found: false},
		{name: "array out of range", pointer: "/servers/9", want: nil, found: false},
		{name: "missing leading slash", pointer: "telemetry/enabled", want: nil, found: false},
		{name: "bad escape", pointer: "/bad~2escape", want: nil, found: false},
		{name: "lone tilde", pointer: "/bad~", want: nil, found: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := omodit.Load(fixturePath)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			got, found := doc.Get(tt.pointer)
			if found != tt.found {
				t.Fatalf("Get(%q) found = %v, want %v", tt.pointer, found, tt.found)
			}
			if !found {
				return
			}
			if tt.pointer == "" {
				if _, ok := got.(map[string]any); !ok {
					t.Errorf("Get(%q) = %T, want whole object", tt.pointer, got)
				}
				return
			}
			if !jsonEqual(got, tt.want) {
				t.Errorf("Get(%q) = %#v, want %#v", tt.pointer, got, tt.want)
			}
		})
	}
}

func jsonEqual(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}

func TestDocument_SetScalar(t *testing.T) {
	tests := []struct {
		name      string
		pointer   string
		value     any
		preserved []string
	}{
		{
			name:    "flip telemetry flag",
			pointer: "/telemetry/enabled",
			value:   false,
			preserved: []string{
				"// Top-level line comment before models",
				"/* block comment before k3 */",
				"// telemetry toggle",
				"// leading comment for server list",
				`"name": "model // not a comment",`,
				`"url": "http://b",`,
			},
		},
		{
			name:    "flip nested reasoning",
			pointer: "/models/k3/reasoning",
			value:   false,
			preserved: []string{
				"// Top-level line comment before models",
				"/* block comment before k3 */",
				"// trailing line comment on reasoning",
				"// telemetry toggle",
				`"url": "http://b",`,
			},
		},
		{
			name:    "change string",
			pointer: "/models/k3/name",
			value:   "renamed",
			preserved: []string{
				"// Top-level line comment before models",
				"/* block comment before k3 */",
				"// trailing line comment on reasoning",
				`"reasoning": true,`,
				`"url": "http://b",`,
			},
		},
		{
			name:    "change array element field",
			pointer: "/servers/0/name",
			value:   "a2",
			preserved: []string{
				"// leading comment for server list",
				"// server a name",
				`"url": "http://b",`,
				"// telemetry toggle",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, _, _ := loadCopy(t)
			if err := doc.Set(tt.pointer, tt.value); err != nil {
				t.Fatalf("Set() error = %v", err)
			}
			got, found := doc.Get(tt.pointer)
			if !found {
				t.Fatalf("Get(%q) found = false after Set", tt.pointer)
			}
			if !jsonEqual(got, tt.value) {
				t.Errorf("Get(%q) = %#v, want %#v", tt.pointer, got, tt.value)
			}
			assertContains(t, doc.Bytes(), tt.preserved)
		})
	}
}

func TestDocument_AddMember(t *testing.T) {
	tests := []struct {
		name       string
		pointer    string
		getPointer string
		value      any
		preserved  []string
	}{
		{
			name:    "new telemetry member",
			pointer: "/telemetry/level",
			value:   "debug",
			preserved: []string{
				"// telemetry toggle",
				"// Top-level line comment before models",
				`"url": "http://b",`,
			},
		},
		{
			name:    "new nested member",
			pointer: "/models/k4/note",
			value:   "new",
			preserved: []string{
				"// line comment before k4",
				"/* block comment before k3 */",
				`"enabled": false,`,
			},
		},
		{
			name:       "append array element",
			pointer:    "/servers/-",
			getPointer: "/servers/2",
			value:      map[string]any{"name": "c", "url": "http://c"},
			preserved: []string{
				"// leading comment for server list",
				"// server a name",
				"// telemetry toggle",
			},
		},
		{
			name:    "append array by index",
			pointer: "/servers/2",
			value:   map[string]any{"name": "c", "url": "http://c"},
			preserved: []string{
				"// leading comment for server list",
				"// telemetry toggle",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, _, _ := loadCopy(t)
			if err := doc.Add(tt.pointer, tt.value); err != nil {
				t.Fatalf("Add() error = %v", err)
			}
			getPointer := tt.getPointer
			if getPointer == "" {
				getPointer = tt.pointer
			}
			got, found := doc.Get(getPointer)
			if !found {
				t.Fatalf("Get(%q) found = false after Add", getPointer)
			}
			if !jsonEqual(got, tt.value) {
				t.Errorf("Get(%q) = %#v, want %#v", getPointer, got, tt.value)
			}
			assertContains(t, doc.Bytes(), tt.preserved)
			// The result must still parse.
			reloadedPath := filepath.Join(t.TempDir(), "reloaded.jsonc")
			if err := os.WriteFile(reloadedPath, doc.Bytes(), 0o644); err != nil {
				t.Fatalf("writing bytes: %v", err)
			}
			if _, err := omodit.Load(reloadedPath); err != nil {
				t.Errorf("re-Load after Add: %v", err)
			}
		})
	}
}

func TestDocument_RemoveMember(t *testing.T) {
	tests := []struct {
		name          string
		pointer       string
		absentPointer string
		checkPointer  string
		checkWant     any
		preserved     []string
	}{
		{
			name:         "remove object member",
			pointer:      "/models/k4",
			checkPointer: "/models/k3/reasoning",
			checkWant:    true,
			preserved: []string{
				"/* block comment before k3 */",
				`"name": "model // not a comment",`,
				"// telemetry toggle",
				`"url": "http://b",`,
			},
		},
		{
			name:         "remove scalar member",
			pointer:      "/telemetry/enabled",
			checkPointer: "/models/k3/reasoning",
			checkWant:    true,
			preserved: []string{
				"// Top-level line comment before models",
				"// leading comment for server list",
				`"url": "http://b",`,
			},
		},
		{
			name:          "remove array element",
			pointer:       "/servers/0",
			absentPointer: "/servers/1",
			checkPointer:  "/servers/0/name",
			checkWant:     "b",
			preserved: []string{
				"// telemetry toggle",
				"/* block comment before k3 */",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, _, _ := loadCopy(t)
			if err := doc.Remove(tt.pointer); err != nil {
				t.Fatalf("Remove() error = %v", err)
			}
			absent := tt.absentPointer
			if absent == "" {
				absent = tt.pointer
			}
			if _, found := doc.Get(absent); found {
				t.Errorf("Get(%q) found = true after Remove", absent)
			}
			if tt.checkPointer != "" {
				got, found := doc.Get(tt.checkPointer)
				if !found {
					t.Errorf("Get(%q) found = false after Remove", tt.checkPointer)
				} else if !jsonEqual(got, tt.checkWant) {
					t.Errorf("Get(%q) = %#v, want %#v", tt.checkPointer, got, tt.checkWant)
				}
			}
			assertContains(t, doc.Bytes(), tt.preserved)
		})
	}
}

func TestDocument_CommentsPreserved(t *testing.T) {
	doc, _, _ := loadCopy(t)
	if err := doc.Set("/telemetry/enabled", false); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	assertContains(t, doc.Bytes(), []string{
		"// Top-level line comment before models",
		"/* block comment before k3 */",
		"// trailing line comment on reasoning",
		"// line comment before k4",
		"// trailing comment after models object",
		"// telemetry toggle",
		"// Escaped keys for pointer tests",
		"// leading comment for server list",
		"// server a name",
	})
}

func TestDocument_TrailingCommasPreserved(t *testing.T) {
	doc, _, _ := loadCopy(t)
	if err := doc.Set("/telemetry/enabled", false); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	assertContains(t, doc.Bytes(), []string{
		`"name": "model // not a comment",`,
		`"enabled": false,`,
		`"url": "http://b",`,
	})
}

func TestDocument_SaveReloadValues(t *testing.T) {
	doc, path, _ := loadCopy(t)
	if err := doc.Set("/telemetry/enabled", false); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := doc.Add("/telemetry/level", "debug"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := doc.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	reloaded, err := omodit.Load(path)
	if err != nil {
		t.Fatalf("re-Load error = %v", err)
	}
	if got, _ := reloaded.Get("/telemetry/enabled"); !jsonEqual(got, false) {
		t.Errorf("reloaded /telemetry/enabled = %#v, want false", got)
	}
	if got, _ := reloaded.Get("/telemetry/level"); !jsonEqual(got, "debug") {
		t.Errorf("reloaded /telemetry/level = %#v, want debug", got)
	}
}

var backupPattern = regexp.MustCompile(`\.bak\.\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}-\d+Z(-\d+)?$`)

func listDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading dir: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestDocument_SaveAtomicity(t *testing.T) {
	doc, path, original := loadCopy(t)
	dir := filepath.Dir(path)
	if err := doc.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	for _, name := range listDir(t, dir) {
		if strings.HasPrefix(name, ".omodit-") {
			t.Errorf("temp leftover %q remains after Save", name)
		}
	}
	var backups []string
	for _, name := range listDir(t, dir) {
		if strings.Contains(name, ".bak.") {
			backups = append(backups, name)
		}
	}
	if len(backups) != 1 {
		t.Fatalf("want 1 backup, got %d (%v)", len(backups), backups)
	}
	if !backupPattern.MatchString(backups[0]) {
		t.Errorf("backup %q does not match expected pattern", backups[0])
	}
	backupBytes, err := os.ReadFile(filepath.Join(dir, backups[0]))
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if !bytes.Equal(backupBytes, original) {
		t.Errorf("backup content differs from original")
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}
	if !bytes.Equal(saved, original) {
		t.Errorf("no-op Save changed file bytes")
	}
}

func TestDocument_SaveNewFile(t *testing.T) {
	doc, path, _ := loadCopy(t)
	if err := os.Remove(path); err != nil {
		t.Fatalf("removing copy: %v", err)
	}
	if err := doc.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat saved file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Errorf("new file mode = %o, want 644", perm)
	}
	for _, name := range listDir(t, filepath.Dir(path)) {
		if strings.Contains(name, ".bak.") {
			t.Errorf("unexpected backup %q for new file", name)
		}
	}
}

func TestDocument_Errors(t *testing.T) {
	tests := []struct {
		name            string
		op              string
		pointer         string
		value           any
		wantErrContains string
	}{
		{name: "set missing pointer", op: "set", pointer: "/nope/missing", value: 1, wantErrContains: "pointer not found"},
		{name: "set invalid pointer", op: "set", pointer: "no-leading-slash", value: 1, wantErrContains: "invalid JSON pointer"},
		{name: "set bad escape", op: "set", pointer: "/bad~2", value: 1, wantErrContains: "bad escape"},
		{name: "remove missing pointer", op: "remove", pointer: "/nope/missing", wantErrContains: "pointer not found"},
		{name: "remove root", op: "remove", pointer: "", wantErrContains: "cannot remove root"},
		{name: "remove invalid pointer", op: "remove", pointer: "/bad~", wantErrContains: "bad escape"},
		{name: "add existing pointer", op: "add", pointer: "/telemetry/enabled", value: false, wantErrContains: "already exists"},
		{name: "add root", op: "add", pointer: "", value: 1, wantErrContains: "already exists"},
		{name: "add invalid pointer", op: "add", pointer: "nope", value: 1, wantErrContains: "invalid JSON pointer"},
		{name: "add missing parent", op: "add", pointer: "/nope/missing/child", value: 1, wantErrContains: "parent"},
		{name: "add array out of range", op: "add", pointer: "/servers/99", value: 1, wantErrContains: "out of range"},
		{name: "add bad array index", op: "add", pointer: "/servers/nope", value: 1, wantErrContains: "invalid array index"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, _, _ := loadCopy(t)
			var err error
			switch tt.op {
			case "set":
				err = doc.Set(tt.pointer, tt.value)
			case "add":
				err = doc.Add(tt.pointer, tt.value)
			case "remove":
				err = doc.Remove(tt.pointer)
			}
			if err == nil {
				t.Fatalf("%s(%q) error = nil, want containing %q", tt.op, tt.pointer, tt.wantErrContains)
			}
			if !strings.Contains(err.Error(), tt.wantErrContains) {
				t.Errorf("%s(%q) error = %q, want containing %q", tt.op, tt.pointer, err.Error(), tt.wantErrContains)
			}
		})
	}
}

func TestLoad_Errors(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		wantErrContains string
	}{
		{name: "invalid JSONC", content: "{ invalid jsonc !!!", wantErrContains: "loading config"},
		{name: "empty object closing", content: `{"a": }`, wantErrContains: "loading config"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "bad.jsonc")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("writing bad fixture: %v", err)
			}
			if _, err := omodit.Load(path); err == nil {
				t.Fatalf("Load() error = nil, want containing %q", tt.wantErrContains)
			} else if !strings.Contains(err.Error(), tt.wantErrContains) {
				t.Errorf("Load() error = %q, want containing %q", err.Error(), tt.wantErrContains)
			}
		})
	}

	t.Run("missing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "does-not-exist.jsonc")
		_, err := omodit.Load(path)
		if err == nil {
			t.Fatalf("Load() error = nil, want containing %q", "loading config")
		}
		if !strings.Contains(err.Error(), "loading config") {
			t.Errorf("Load() error = %q, want containing %q", err.Error(), "loading config")
		}
	})
}

func TestDocument_PointerEscaping(t *testing.T) {
	doc, _, _ := loadCopy(t)
	got, found := doc.Get("/a~1b/c~0d")
	if !found || !jsonEqual(got, "escaped") {
		t.Fatalf("Get escaped = %#v, %v; want %q, true", got, found, "escaped")
	}
	if err := doc.Set("/a~1b/c~0d", "changed"); err != nil {
		t.Fatalf("Set escaped error = %v", err)
	}
	if got, _ := doc.Get("/a~1b/c~0d"); !jsonEqual(got, "changed") {
		t.Errorf("Get escaped after Set = %#v, want changed", got)
	}
	if _, found := doc.Get("/a~1b/c~1d"); found {
		t.Errorf("Get with wrong escape found = true, want false")
	}
	if err := doc.Add("/a~1b/new~1key", 1.0); err != nil {
		t.Fatalf("Add escaped error = %v", err)
	}
	if got, found := doc.Get("/a~1b/new~1key"); !found || !jsonEqual(got, 1.0) {
		t.Errorf("Get new escaped = %#v, %v; want 1, true", got, found)
	}
}

// TestRealConfigRoundTrip exercises a real-world config file whose path is
// supplied via LAZYOMO_REAL_CONFIG. The env value is only ever read; all
// mutation happens on a temp copy, so the referenced file is never modified.
func TestRealConfigRoundTrip(t *testing.T) {
	envPath := os.Getenv("LAZYOMO_REAL_CONFIG")
	if envPath == "" {
		t.Skip("set LAZYOMO_REAL_CONFIG to run")
	}
	original, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("reading LAZYOMO_REAL_CONFIG: %v", err)
	}
	workdir := t.TempDir()
	copyPath := filepath.Join(workdir, "omo.jsonc")
	if err := os.WriteFile(copyPath, original, 0o644); err != nil {
		t.Fatalf("copying config: %v", err)
	}
	origSum := sha256.Sum256(original)

	t.Run("identity", func(t *testing.T) {
		doc, err := omodit.Load(copyPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if sum := sha256.Sum256(doc.Bytes()); sum != origSum {
			t.Errorf("Load then Bytes differs from original file bytes")
		}
	})

	t.Run("no-op Save", func(t *testing.T) {
		doc, err := omodit.Load(copyPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if err := doc.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		saved, err := os.ReadFile(copyPath)
		if err != nil {
			t.Fatalf("reading saved file: %v", err)
		}
		if sum := sha256.Sum256(saved); sum != origSum {
			t.Errorf("no-op Save changed file bytes")
		}
		backups := backupsFor(t, workdir, "omo.jsonc")
		if len(backups) == 0 {
			t.Fatalf("no backup created by Save")
		}
		for _, name := range backups {
			if !backupPattern.MatchString(name) {
				t.Errorf("backup %q does not match expected pattern", name)
			}
		}
	})

	t.Run("mutation on the COPY", func(t *testing.T) {
		doc, err := omodit.Load(copyPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		pointer, oldValue, newValue := pickFlipTarget(t, doc, original)
		oldJSON, _ := json.Marshal(oldValue)
		if err := doc.Set(pointer, newValue); err != nil {
			t.Fatalf("Set(%q) error = %v", pointer, err)
		}
		if err := doc.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		reloaded, err := omodit.Load(copyPath)
		if err != nil {
			t.Fatalf("re-Load error = %v (file still parses check failed)", err)
		}
		got, found := reloaded.Get(pointer)
		if !found {
			t.Fatalf("Get(%q) found = false after mutation", pointer)
		}
		if !jsonEqual(got, newValue) {
			t.Errorf("Get(%q) = %#v, want %#v", pointer, got, newValue)
		}
		mutated, err := os.ReadFile(copyPath)
		if err != nil {
			t.Fatalf("reading mutated file: %v", err)
		}
		assertCommentLinesKept(t, original, mutated, pointerToken(pointer), string(oldJSON))
		// Restore the copy from the newest backup so it returns byte-identical.
		backups := backupsFor(t, workdir, "omo.jsonc")
		if len(backups) == 0 {
			t.Fatalf("no backup to restore from")
		}
		sort.Strings(backups)
		backupBytes, err := os.ReadFile(filepath.Join(workdir, backups[len(backups)-1]))
		if err != nil {
			t.Fatalf("reading backup: %v", err)
		}
		if err := os.WriteFile(copyPath, backupBytes, 0o644); err != nil {
			t.Fatalf("restoring copy: %v", err)
		}
		restored, err := os.ReadFile(copyPath)
		if err != nil {
			t.Fatalf("reading restored copy: %v", err)
		}
		if sum := sha256.Sum256(restored); sum != origSum {
			t.Errorf("restored copy differs from original")
		}
	})
}

// backupsFor lists backup file names for base in dir.
func backupsFor(t *testing.T, dir, base string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading dir: %v", err)
	}
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), base+".bak.") {
			out = append(out, e.Name())
		}
	}
	return out
}

// pickFlipTarget finds a scalar to flip: /telemetry/enabled when it holds a
// bool, else the first scalar from a small pointer list, else the first bool
// (then string, then number) found by walking the decoded document with
// sorted object keys for determinism. String and number values are nudged;
// bools are negated.
func pickFlipTarget(t *testing.T, doc *omodit.Document, original []byte) (string, any, any) {
	t.Helper()
	if old, found := doc.Get("/telemetry/enabled"); found {
		if b, ok := old.(bool); ok {
			return "/telemetry/enabled", b, !b
		}
	}
	for _, pointer := range []string{"/telemetry/enabled", "/telemetry/level", "/models/default"} {
		if old, found := doc.Get(pointer); found && isScalar(old) {
			return pointer, old, nudged(old)
		}
	}
	root, found := doc.Get("")
	if !found {
		t.Fatalf("cannot read document root")
	}
	var bools, stringsFound, numbers [][2]any
	var walk func(node any, pointer string)
	walk = func(node any, pointer string) {
		switch typed := node.(type) {
		case map[string]any:
			keys := make([]string, 0, len(typed))
			for k := range typed {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				walk(typed[k], pointer+"/"+escapeSegment(k))
			}
		case []any:
			for i, item := range typed {
				walk(item, pointer+"/"+itoa(i))
			}
		case bool:
			bools = append(bools, [2]any{pointer, typed})
		case string:
			stringsFound = append(stringsFound, [2]any{pointer, typed})
		case float64:
			numbers = append(numbers, [2]any{pointer, typed})
		}
	}
	walk(root, "")
	for _, group := range [][][2]any{bools, stringsFound, numbers} {
		if best, ok := pickLineWithoutComment(original, group); ok {
			old := best[1]
			return best[0].(string), old, nudged(old)
		}
	}
	t.Fatalf("no scalar value found to mutate")
	return "", nil, nil
}

func isScalar(v any) bool {
	switch v.(type) {
	case bool, string, float64:
		return true
	}
	return false
}

func nudged(v any) any {
	switch typed := v.(type) {
	case bool:
		return !typed
	case string:
		return typed + "-omodit-test"
	case float64:
		return typed + 1
	}
	return v
}

// pickLineWithoutComment prefers a candidate whose source line carries no
// "//", so every original comment line stays literally present after the
// mutation. It falls back to the first candidate.
func pickLineWithoutComment(original []byte, candidates [][2]any) ([2]any, bool) {
	if len(candidates) == 0 {
		return [2]any{}, false
	}
	lines := strings.Split(string(original), "\n")
outer:
	for _, cand := range candidates {
		token := pointerToken(cand[0].(string))
		oldJSON, _ := json.Marshal(cand[1])
		for _, line := range lines {
			if strings.Contains(line, `"`) && strings.Contains(line, token) &&
				strings.Contains(line, string(oldJSON)) && !strings.Contains(line, "//") {
				return cand, true
			}
		}
		continue outer
	}
	return candidates[0], true
}

// pointerToken returns the last token of a JSON pointer with escapes resolved.
func pointerToken(pointer string) string {
	idx := strings.LastIndex(pointer, "/")
	token := pointer[idx+1:]
	token = strings.ReplaceAll(token, "~1", "/")
	return strings.ReplaceAll(token, "~0", "~")
}

// escapeSegment encodes one object key as a pointer token.
func escapeSegment(key string) string {
	key = strings.ReplaceAll(key, "~", "~0")
	return strings.ReplaceAll(key, "/", "~1")
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

// assertCommentLinesKept requires every original line containing "//" to
// still be present in the mutated file, except the edited line itself, whose
// comment suffix must survive.
func assertCommentLinesKept(t *testing.T, original, mutated []byte, editedToken, oldJSON string) {
	t.Helper()
	mutatedText := string(mutated)
	for _, line := range strings.Split(string(original), "\n") {
		if !strings.Contains(line, "//") {
			continue
		}
		if strings.Contains(mutatedText, line) {
			continue
		}
		if strings.Contains(line, editedToken) && strings.Contains(line, oldJSON) {
			suffix := line[strings.Index(line, "//"):]
			if strings.Contains(mutatedText, strings.TrimSpace(suffix)) {
				continue
			}
		}
		t.Errorf("original comment line lost:\n%s\nmutated file:\n%s", line, mutated)
	}
}
