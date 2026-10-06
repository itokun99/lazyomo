package editor_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/itokun99/lazyomo/internal/editor"
	"github.com/itokun99/lazyomo/internal/omodit"
)

const fixturePath = "testdata/sample.jsonc"

// fixtureComments is every comment line in testdata/sample.jsonc. None sits
// inside an array that the end-to-end test replaces, so all must survive.
var fixtureComments = []string{
	"// lazyomo editor test fixture: comments everywhere on purpose.",
	"// schema pin (read-only in v1)",
	"/* block comment before the first alias */",
	"// primary model",
	"// second alias carries a nested read-only option block",
	"// remaining aliases keep this fixture complete",
	"// profiles select model lanes",
	"// main lane",
	"// daily chain ends here",
	"// active profile selector",
	"// overlay reference",
	"// librarian fallback chain",
	"// ultrabrain chain",
	"// telemetry toggle",
}

func copyFixture(t *testing.T) (string, []byte) {
	t.Helper()
	original, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), "omo.jsonc")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("copying fixture: %v", err)
	}
	return path, original
}

func loadEditor(t *testing.T) (*editor.Editor, string, []byte) {
	t.Helper()
	path, original := copyFixture(t)
	ed, err := editor.Load(path)
	if err != nil {
		t.Fatalf("editor.Load() error = %v", err)
	}
	return ed, path, original
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "omo.jsonc")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	return path
}

// rawValue loads path with omodit and returns the decoded value at pointer.
func rawValue(t *testing.T, path, pointer string) (any, bool) {
	t.Helper()
	doc, err := omodit.Load(path)
	if err != nil {
		t.Fatalf("omodit.Load(%q) error = %v", path, err)
	}
	return doc.Get(pointer)
}

func jsonEqual(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}

func sectionByID(sections []editor.Section, id editor.SectionID) (editor.Section, bool) {
	for _, section := range sections {
		if section.ID == id {
			return section, true
		}
	}
	return editor.Section{}, false
}

func entryByKey(entries []editor.Entry, key string) (editor.Entry, bool) {
	for _, entry := range entries {
		if entry.Key == key {
			return entry, true
		}
	}
	return editor.Entry{}, false
}

func entryKeys(entries []editor.Entry) []string {
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		keys = append(keys, entry.Key)
	}
	return keys
}

func TestLoad_Sections(t *testing.T) {
	ed, _, _ := loadEditor(t)
	sections := ed.Sections()

	if len(sections) != 8 {
		t.Fatalf("Sections() returned %d sections, want 8", len(sections))
	}
	wantIDs := []editor.SectionID{
		editor.SectionModels,
		editor.SectionModelProfiles,
		editor.SectionAgents,
		editor.SectionCategories,
		editor.SectionTelemetry,
		editor.SectionGitMaster,
		editor.SectionID("$schema"),
		editor.SectionID("model_profile"),
	}
	wantTitles := []string{"Models", "Model Profiles", "Agents", "Categories", "Telemetry", "Git", "$schema", "model_profile"}
	wantReadOnly := []bool{false, false, false, false, false, false, true, true}
	for i, section := range sections {
		if section.ID != wantIDs[i] {
			t.Errorf("Sections()[%d].ID = %q, want %q", i, section.ID, wantIDs[i])
		}
		if section.Title != wantTitles[i] {
			t.Errorf("Sections()[%d].Title = %q, want %q", i, section.Title, wantTitles[i])
		}
		if section.ReadOnly != wantReadOnly[i] {
			t.Errorf("Sections()[%d] (%s) ReadOnly = %v, want %v", i, section.ID, section.ReadOnly, wantReadOnly[i])
		}
	}

	models, _ := sectionByID(sections, editor.SectionModels)
	wantModelKeys := []string{"agentChain", "catChain", "chainOnly", "dsflash", "k3", "spaceBunny", "spare"}
	if got := entryKeys(models.Entries); !reflect.DeepEqual(got, wantModelKeys) {
		t.Errorf("model keys = %v, want %v", got, wantModelKeys)
	}
	k3, _ := entryByKey(models.Entries, "k3")
	if k3.Path != "/models/k3" || k3.Kind != editor.KindAlias {
		t.Errorf("k3 entry = %+v, want path /models/k3 kind KindAlias", k3)
	}
	if k3.Value != "github-copilot/claude-sonnet-5" {
		t.Errorf("k3.Value = %q, want %q", k3.Value, "github-copilot/claude-sonnet-5")
	}
	if k3.Dirty || k3.ReadOnly {
		t.Errorf("k3 entry = %+v, want clean and editable", k3)
	}
	librarianAgent, _ := entryByKey(mustSection(t, sections, editor.SectionAgents).Entries, "librarian")
	if librarianAgent.Value != "" {
		t.Errorf("librarian.Value = %q, want empty (no model key)", librarianAgent.Value)
	}
	daily, _ := entryByKey(mustSection(t, sections, editor.SectionModelProfiles).Entries, "daily")
	if daily.Value != "Daily Driver" {
		t.Errorf("daily.Value = %q, want %q", daily.Value, "Daily Driver")
	}
	quick, _ := entryByKey(mustSection(t, sections, editor.SectionCategories).Entries, "quick")
	if quick.Value != "k3" {
		t.Errorf("quick.Value = %q, want k3", quick.Value)
	}
	telemetry, _ := sectionByID(sections, editor.SectionTelemetry)
	if len(telemetry.Entries) != 1 {
		t.Fatalf("telemetry entries = %d, want 1", len(telemetry.Entries))
	}
	enabled := telemetry.Entries[0]
	if enabled.Key != "enabled" || enabled.Path != "/telemetry/enabled" || enabled.Kind != editor.KindBool {
		t.Errorf("telemetry entry = %+v, want enabled KindBool at /telemetry/enabled", enabled)
	}
	if enabled.Value != "false" {
		t.Errorf("telemetry enabled value = %q, want false", enabled.Value)
	}
	if enabled.ReadOnly {
		t.Errorf("telemetry entry = %+v, want editable", enabled)
	}
	git, _ := sectionByID(sections, editor.SectionGitMaster)
	if len(git.Entries) != 2 {
		t.Fatalf("git entries = %d, want 2", len(git.Entries))
	}
	for i, key := range []string{"commit_footer", "include_co_authored_by"} {
		entry := git.Entries[i]
		if entry.Key != key || entry.Path != "/git_master/"+key || entry.Kind != editor.KindBool {
			t.Errorf("git entry %d = %+v, want %s KindBool at /git_master/%s", i, entry, key, key)
		}
		if entry.Value != "false" {
			t.Errorf("git %s value = %q, want false", key, entry.Value)
		}
		if entry.ReadOnly {
			t.Errorf("git entry %d = %+v, want editable", i, entry)
		}
	}
	schema, _ := sectionByID(sections, editor.SectionID("$schema"))
	if len(schema.Entries) != 1 {
		t.Fatalf("schema entries = %d, want 1", len(schema.Entries))
	}
	schemaEntry := schema.Entries[0]
	if schemaEntry.Key != "$schema" || schemaEntry.Path != "/$schema" {
		t.Errorf("schema entry = %+v, want key $schema at /$schema", schemaEntry)
	}
	if schemaEntry.Value != "https://example.invalid/omo.schema.json" {
		t.Errorf("schema entry value = %q, want the schema URL", schemaEntry.Value)
	}
	if !schemaEntry.ReadOnly {
		t.Errorf("schema entry = %+v, want read-only", schemaEntry)
	}
	profile, _ := sectionByID(sections, editor.SectionID("model_profile"))
	if len(profile.Entries) != 1 {
		t.Fatalf("model_profile entries = %d, want 1", len(profile.Entries))
	}
	profileEntry := profile.Entries[0]
	if profileEntry.Key != "model_profile" || profileEntry.Value != "daily" {
		t.Errorf("model_profile entry = %+v, want key model_profile with value daily", profileEntry)
	}
	if !profileEntry.ReadOnly {
		t.Errorf("model_profile entry = %+v, want read-only", profileEntry)
	}
}

func mustSection(t *testing.T, sections []editor.Section, id editor.SectionID) editor.Section {
	t.Helper()
	section, ok := sectionByID(sections, id)
	if !ok {
		t.Fatalf("section %q not found", id)
	}
	return section
}

func TestLoad_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.jsonc")
	_, err := editor.Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("Load() error = %q, want it to name %q", err, path)
	}
}

func TestLoad_MissingBlocks(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantEmpty []editor.SectionID
	}{
		{
			name:    "empty object",
			content: "{}\n",
			wantEmpty: []editor.SectionID{
				editor.SectionModels,
				editor.SectionModelProfiles,
				editor.SectionAgents,
				editor.SectionCategories,
			},
		},
		{
			name:    "block of wrong type",
			content: "{\n  \"models\": 42,\n  \"agents\": \"none\",\n}\n",
			wantEmpty: []editor.SectionID{
				editor.SectionModels,
				editor.SectionAgents,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeConfig(t, tt.content)
			ed, err := editor.Load(path)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			sections := ed.Sections()
			if len(sections) != 6 {
				t.Fatalf("Sections() returned %d sections, want 6", len(sections))
			}
			for _, id := range tt.wantEmpty {
				section := mustSection(t, sections, id)
				if len(section.Entries) != 0 {
					t.Errorf("section %s entries = %v, want empty", id, entryKeys(section.Entries))
				}
			}
			telemetry := mustSection(t, sections, editor.SectionTelemetry)
			if len(telemetry.Entries) != 1 || telemetry.Entries[0].Value != "false" {
				t.Errorf("telemetry entries = %+v, want a single false switch", telemetry.Entries)
			}
			git := mustSection(t, sections, editor.SectionGitMaster)
			if len(git.Entries) != 2 || git.Entries[0].Value != "false" || git.Entries[1].Value != "false" {
				t.Errorf("git entries = %+v, want two false switches", git.Entries)
			}
		})
	}
}

func TestLoadDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".omo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	content := "{\n  \"telemetry\": {\n    \"enabled\": true, // on\n  },\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "omo.jsonc"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing default config: %v", err)
	}

	ed, err := editor.LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error = %v", err)
	}
	if want := filepath.Join(dir, "omo.jsonc"); ed.Path() != want {
		t.Errorf("Path() = %q, want %q", ed.Path(), want)
	}
	telemetry := mustSection(t, ed.Sections(), editor.SectionTelemetry)
	if telemetry.Entries[0].Value != "true" {
		t.Errorf("telemetry value = %q, want true", telemetry.Entries[0].Value)
	}
}

func TestDetail(t *testing.T) {
	tests := []struct {
		name    string
		section editor.SectionID
		key     string
		want    editor.Detail
	}{
		{
			name:    "model alias",
			section: editor.SectionModels,
			key:     "k3",
			want: editor.Detail{Title: "k3", Lines: []editor.DetailLine{
				{Label: "model", Path: "/models/k3/model", Value: "github-copilot/claude-sonnet-5", Editable: true},
				{Label: "reasoning", Path: "/models/k3/reasoning", Value: "high", Editable: true},
			}},
		},
		{
			name:    "model alias with read-only extra",
			section: editor.SectionModels,
			key:     "spaceBunny",
			want: editor.Detail{Title: "spaceBunny", Lines: []editor.DetailLine{
				{Label: "model", Path: "/models/spaceBunny/model", Value: "opencode-go/space-bunny-free", Editable: true},
				{Label: "reasoning", Path: "/models/spaceBunny/reasoning", Value: "max", Editable: true},
				{Label: "options", Path: "/models/spaceBunny/options", Value: `{"temperature":0.2}`},
			}},
		},
		{
			name:    "profile",
			section: editor.SectionModelProfiles,
			key:     "daily",
			want: editor.Detail{Title: "daily", Lines: []editor.DetailLine{
				{Label: "display_name", Path: "/model_profiles/daily/display_name", Value: "Daily Driver", Editable: true},
				{
					Label: "models", Path: "/model_profiles/daily/models", Editable: true,
					Value: `["github-copilot/claude-sonnet-5:max","spaceBunny:max","chainOnly:high"]`,
				},
			}},
		},
		{
			name:    "agent with empty chain and default disable",
			section: editor.SectionAgents,
			key:     "explore",
			want: editor.Detail{Title: "explore", Lines: []editor.DetailLine{
				{Label: "model", Path: "/agents/explore/model", Value: "spaceBunny", Editable: true},
				{Label: "models", Path: "/agents/explore/models", Editable: true},
				{Label: "reasoning", Path: "/agents/explore/reasoning", Value: "max", Editable: true},
				{Label: "disable", Path: "/agents/explore/disable", Value: "false", Bool: true, Editable: true},
			}},
		},
		{
			name:    "agent with chain only",
			section: editor.SectionAgents,
			key:     "librarian",
			want: editor.Detail{Title: "librarian", Lines: []editor.DetailLine{
				{Label: "model", Path: "/agents/librarian/model", Editable: true},
				{Label: "models", Path: "/agents/librarian/models", Value: `["agentChain"]`, Editable: true},
				{Label: "reasoning", Path: "/agents/librarian/reasoning", Editable: true},
				{Label: "disable", Path: "/agents/librarian/disable", Value: "false", Bool: true, Editable: true},
			}},
		},
		{
			name:    "category",
			section: editor.SectionCategories,
			key:     "quick",
			want: editor.Detail{Title: "quick", Lines: []editor.DetailLine{
				{Label: "model", Path: "/categories/quick/model", Value: "k3", Editable: true},
				{Label: "models", Path: "/categories/quick/models", Editable: true},
			}},
		},
		{
			name:    "telemetry switch",
			section: editor.SectionTelemetry,
			key:     "enabled",
			want: editor.Detail{Title: "enabled", Lines: []editor.DetailLine{
				{Label: "enabled", Path: "/telemetry/enabled", Value: "false", Bool: true, Editable: true},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ed, _, _ := loadEditor(t)
			got, err := ed.Detail(tt.section, tt.key)
			if err != nil {
				t.Fatalf("Detail(%q, %q) error = %v", tt.section, tt.key, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Detail(%q, %q) =\n%#v\nwant\n%#v", tt.section, tt.key, got, tt.want)
			}
		})
	}

	t.Run("errors", func(t *testing.T) {
		ed, _, _ := loadEditor(t)
		errorTests := []struct {
			name    string
			section editor.SectionID
			key     string
			wantErr string
		}{
			{name: "unknown section", section: "nope", key: "k3", wantErr: "unknown section"},
			{name: "missing entry", section: editor.SectionModels, key: "ghost", wantErr: "no models entry"},
			{name: "missing telemetry key", section: editor.SectionTelemetry, key: "other", wantErr: "no telemetry entry"},
		}
		for _, tt := range errorTests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := ed.Detail(tt.section, tt.key)
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Detail(%q, %q) error = %v, want substring %q", tt.section, tt.key, err, tt.wantErr)
				}
			})
		}
	})
}

func TestSetScalar(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		value       string
		wantErrPart string
		wantPointer string
		wantValue   any
		wantMissing bool
	}{
		{name: "model", path: "/models/k3/model", value: "adacode/claude-opus-4-6", wantValue: "adacode/claude-opus-4-6"},
		{name: "model with suffix", path: "/models/k3/model", value: "vendor/model:max", wantValue: "vendor/model:max"},
		{name: "reasoning", path: "/models/k3/reasoning", value: "low", wantValue: "low"},
		{name: "reasoning unset", path: "/models/k3/reasoning", value: "", wantPointer: "/models/k3/reasoning", wantMissing: true},
		{
			name: "reasoning added to alias without one", path: "/agents/oracle/reasoning",
			value: "medium", wantValue: "medium",
		},
		{name: "display name", path: "/model_profiles/cheap/display_name", value: "Cheapest", wantValue: "Cheapest"},
		{
			name: "display name at 64 characters", path: "/model_profiles/cheap/display_name",
			value: strings.Repeat("x", 64), wantValue: strings.Repeat("x", 64),
		},
		{name: "active profile", path: "/model_profile", value: "cheap", wantValue: "cheap"},
		{name: "active profile pin", path: "/model_profile", value: "anthropic/claude-opus-5", wantValue: "anthropic/claude-opus-5"},
		{name: "active profile pin with suffix", path: "/model_profile", value: "adacode/gpt-5-3:max", wantValue: "adacode/gpt-5-3:max"},
		{name: "agent model alias", path: "/agents/oracle/model", value: "k3", wantValue: "k3"},
		{name: "agent model full reference", path: "/agents/oracle/model", value: "vendor/model-1", wantValue: "vendor/model-1"},
		{name: "agent reasoning", path: "/agents/oracle/reasoning", value: "high", wantValue: "high"},
		{name: "category model alias", path: "/categories/ultrabrain/model", value: "spare", wantValue: "spare"},
		{
			name: "category model full reference", path: "/categories/ultrabrain/model",
			value: "vendor/model-2", wantValue: "vendor/model-2",
		},
		{name: "empty model", path: "/models/k3/model", value: "", wantErrPart: "invalid model"},
		{name: "model without provider", path: "/models/k3/model", value: "plain", wantErrPart: "invalid model"},
		{name: "model empty provider", path: "/models/k3/model", value: "/model", wantErrPart: "invalid model"},
		{name: "model empty id", path: "/models/k3/model", value: "provider/", wantErrPart: "invalid model"},
		{name: "model with space", path: "/models/k3/model", value: "provi der/model", wantErrPart: "invalid model"},
		{name: "bad reasoning", path: "/models/k3/reasoning", value: "extreme", wantErrPart: "invalid reasoning"},
		{name: "empty display name", path: "/model_profiles/cheap/display_name", value: "", wantErrPart: "invalid display_name"},
		{
			name: "display name too long", path: "/model_profiles/cheap/display_name",
			value: strings.Repeat("x", 65), wantErrPart: "invalid display_name",
		},
		{name: "unknown profile", path: "/model_profile", value: "ghost", wantErrPart: "unknown model profile"},
		{name: "nonsense profile", path: "/model_profile", value: "nonsense", wantErrPart: "unknown model profile"},
		{name: "malformed pin empty id", path: "/model_profile", value: "x/", wantErrPart: "unknown model profile"},
		{name: "clearing active profile blocked", path: "/model_profile", value: "", wantErrPart: "blocked"},
		{name: "agent model unknown", path: "/agents/oracle/model", value: "ghost", wantErrPart: "invalid model"},
		{name: "agent model empty", path: "/agents/oracle/model", value: "", wantErrPart: "invalid model"},
		{name: "category model unknown", path: "/categories/quick/model", value: "ghost", wantErrPart: "invalid model"},
		{name: "missing alias", path: "/models/ghost/model", value: "a/b", wantErrPart: "no models entry"},
		{
			name: "missing profile", path: "/model_profiles/ghost/display_name",
			value: "x", wantErrPart: "no model_profiles entry",
		},
		{name: "missing agent", path: "/agents/ghost/model", value: "a/b", wantErrPart: "no agents entry"},
		{name: "read-only top-level key", path: "/git_master/commit_footer", value: "true", wantErrPart: "not editable"},
		{name: "read-only entry field", path: "/models/k3/name", value: "x", wantErrPart: "not editable"},
		{name: "bool via SetScalar", path: "/telemetry/enabled", value: "true", wantErrPart: "not editable"},
		{name: "entry itself", path: "/models/k3", value: "x", wantErrPart: "not editable"},
		{name: "unknown root", path: "/foo/bar", value: "x", wantErrPart: "not editable"},
		{name: "bad pointer", path: "models/k3/model", value: "a/b", wantErrPart: "invalid path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ed, path, _ := loadEditor(t)
			err := ed.SetScalar(tt.path, tt.value)
			if tt.wantErrPart != "" {
				if err == nil {
					t.Fatalf("SetScalar(%q, %q) error = nil, want substring %q", tt.path, tt.value, tt.wantErrPart)
				}
				if !strings.Contains(err.Error(), tt.wantErrPart) {
					t.Fatalf("SetScalar(%q, %q) error = %q, want substring %q", tt.path, tt.value, err, tt.wantErrPart)
				}
				return
			}
			if err != nil {
				t.Fatalf("SetScalar(%q, %q) error = %v", tt.path, tt.value, err)
			}
			if _, err := ed.Save(); err != nil {
				t.Fatalf("Save() after SetScalar error = %v", err)
			}
			pointer := tt.path
			if tt.wantPointer != "" {
				pointer = tt.wantPointer
			}
			got, found := rawValue(t, path, pointer)
			if tt.wantMissing {
				if found {
					t.Fatalf("value at %q still present: %#v", pointer, got)
				}
				return
			}
			if !found {
				t.Fatalf("value at %q missing after save", pointer)
			}
			if !jsonEqual(got, tt.wantValue) {
				t.Errorf("value at %q = %#v, want %#v", pointer, got, tt.wantValue)
			}
		})
	}
}

func TestSetScalar_Chain(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		value       string
		wantErrPart string
		wantPointer string
		want        any
	}{
		{
			name:  "json array whole chain",
			path:  "/model_profiles/cheap/models",
			value: `["github-copilot/claude-sonnet-5:max", "vendor/x"]`,
			want:  []string{"github-copilot/claude-sonnet-5:max", "vendor/x"},
		},
		{
			name:  "comma separated whole chain",
			path:  "/model_profiles/cheap/models",
			value: " github-copilot/claude-sonnet-5:max , vendor/x ",
			want:  []string{"github-copilot/claude-sonnet-5:max", "vendor/x"},
		},
		{
			name:  "alias reference elements",
			path:  "/model_profiles/cheap/models",
			value: "k3, spaceBunny:max",
			want:  []string{"k3", "spaceBunny:max"},
		},
		{
			name:  "keeps duplicates",
			path:  "/model_profiles/daily/models",
			value: "a/b,a/b",
			want:  []string{"a/b", "a/b"},
		},
		{
			name:        "element replace",
			path:        "/model_profiles/daily/models/1",
			value:       "vendor/model:max",
			wantPointer: "/model_profiles/daily/models",
			want:        []string{"github-copilot/claude-sonnet-5:max", "vendor/model:max", "chainOnly:high"},
		},
		{
			name:  "agent chain",
			path:  "/agents/oracle/models",
			value: "k3:max,adacode/gpt-5-3",
			want:  []string{"k3:max", "adacode/gpt-5-3"},
		},
		{
			name:        "category chain element",
			path:        "/categories/ultrabrain/models/0",
			value:       "spare:high",
			wantPointer: "/categories/ultrabrain/models",
			want:        []string{"spare:high"},
		},
		{name: "empty chain", path: "/model_profiles/daily/models", value: "", wantErrPart: "invalid chain"},
		{name: "empty json array", path: "/model_profiles/daily/models", value: "[]", wantErrPart: "at least one"},
		{
			name: "json array of non-strings", path: "/model_profiles/daily/models",
			value: "[1,2]", wantErrPart: "invalid chain",
		},
		{name: "empty comma entry", path: "/model_profiles/daily/models", value: "a/b, ", wantErrPart: "empty entry"},
		{name: "element with space", path: "/model_profiles/daily/models", value: "bad model", wantErrPart: "invalid chain entry"},
		{
			name: "json array with bad element", path: "/model_profiles/daily/models",
			value: `["ok/one","bad element"]`, wantErrPart: "invalid chain entry",
		},
		{
			name: "element index out of range", path: "/model_profiles/daily/models/9",
			value: "a/b", wantErrPart: "out of range",
		},
		{
			name: "element index not a number", path: "/model_profiles/daily/models/x",
			value: "a/b", wantErrPart: "invalid chain index",
		},
		{
			name: "element on missing chain", path: "/agents/oracle/models/0",
			value: "a/b", wantErrPart: "no chain",
		},
		{
			name: "missing entry", path: "/model_profiles/ghost/models",
			value: "a/b", wantErrPart: "no model_profiles entry",
		},
		{name: "not an editable path", path: "/models/k3/models", value: "a/b", wantErrPart: "not editable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ed, path, _ := loadEditor(t)
			err := ed.SetScalar(tt.path, tt.value)
			if tt.wantErrPart != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrPart) {
					t.Fatalf("SetScalar(%q, %q) error = %v, want substring %q", tt.path, tt.value, err, tt.wantErrPart)
				}
				return
			}
			if err != nil {
				t.Fatalf("SetScalar(%q, %q) error = %v", tt.path, tt.value, err)
			}
			if _, err := ed.Save(); err != nil {
				t.Fatalf("Save() error = %v", err)
			}
			pointer := tt.path
			if tt.wantPointer != "" {
				pointer = tt.wantPointer
			}
			got, found := rawValue(t, path, pointer)
			if !found {
				t.Fatalf("chain at %q missing after save", pointer)
			}
			if !jsonEqual(got, tt.want) {
				t.Errorf("chain at %q = %#v, want %#v", pointer, got, tt.want)
			}
		})
	}
}

func TestSetScalar_ModelProfile(t *testing.T) {
	t.Run("selects when the key was absent", func(t *testing.T) {
		path := writeConfig(t, "{\n  \"model_profiles\": {\n    \"solo\": {\n      \"display_name\": \"Solo\",\n    },\n  },\n}\n")
		ed, err := editor.Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if err := ed.SetScalar("/model_profile", "solo"); err != nil {
			t.Fatalf("SetScalar() error = %v", err)
		}
		if _, err := ed.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		if got, found := rawValue(t, path, "/model_profile"); !found || !jsonEqual(got, "solo") {
			t.Errorf("model_profile = %#v (found %v), want solo", got, found)
		}
	})

	t.Run("clears when no profiles exist", func(t *testing.T) {
		path := writeConfig(t, "{\n  \"model_profile\": \"ghost\",\n}\n")
		ed, err := editor.Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if err := ed.SetScalar("/model_profile", ""); err != nil {
			t.Fatalf("SetScalar() error = %v", err)
		}
		if got := ed.DirtyPaths(); !reflect.DeepEqual(got, []string{"/model_profile"}) {
			t.Errorf("DirtyPaths() = %v, want [/model_profile]", got)
		}
		if _, err := ed.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		if got, found := rawValue(t, path, "/model_profile"); found {
			t.Errorf("model_profile still present: %#v", got)
		}
	})

	t.Run("accepts lane key daily-normal", func(t *testing.T) {
		path := writeConfig(t, "{\n  \"model_profiles\": {\n    \"daily-normal\": {\n      \"display_name\": \"Daily Normal\",\n    },\n  },\n}\n")
		ed, err := editor.Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if err := ed.SetScalar("/model_profile", "daily-normal"); err != nil {
			t.Fatalf("SetScalar() error = %v", err)
		}
		if _, err := ed.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading saved config: %v", err)
		}
		if !strings.Contains(string(data), `"model_profile": "daily-normal"`) {
			t.Errorf("saved bytes lack the lane key:\n%s", data)
		}
		if got, found := rawValue(t, path, "/model_profile"); !found || !jsonEqual(got, "daily-normal") {
			t.Errorf("model_profile = %#v (found %v), want daily-normal", got, found)
		}
	})

	t.Run("accepts provider/model pin and renders read-only", func(t *testing.T) {
		path := writeConfig(t, "{\n  \"model_profiles\": {\n    \"daily-normal\": {\n      \"display_name\": \"Daily Normal\",\n    },\n  },\n  \"model_profile\": \"daily-normal\",\n}\n")
		ed, err := editor.Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if err := ed.SetScalar("/model_profile", "anthropic/claude-opus-5"); err != nil {
			t.Fatalf("SetScalar(pin) error = %v", err)
		}
		if _, err := ed.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading saved config: %v", err)
		}
		if !strings.Contains(string(data), `"model_profile": "anthropic/claude-opus-5"`) {
			t.Errorf("saved bytes lack the pin:\n%s", data)
		}
		sections := ed.Sections()
		profile, ok := sectionByID(sections, editor.SectionID("model_profile"))
		if !ok {
			t.Fatalf("Sections() has no model_profile section")
		}
		if !profile.ReadOnly {
			t.Errorf("model_profile section = %+v, want read-only", profile)
		}
		if len(profile.Entries) != 1 || profile.Entries[0].Value != "anthropic/claude-opus-5" || !profile.Entries[0].ReadOnly {
			t.Errorf("model_profile entries = %+v, want one read-only pin value", profile.Entries)
		}
		detail, err := ed.Detail(editor.SectionID("model_profile"), "model_profile")
		if err != nil {
			t.Fatalf("Detail() error = %v", err)
		}
		if len(detail.Lines) != 1 || detail.Lines[0].Value != "anthropic/claude-opus-5" || detail.Lines[0].Editable {
			t.Errorf("model_profile detail = %+v, want read-only pin value", detail)
		}
		// A fresh load (stale-state guard) sees the persisted pin.
		fresh, err := editor.Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		got, err := fresh.Detail(editor.SectionID("model_profile"), "model_profile")
		if err != nil {
			t.Fatalf("Detail() after reload error = %v", err)
		}
		if len(got.Lines) != 1 || got.Lines[0].Value != "anthropic/claude-opus-5" {
			t.Errorf("model_profile detail after reload = %+v, want pin value", got)
		}
	})

	t.Run("accepts pin without a profiles block", func(t *testing.T) {
		path := writeConfig(t, "{\n  \"model_profile\": \"daily\",\n}\n")
		ed, err := editor.Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if err := ed.SetScalar("/model_profile", "anthropic/claude-opus-5"); err != nil {
			t.Fatalf("SetScalar(pin) error = %v", err)
		}
		if _, err := ed.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		if got, found := rawValue(t, path, "/model_profile"); !found || !jsonEqual(got, "anthropic/claude-opus-5") {
			t.Errorf("model_profile = %#v (found %v), want pin", got, found)
		}
	})

	t.Run("rejects nonsense and leaves the file untouched", func(t *testing.T) {
		path := writeConfig(t, "{\n  \"model_profiles\": {\n    \"daily-normal\": {\n      \"display_name\": \"Daily Normal\",\n    },\n  },\n  \"model_profile\": \"daily-normal\",\n}\n")
		original, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading config: %v", err)
		}
		ed, err := editor.Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if err := ed.SetScalar("/model_profile", "nonsense"); err == nil {
			t.Fatal("SetScalar(nonsense) error = nil, want error")
		}
		if got := ed.DirtyPaths(); len(got) != 0 {
			t.Fatalf("DirtyPaths() = %v after rejected value, want empty", got)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading config: %v", err)
		}
		if !bytes.Equal(data, original) {
			t.Errorf("file changed after rejected value:\n%s", data)
		}
	})

	t.Run("malformed pins follow modelRefPattern", func(t *testing.T) {
		// modelRefPattern requires a non-empty segment after every
		// slash, so "x/" is rejected; it allows repeated /segments,
		// so "a/b/c" is accepted as a pin.
		cases := []struct {
			value     string
			wantError bool
		}{
			{value: "x/", wantError: true},
			{value: "/model", wantError: true},
			{value: "a/b/c", wantError: false},
		}
		for _, tc := range cases {
			path := writeConfig(t, "{\n  \"model_profiles\": {\n    \"daily-normal\": {\n      \"display_name\": \"Daily Normal\",\n    },\n  },\n  \"model_profile\": \"daily-normal\",\n}\n")
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading config: %v", err)
			}
			ed, err := editor.Load(path)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			err = ed.SetScalar("/model_profile", tc.value)
			if tc.wantError {
				if err == nil {
					t.Errorf("SetScalar(%q) error = nil, want error", tc.value)
				}
				data, readErr := os.ReadFile(path)
				if readErr != nil {
					t.Fatalf("reading config: %v", readErr)
				}
				if !bytes.Equal(data, original) {
					t.Errorf("file changed after rejected %q:\n%s", tc.value, data)
				}
				continue
			}
			if err != nil {
				t.Errorf("SetScalar(%q) error = %v, want nil", tc.value, err)
				continue
			}
			if _, err := ed.Save(); err != nil {
				t.Fatalf("Save() error = %v", err)
			}
			if got, found := rawValue(t, path, "/model_profile"); !found || !jsonEqual(got, tc.value) {
				t.Errorf("model_profile = %#v (found %v), want %q", got, found, tc.value)
			}
		}
	})
}

func TestMutations_InvalidInputLeavesDocumentUntouched(t *testing.T) {
	ops := []struct {
		name string
		op   func(ed *editor.Editor) error
	}{
		{"invalid model", func(ed *editor.Editor) error { return ed.SetScalar("/models/k3/model", "no-slash") }},
		{"invalid reasoning", func(ed *editor.Editor) error { return ed.SetScalar("/models/k3/reasoning", "extreme") }},
		{"unknown path", func(ed *editor.Editor) error { return ed.SetScalar("/models/k3/nope", "x") }},
		{"missing entry", func(ed *editor.Editor) error { return ed.SetScalar("/models/ghost/model", "a/b") }},
		{"empty chain", func(ed *editor.Editor) error { return ed.SetScalar("/model_profiles/daily/models", "[]") }},
		{"unknown profile", func(ed *editor.Editor) error { return ed.SetScalar("/model_profile", "ghost") }},
		{"blocked alias removal", func(ed *editor.Editor) error { return ed.RemoveEntry(editor.SectionModels, "spaceBunny") }},
		{"blocked profile removal", func(ed *editor.Editor) error { return ed.RemoveEntry(editor.SectionModelProfiles, "daily") }},
		{"unknown bool", func(ed *editor.Editor) error { return ed.ToggleBool("/nope/nope") }},
		{"toggle non-bool path", func(ed *editor.Editor) error { return ed.ToggleBool("/models/k3/model") }},
		{"invalid add key", func(ed *editor.Editor) error { return ed.AddEntry(editor.SectionModels, "bad key") }},
		{"unknown category", func(ed *editor.Editor) error { return ed.AddEntry(editor.SectionCategories, "custom") }},
		{"out of range move", func(ed *editor.Editor) error { return ed.MoveChain("/model_profiles/daily/models", 0, 9) }},
	}

	for _, tt := range ops {
		t.Run(tt.name, func(t *testing.T) {
			ed, path, original := loadEditor(t)
			if err := tt.op(ed); err == nil {
				t.Fatal("operation error = nil, want error")
			}
			if got := ed.DirtyPaths(); len(got) != 0 {
				t.Fatalf("DirtyPaths() = %v after failed operation, want empty", got)
			}
			backup, err := ed.Save()
			if err != nil {
				t.Fatalf("Save() error = %v", err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading saved config: %v", err)
			}
			if !bytes.Equal(data, original) {
				t.Errorf("file changed after failed operation:\n%s", data)
			}
			if backup == "" {
				t.Fatal("Save() backup path = empty, want a backup")
			}
			backupData, err := os.ReadFile(backup)
			if err != nil {
				t.Fatalf("reading backup: %v", err)
			}
			if !bytes.Equal(backupData, original) {
				t.Error("backup does not match the original file")
			}
		})
	}
}

func TestRemoveEntry_ReferenceBlocking(t *testing.T) {
	tests := []struct {
		name         string
		alias        string
		wantReferrer string
	}{
		{name: "agent model", alias: "spaceBunny", wantReferrer: "/agents/explore/model"},
		{name: "agent chain", alias: "agentChain", wantReferrer: "/agents/librarian/models/0"},
		{name: "category model", alias: "k3", wantReferrer: "/categories/quick/model"},
		{name: "category chain", alias: "catChain", wantReferrer: "/categories/ultrabrain/models/0"},
		{name: "profile chain", alias: "chainOnly", wantReferrer: "/model_profiles/daily/models/2"},
		{name: "second agent referrer", alias: "dsflash", wantReferrer: "/agents/oracle/model"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ed, path, _ := loadEditor(t)
			err := ed.RemoveEntry(editor.SectionModels, tt.alias)
			if err == nil {
				t.Fatalf("RemoveEntry(models, %q) error = nil, want blocking error", tt.alias)
			}
			if !strings.Contains(err.Error(), tt.wantReferrer) {
				t.Errorf("error = %q, want it to name referrer %q", err, tt.wantReferrer)
			}
			if _, err := ed.Save(); err != nil {
				t.Fatalf("Save() error = %v", err)
			}
			if _, found := rawValue(t, path, "/models/"+tt.alias); !found {
				t.Errorf("alias %q was removed despite the blocker", tt.alias)
			}
		})
	}

	t.Run("selected profile", func(t *testing.T) {
		ed, _, _ := loadEditor(t)
		err := ed.RemoveEntry(editor.SectionModelProfiles, "daily")
		if err == nil || !strings.Contains(err.Error(), "/model_profile") {
			t.Fatalf("RemoveEntry(model_profiles, daily) error = %v, want it to name /model_profile", err)
		}
	})

	t.Run("allowed removals", func(t *testing.T) {
		ed, path, _ := loadEditor(t)
		steps := []struct {
			section editor.SectionID
			key     string
			pointer string
		}{
			{editor.SectionModels, "spare", "/models/spare"},
			{editor.SectionModelProfiles, "cheap", "/model_profiles/cheap"},
			{editor.SectionAgents, "librarian", "/agents/librarian"},
			{editor.SectionCategories, "quick", "/categories/quick"},
		}
		for _, step := range steps {
			if err := ed.RemoveEntry(step.section, step.key); err != nil {
				t.Fatalf("RemoveEntry(%q, %q) error = %v", step.section, step.key, err)
			}
		}
		if _, err := ed.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		for _, step := range steps {
			if _, found := rawValue(t, path, step.pointer); found {
				t.Errorf("%q still present after removal", step.pointer)
			}
		}
		wantDirty := []string{
			"/categories/quick",
			"/model_profiles/cheap",
			"/models/spare",
			"/agents/librarian",
		}
		_ = wantDirty
		if got := ed.DirtyPaths(); len(got) != 0 {
			t.Errorf("DirtyPaths() = %v after Save, want empty", got)
		}
	})

	t.Run("missing entry", func(t *testing.T) {
		ed, _, _ := loadEditor(t)
		err := ed.RemoveEntry(editor.SectionModels, "ghost")
		if err == nil || !strings.Contains(err.Error(), "no models entry") {
			t.Fatalf("RemoveEntry error = %v, want missing entry error", err)
		}
	})

	t.Run("telemetry toggle cannot be removed", func(t *testing.T) {
		ed, _, _ := loadEditor(t)
		err := ed.RemoveEntry(editor.SectionTelemetry, "enabled")
		if err == nil || !strings.Contains(err.Error(), "not supported") {
			t.Fatalf("RemoveEntry error = %v, want not-supported error", err)
		}
	})
}

func TestToggleBool(t *testing.T) {
	t.Run("telemetry flip and flop", func(t *testing.T) {
		ed, path, _ := loadEditor(t)
		if err := ed.ToggleBool("/telemetry/enabled"); err != nil {
			t.Fatalf("ToggleBool() error = %v", err)
		}
		if got := ed.DirtyPaths(); !reflect.DeepEqual(got, []string{"/telemetry/enabled"}) {
			t.Errorf("DirtyPaths() = %v, want [/telemetry/enabled]", got)
		}
		if _, err := ed.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		if got, found := rawValue(t, path, "/telemetry/enabled"); !found || !jsonEqual(got, true) {
			t.Fatalf("telemetry enabled = %#v (found %v), want true", got, found)
		}
		if err := ed.ToggleBool("/telemetry/enabled"); err != nil {
			t.Fatalf("ToggleBool() error = %v", err)
		}
		if _, err := ed.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		if got, _ := rawValue(t, path, "/telemetry/enabled"); !jsonEqual(got, false) {
			t.Errorf("telemetry enabled = %#v, want false", got)
		}
	})

	t.Run("creates missing telemetry block", func(t *testing.T) {
		path := writeConfig(t, "{\n}\n")
		ed, err := editor.Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if err := ed.ToggleBool("/telemetry/enabled"); err != nil {
			t.Fatalf("ToggleBool() error = %v", err)
		}
		if _, err := ed.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		got, found := rawValue(t, path, "/telemetry")
		if !found || !jsonEqual(got, map[string]any{"enabled": true}) {
			t.Errorf("telemetry = %#v (found %v), want {\"enabled\": true}", got, found)
		}
	})

	t.Run("fills existing telemetry block", func(t *testing.T) {
		path := writeConfig(t, "{\n  \"telemetry\": {},\n}\n")
		ed, err := editor.Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if err := ed.ToggleBool("/telemetry/enabled"); err != nil {
			t.Fatalf("ToggleBool() error = %v", err)
		}
		if _, err := ed.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		if got, found := rawValue(t, path, "/telemetry/enabled"); !found || !jsonEqual(got, true) {
			t.Errorf("telemetry enabled = %#v (found %v), want true", got, found)
		}
	})

	t.Run("agent disable", func(t *testing.T) {
		ed, path, _ := loadEditor(t)
		if err := ed.ToggleBool("/agents/explore/disable"); err != nil {
			t.Fatalf("ToggleBool() error = %v", err)
		}
		if err := ed.ToggleBool("/agents/librarian/disable"); err != nil {
			t.Fatalf("ToggleBool() error = %v", err)
		}
		if _, err := ed.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		if got, found := rawValue(t, path, "/agents/explore/disable"); !found || !jsonEqual(got, true) {
			t.Errorf("explore disable = %#v (found %v), want true", got, found)
		}
		if got, found := rawValue(t, path, "/agents/librarian/disable"); !found || !jsonEqual(got, true) {
			t.Errorf("librarian disable = %#v (found %v), want true", got, found)
		}
	})

	t.Run("agent placeholder materializes", func(t *testing.T) {
		ed, path, _ := loadEditor(t)
		if err := ed.AddEntry(editor.SectionAgents, "fresh"); err != nil {
			t.Fatalf("AddEntry() error = %v", err)
		}
		if err := ed.ToggleBool("/agents/fresh/disable"); err != nil {
			t.Fatalf("ToggleBool() error = %v", err)
		}
		if _, err := ed.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		if got, found := rawValue(t, path, "/agents/fresh"); !found || !jsonEqual(got, map[string]any{"disable": true}) {
			t.Errorf("fresh agent = %#v (found %v), want {\"disable\": true}", got, found)
		}
	})

	t.Run("git master toggles", func(t *testing.T) {
		ed, path, _ := loadEditor(t)
		if err := ed.ToggleBool("/git_master/commit_footer"); err != nil {
			t.Fatalf("ToggleBool() error = %v", err)
		}
		if _, err := ed.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		if got, found := rawValue(t, path, "/git_master/commit_footer"); !found || !jsonEqual(got, true) {
			t.Errorf("commit_footer = %#v (found %v), want true", got, found)
		}

		path = writeConfig(t, "{\n}\n")
		ed, err := editor.Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if err := ed.ToggleBool("/git_master/include_co_authored_by"); err != nil {
			t.Fatalf("ToggleBool() error = %v", err)
		}
		if _, err := ed.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		got, found := rawValue(t, path, "/git_master")
		if !found || !jsonEqual(got, map[string]any{"include_co_authored_by": true}) {
			t.Errorf("git_master = %#v (found %v), want {\"include_co_authored_by\": true}", got, found)
		}
	})

	t.Run("errors", func(t *testing.T) {
		tests := []struct {
			name    string
			content string
			path    string
			wantErr string
		}{
			{name: "unknown path", path: "/foo/bar", wantErr: "unknown boolean path"},
			{name: "scalar path", path: "/models/k3/model", wantErr: "unknown boolean path"},
			{name: "missing agent", path: "/agents/ghost/disable", wantErr: "no agents entry"},
			{
				name:    "value not a boolean",
				content: "{\n  \"telemetry\": {\n    \"enabled\": \"yes\",\n  },\n}\n",
				path:    "/telemetry/enabled",
				wantErr: "not a boolean",
			},
			{
				name:    "agent disable not a boolean",
				content: "{\n  \"agents\": {\n    \"a\": {\n      \"disable\": 1,\n    },\n  },\n}\n",
				path:    "/agents/a/disable",
				wantErr: "not a boolean",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				ed, _, _ := loadEditor(t)
				if tt.content != "" {
					custom, err := editor.Load(writeConfig(t, tt.content))
					if err != nil {
						t.Fatalf("Load() error = %v", err)
					}
					ed = custom
				}
				err := ed.ToggleBool(tt.path)
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ToggleBool(%q) error = %v, want substring %q", tt.path, err, tt.wantErr)
				}
			})
		}
	})
}

func TestAddEntry(t *testing.T) {
	t.Run("adds placeholder to each section", func(t *testing.T) {
		sections := []editor.SectionID{
			editor.SectionModels,
			editor.SectionModelProfiles,
			editor.SectionAgents,
		}
		for _, section := range sections {
			t.Run(string(section), func(t *testing.T) {
				ed, path, _ := loadEditor(t)
				if err := ed.AddEntry(section, "fresh"); err != nil {
					t.Fatalf("AddEntry(%q, fresh) error = %v", section, err)
				}
				entryPath := "/" + string(section) + "/fresh"
				if got := ed.DirtyPaths(); !reflect.DeepEqual(got, []string{entryPath}) {
					t.Errorf("DirtyPaths() = %v, want [%s]", got, entryPath)
				}
				entry, found := entryByKey(mustSection(t, ed.Sections(), section).Entries, "fresh")
				if !found {
					t.Fatalf("entry fresh missing from section %s", section)
				}
				if entry.Value != "" || !entry.Dirty || entry.ReadOnly {
					t.Errorf("entry = %+v, want empty, dirty, editable", entry)
				}
				if section == editor.SectionModels {
					if err := ed.SetScalar(entryPath+"/model", "adacode/gpt-5-3"); err != nil {
						t.Fatalf("SetScalar() error = %v", err)
					}
				}
				if _, err := ed.Save(); err != nil {
					t.Fatalf("Save() error = %v", err)
				}
				got, found := rawValue(t, path, entryPath)
				if !found {
					t.Fatalf("value at %q missing after save", entryPath)
				}
				want := any("")
				if section == editor.SectionModels {
					want = map[string]any{"model": "adacode/gpt-5-3"}
				}
				if !jsonEqual(got, want) {
					t.Errorf("value at %q = %#v, want %#v", entryPath, got, want)
				}
			})
		}
	})

	t.Run("creates missing block", func(t *testing.T) {
		path := writeConfig(t, "{\n}\n")
		ed, err := editor.Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if err := ed.AddEntry(editor.SectionModelProfiles, "fresh"); err != nil {
			t.Fatalf("AddEntry() error = %v", err)
		}
		if _, err := ed.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		got, found := rawValue(t, path, "/model_profiles")
		if !found || !jsonEqual(got, map[string]any{"fresh": ""}) {
			t.Errorf("model_profiles = %#v (found %v), want {\"fresh\": \"\"}", got, found)
		}
	})

	t.Run("category keys are fixed", func(t *testing.T) {
		path := writeConfig(t, "{\n  \"categories\": {},\n}\n")
		ed, err := editor.Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if err := ed.AddEntry(editor.SectionCategories, "quick"); err != nil {
			t.Fatalf("AddEntry(categories, quick) error = %v", err)
		}
		if err := ed.AddEntry(editor.SectionCategories, "custom"); err == nil || !strings.Contains(err.Error(), "unknown category") {
			t.Fatalf("AddEntry(categories, custom) error = %v, want unknown category error", err)
		}
	})

	t.Run("errors", func(t *testing.T) {
		tests := []struct {
			name    string
			section editor.SectionID
			key     string
			wantErr string
		}{
			{name: "duplicate", section: editor.SectionModels, key: "k3", wantErr: "already exists"},
			{name: "key with space", section: editor.SectionModels, key: "bad key", wantErr: "invalid key"},
			{name: "key with slash", section: editor.SectionModels, key: "bad/key", wantErr: "invalid key"},
			{name: "empty key", section: editor.SectionAgents, key: "", wantErr: "invalid key"},
			{
				name: "key too long", section: editor.SectionModels, key: strings.Repeat("k", 33),
				wantErr: "invalid key",
			},
			{name: "telemetry", section: editor.SectionTelemetry, key: "enabled", wantErr: "does not support adding entries"},
			{name: "unknown section", section: "nope", key: "fresh", wantErr: "unknown section"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				ed, _, _ := loadEditor(t)
				err := ed.AddEntry(tt.section, tt.key)
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("AddEntry(%q, %q) error = %v, want substring %q", tt.section, tt.key, err, tt.wantErr)
				}
			})
		}
	})
}

func TestMoveChain(t *testing.T) {
	const dailyModels = "/model_profiles/daily/models"

	t.Run("moves element down", func(t *testing.T) {
		ed, path, _ := loadEditor(t)
		if err := ed.MoveChain(dailyModels, 0, 2); err != nil {
			t.Fatalf("MoveChain() error = %v", err)
		}
		if got := ed.DirtyPaths(); !reflect.DeepEqual(got, []string{dailyModels}) {
			t.Errorf("DirtyPaths() = %v, want [%s]", got, dailyModels)
		}
		if _, err := ed.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		want := []string{"spaceBunny:max", "chainOnly:high", "github-copilot/claude-sonnet-5:max"}
		if got, found := rawValue(t, path, dailyModels); !found || !jsonEqual(got, want) {
			t.Errorf("chain = %#v (found %v), want %v", got, found, want)
		}
	})

	t.Run("moves element up", func(t *testing.T) {
		ed, path, _ := loadEditor(t)
		if err := ed.MoveChain(dailyModels, 2, 0); err != nil {
			t.Fatalf("MoveChain() error = %v", err)
		}
		if _, err := ed.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		want := []string{"chainOnly:high", "github-copilot/claude-sonnet-5:max", "spaceBunny:max"}
		if got, found := rawValue(t, path, dailyModels); !found || !jsonEqual(got, want) {
			t.Errorf("chain = %#v (found %v), want %v", got, found, want)
		}
	})

	t.Run("equal indexes are a no-op", func(t *testing.T) {
		ed, _, _ := loadEditor(t)
		if err := ed.MoveChain(dailyModels, 1, 1); err != nil {
			t.Fatalf("MoveChain() error = %v", err)
		}
		if got := ed.DirtyPaths(); len(got) != 0 {
			t.Errorf("DirtyPaths() = %v, want empty", got)
		}
	})

	t.Run("keeps duplicates", func(t *testing.T) {
		path := writeConfig(t, "{\n  \"model_profiles\": {\n    \"p\": {\n      \"display_name\": \"P\",\n      \"models\": [\"a/b\", \"a/b\", \"c/d\"],\n    },\n  },\n}\n")
		ed, err := editor.Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if err := ed.MoveChain("/model_profiles/p/models", 0, 2); err != nil {
			t.Fatalf("MoveChain() error = %v", err)
		}
		if _, err := ed.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		want := []string{"a/b", "c/d", "a/b"}
		if got, found := rawValue(t, path, "/model_profiles/p/models"); !found || !jsonEqual(got, want) {
			t.Errorf("chain = %#v (found %v), want %v", got, found, want)
		}
	})

	t.Run("agent chain", func(t *testing.T) {
		path := writeConfig(t, "{\n  \"agents\": {\n    \"a\": {\n      \"models\": [\"a/b\", \"c/d\"],\n    },\n  },\n}\n")
		ed, err := editor.Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if err := ed.MoveChain("/agents/a/models", 0, 1); err != nil {
			t.Fatalf("MoveChain() error = %v", err)
		}
		if _, err := ed.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		want := []string{"c/d", "a/b"}
		if got, found := rawValue(t, path, "/agents/a/models"); !found || !jsonEqual(got, want) {
			t.Errorf("chain = %#v (found %v), want %v", got, found, want)
		}
	})

	t.Run("errors", func(t *testing.T) {
		tests := []struct {
			name    string
			path    string
			from    int
			to      int
			wantErr string
		}{
			{name: "from out of range", path: dailyModels, from: 0, to: 3, wantErr: "out of range"},
			{name: "to out of range", path: dailyModels, from: 3, to: 0, wantErr: "out of range"},
			{name: "negative from", path: dailyModels, from: -1, to: 0, wantErr: "out of range"},
			{name: "negative to", path: dailyModels, from: 0, to: -1, wantErr: "out of range"},
			{name: "not a chain path", path: "/agents/explore/model", from: 0, to: 1, wantErr: "not an editable chain"},
			{name: "scalar path", path: "/model_profile", from: 0, to: 1, wantErr: "not an editable chain"},
			{name: "missing chain", path: "/model_profiles/ghost/models", from: 0, to: 1, wantErr: "no chain"},
			{name: "missing models field", path: "/agents/oracle/models", from: 0, to: 1, wantErr: "no chain"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				ed, _, _ := loadEditor(t)
				err := ed.MoveChain(tt.path, tt.from, tt.to)
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("MoveChain(%q, %d, %d) error = %v, want substring %q", tt.path, tt.from, tt.to, err, tt.wantErr)
				}
			})
		}

		t.Run("models is not an array", func(t *testing.T) {
			path := writeConfig(t, "{\n  \"agents\": {\n    \"a\": {\n      \"models\": \"a/b\",\n    },\n  },\n}\n")
			ed, err := editor.Load(path)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if err := ed.MoveChain("/agents/a/models", 0, 1); err == nil || !strings.Contains(err.Error(), "not a chain") {
				t.Fatalf("MoveChain() error = %v, want not-a-chain error", err)
			}
		})
	})
}

func TestDirtyPaths(t *testing.T) {
	t.Run("clean editor", func(t *testing.T) {
		ed, _, _ := loadEditor(t)
		if got := ed.DirtyPaths(); len(got) != 0 {
			t.Errorf("DirtyPaths() = %v, want empty", got)
		}
	})

	t.Run("deduplicates and sorts", func(t *testing.T) {
		ed, _, _ := loadEditor(t)
		if err := ed.SetScalar("/models/k3/model", "adacode/claude-opus-4-6"); err != nil {
			t.Fatalf("SetScalar() error = %v", err)
		}
		if err := ed.SetScalar("/models/k3/model", "adacode/claude-opus-5"); err != nil {
			t.Fatalf("SetScalar() error = %v", err)
		}
		if err := ed.SetScalar("/models/k3/reasoning", "low"); err != nil {
			t.Fatalf("SetScalar() error = %v", err)
		}
		if err := ed.ToggleBool("/telemetry/enabled"); err != nil {
			t.Fatalf("ToggleBool() error = %v", err)
		}
		if err := ed.AddEntry(editor.SectionAgents, "extra"); err != nil {
			t.Fatalf("AddEntry() error = %v", err)
		}
		want := []string{"/agents/extra", "/models/k3/model", "/models/k3/reasoning", "/telemetry/enabled"}
		if got := ed.DirtyPaths(); !reflect.DeepEqual(got, want) {
			t.Errorf("DirtyPaths() = %v, want %v", got, want)
		}
	})

	t.Run("save clears", func(t *testing.T) {
		ed, _, _ := loadEditor(t)
		if err := ed.SetScalar("/models/k3/model", "adacode/claude-opus-4-6"); err != nil {
			t.Fatalf("SetScalar() error = %v", err)
		}
		if _, err := ed.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		if got := ed.DirtyPaths(); len(got) != 0 {
			t.Errorf("DirtyPaths() = %v after Save, want empty", got)
		}
	})

	t.Run("reload clears", func(t *testing.T) {
		ed, _, _ := loadEditor(t)
		if err := ed.SetScalar("/models/k3/model", "adacode/claude-opus-4-6"); err != nil {
			t.Fatalf("SetScalar() error = %v", err)
		}
		if err := ed.Reload(); err != nil {
			t.Fatalf("Reload() error = %v", err)
		}
		if got := ed.DirtyPaths(); len(got) != 0 {
			t.Errorf("DirtyPaths() = %v after Reload, want empty", got)
		}
	})

	t.Run("removal drops descendants", func(t *testing.T) {
		ed, _, _ := loadEditor(t)
		if err := ed.SetScalar("/models/spare/model", "adacode/x"); err != nil {
			t.Fatalf("SetScalar() error = %v", err)
		}
		if err := ed.RemoveEntry(editor.SectionModels, "spare"); err != nil {
			t.Fatalf("RemoveEntry() error = %v", err)
		}
		want := []string{"/models/spare"}
		if got := ed.DirtyPaths(); !reflect.DeepEqual(got, want) {
			t.Errorf("DirtyPaths() = %v, want %v", got, want)
		}
	})
}

func TestSave_BackupLifecycle(t *testing.T) {
	ed, path, original := loadEditor(t)
	if err := ed.SetScalar("/models/k3/model", "adacode/claude-opus-4-6"); err != nil {
		t.Fatalf("SetScalar() error = %v", err)
	}

	backup, err := ed.Save()
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if backup == "" {
		t.Fatal("Save() backup path = empty, want a backup path")
	}
	if !strings.HasPrefix(filepath.Base(backup), "omo.jsonc.bak.") {
		t.Errorf("backup = %q, want name prefixed omo.jsonc.bak.", backup)
	}
	backupData, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if !bytes.Equal(backupData, original) {
		t.Error("backup does not hold the original file bytes")
	}
	if got := ed.DirtyPaths(); len(got) != 0 {
		t.Errorf("DirtyPaths() = %v after Save, want empty", got)
	}
	if got, found := rawValue(t, path, "/models/k3/model"); !found || !jsonEqual(got, "adacode/claude-opus-4-6") {
		t.Fatalf("saved model = %#v (found %v), want adacode/claude-opus-4-6", got, found)
	}
	afterFirst, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}

	if err := ed.ToggleBool("/telemetry/enabled"); err != nil {
		t.Fatalf("ToggleBool() error = %v", err)
	}
	backup2, err := ed.Save()
	if err != nil {
		t.Fatalf("second Save() error = %v", err)
	}
	if backup2 == "" || backup2 == backup {
		t.Fatalf("second backup = %q, want a new backup path", backup2)
	}
	backup2Data, err := os.ReadFile(backup2)
	if err != nil {
		t.Fatalf("reading second backup: %v", err)
	}
	if !bytes.Equal(backup2Data, afterFirst) {
		t.Error("second backup does not hold the file state before the second save")
	}
}

func TestSave_RefusesEmptyModel(t *testing.T) {
	ed, path, original := loadEditor(t)
	if err := ed.AddEntry(editor.SectionModels, "emptyOne"); err != nil {
		t.Fatalf("AddEntry() error = %v", err)
	}
	backup, err := ed.Save()
	if err == nil || !strings.Contains(err.Error(), "emptyOne") {
		t.Fatalf("Save() error = %v, want refusal naming emptyOne", err)
	}
	if backup != "" {
		t.Errorf("Save() backup = %q, want empty on refusal", backup)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	if !bytes.Equal(data, original) {
		t.Error("file changed despite the refusal")
	}
	if got := ed.DirtyPaths(); !reflect.DeepEqual(got, []string{"/models/emptyOne"}) {
		t.Errorf("DirtyPaths() = %v, want [/models/emptyOne]", got)
	}

	if err := ed.SetScalar("/models/emptyOne/model", "adacode/gpt-5-3"); err != nil {
		t.Fatalf("SetScalar() error = %v", err)
	}
	if backup, err := ed.Save(); err != nil || backup == "" {
		t.Fatalf("Save() after filling the model = (%q, %v), want a backup", backup, err)
	}
}

func TestReload(t *testing.T) {
	ed, path, _ := loadEditor(t)
	if err := ed.SetScalar("/models/k3/model", "adacode/claude-opus-4-6"); err != nil {
		t.Fatalf("SetScalar() error = %v", err)
	}
	if err := ed.Reload(); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	detail, err := ed.Detail(editor.SectionModels, "k3")
	if err != nil {
		t.Fatalf("Detail() error = %v", err)
	}
	if detail.Lines[0].Value != "github-copilot/claude-sonnet-5" {
		t.Errorf("model after Reload = %q, want the on-disk value", detail.Lines[0].Value)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	updated := strings.Replace(string(data), "github-copilot/claude-sonnet-5", "adacode/claude-opus-4-6", 1)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatalf("writing external change: %v", err)
	}
	if err := ed.Reload(); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	detail, err = ed.Detail(editor.SectionModels, "k3")
	if err != nil {
		t.Fatalf("Detail() error = %v", err)
	}
	if detail.Lines[0].Value != "adacode/claude-opus-4-6" {
		t.Errorf("model after external change = %q, want adacode/claude-opus-4-6", detail.Lines[0].Value)
	}
}

func TestEndToEnd_EditSaveReloadPreservesComments(t *testing.T) {
	ed, path, original := loadEditor(t)

	steps := []struct {
		name string
		op   func() error
	}{
		{"set model", func() error { return ed.SetScalar("/models/k3/model", "github-copilot/claude-opus-5") }},
		{"set reasoning", func() error { return ed.SetScalar("/models/k3/reasoning", "low") }},
		{"toggle telemetry", func() error { return ed.ToggleBool("/telemetry/enabled") }},
		{"add alias", func() error { return ed.AddEntry(editor.SectionModels, "freshAlias") }},
		{"fill new alias", func() error { return ed.SetScalar("/models/freshAlias/model", "adacode/gpt-5-3") }},
		{"move chain", func() error { return ed.MoveChain("/model_profiles/daily/models", 0, 2) }},
		{"toggle agent disable", func() error { return ed.ToggleBool("/agents/explore/disable") }},
		{"remove profile", func() error { return ed.RemoveEntry(editor.SectionModelProfiles, "cheap") }},
	}
	for _, step := range steps {
		if err := step.op(); err != nil {
			t.Fatalf("step %q: %v", step.name, err)
		}
	}

	wantDirty := []string{
		"/agents/explore/disable",
		"/model_profiles/cheap",
		"/model_profiles/daily/models",
		"/models/freshAlias",
		"/models/freshAlias/model",
		"/models/k3/model",
		"/models/k3/reasoning",
		"/telemetry/enabled",
	}
	if got := ed.DirtyPaths(); !reflect.DeepEqual(got, wantDirty) {
		t.Fatalf("DirtyPaths() = %v, want %v", got, wantDirty)
	}

	backup, err := ed.Save()
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if backup == "" {
		t.Fatal("Save() backup path = empty")
	}
	if backupData, err := os.ReadFile(backup); err != nil || !bytes.Equal(backupData, original) {
		t.Errorf("backup does not hold the original bytes (err %v)", err)
	}

	checks := []struct {
		pointer string
		want    any
	}{
		{"/models/k3/model", "github-copilot/claude-opus-5"},
		{"/models/k3/reasoning", "low"},
		{"/telemetry/enabled", true},
		{"/models/freshAlias/model", "adacode/gpt-5-3"},
		{"/agents/explore/disable", true},
		{
			"/model_profiles/daily/models",
			[]string{"spaceBunny:max", "chainOnly:high", "github-copilot/claude-sonnet-5:max"},
		},
	}
	for _, check := range checks {
		got, found := rawValue(t, path, check.pointer)
		if !found || !jsonEqual(got, check.want) {
			t.Errorf("value at %q = %#v (found %v), want %#v", check.pointer, got, found, check.want)
		}
	}
	if got, found := rawValue(t, path, "/model_profiles/cheap"); found {
		t.Errorf("removed profile still present: %#v", got)
	}
	if got := ed.DirtyPaths(); len(got) != 0 {
		t.Errorf("DirtyPaths() = %v after Save, want empty", got)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved config: %v", err)
	}
	for _, comment := range fixtureComments {
		if !strings.Contains(string(data), comment) {
			t.Errorf("comment %q did not survive the save\nfile:\n%s", comment, data)
		}
	}

	reloaded, err := editor.Load(path)
	if err != nil {
		t.Fatalf("reload after save error = %v", err)
	}
	sections := reloaded.Sections()
	k3, _ := entryByKey(mustSection(t, sections, editor.SectionModels).Entries, "k3")
	if k3.Value != "github-copilot/claude-opus-5" {
		t.Errorf("reloaded k3.Value = %q, want github-copilot/claude-opus-5", k3.Value)
	}
	fresh, found := entryByKey(mustSection(t, sections, editor.SectionModels).Entries, "freshAlias")
	if !found || fresh.Value != "adacode/gpt-5-3" {
		t.Errorf("reloaded freshAlias = %+v (found %v), want model adacode/gpt-5-3", fresh, found)
	}
	telemetry := mustSection(t, sections, editor.SectionTelemetry)
	if telemetry.Entries[0].Value != "true" {
		t.Errorf("reloaded telemetry = %q, want true", telemetry.Entries[0].Value)
	}
	if _, found := entryByKey(mustSection(t, sections, editor.SectionModelProfiles).Entries, "cheap"); found {
		t.Error("removed profile still listed after reload")
	}
	dailyDetail, err := reloaded.Detail(editor.SectionModelProfiles, "daily")
	if err != nil {
		t.Fatalf("Detail() error = %v", err)
	}
	wantChain := `["spaceBunny:max","chainOnly:high","github-copilot/claude-sonnet-5:max"]`
	if dailyDetail.Lines[1].Value != wantChain {
		t.Errorf("reloaded daily chain = %q, want %q", dailyDetail.Lines[1].Value, wantChain)
	}
}

func TestDetail_GitMaster(t *testing.T) {
	ed, _, _ := loadEditor(t)
	for _, key := range []string{"commit_footer", "include_co_authored_by"} {
		got, err := ed.Detail(editor.SectionGitMaster, key)
		if err != nil {
			t.Fatalf("Detail(git_master, %q) error = %v", key, err)
		}
		if got.Title != key {
			t.Errorf("Detail title = %q, want %q", got.Title, key)
		}
		if len(got.Lines) != 1 {
			t.Fatalf("Detail lines = %d, want 1", len(got.Lines))
		}
		line := got.Lines[0]
		if line.Label != key || line.Path != "/git_master/"+key {
			t.Errorf("Detail line = %+v, want label/path for %q", line, key)
		}
		if line.Value != "false" || !line.Bool || !line.Editable {
			t.Errorf("Detail line = %+v, want false editable bool", line)
		}
	}
	if _, err := ed.Detail(editor.SectionGitMaster, "signing"); err == nil {
		t.Error("Detail(git_master, signing) error = nil, want error")
	}
}

func TestDetail_ReadOnly(t *testing.T) {
	t.Run("scalar keys stay single read-only lines", func(t *testing.T) {
		ed, _, _ := loadEditor(t)
		got, err := ed.Detail(editor.SectionID("$schema"), "$schema")
		if err != nil {
			t.Fatalf("Detail() error = %v", err)
		}
		if got.Title != "$schema" || len(got.Lines) != 1 {
			t.Fatalf("Detail() = %+v, want one line titled $schema", got)
		}
		line := got.Lines[0]
		if line.Value != "https://example.invalid/omo.schema.json" || line.Editable || line.Bool {
			t.Errorf("Detail line = %+v, want read-only non-bool URL", line)
		}
		profile, err := ed.Detail(editor.SectionID("model_profile"), "model_profile")
		if err != nil {
			t.Fatalf("Detail() error = %v", err)
		}
		if len(profile.Lines) != 1 || profile.Lines[0].Value != "daily" || profile.Lines[0].Editable {
			t.Errorf("model_profile detail = %+v, want read-only value daily", profile)
		}
	})
	t.Run("unusual JSON types survive previews", func(t *testing.T) {
		path := writeConfig(t, "{\n  \"arr\": [1, \"two\", null],\n  \"nothing\": null,\n  \"count\": 42,\n  \"flag\": true,\n  \"obj\": {\n    \"b\": 2,\n    \"a\": \"x\",\n  },\n}\n")
		ed, err := editor.Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		sections := ed.Sections()
		// 6 editable + 5 read-only keys.
		if len(sections) != 11 {
			t.Fatalf("Sections() returned %d sections, want 11", len(sections))
		}
		for _, section := range sections[6:] {
			if !section.ReadOnly {
				t.Errorf("section %s unexpectedly editable", section.ID)
			}
			if len(section.Entries) != 1 || !section.Entries[0].ReadOnly {
				t.Errorf("section %s entries = %+v, want one read-only preview", section.ID, section.Entries)
			}
		}
		arr, _ := sectionByID(sections, editor.SectionID("arr"))
		if arr.Entries[0].Value != `[1,"two",null]` {
			t.Errorf("arr preview = %q, want compact JSON", arr.Entries[0].Value)
		}
		nothing, _ := sectionByID(sections, editor.SectionID("nothing"))
		if nothing.Entries[0].Value != "" {
			t.Errorf("null preview = %q, want empty", nothing.Entries[0].Value)
		}
		obj, err := ed.Detail(editor.SectionID("obj"), "obj")
		if err != nil {
			t.Fatalf("Detail() error = %v", err)
		}
		if len(obj.Lines) != 2 || obj.Lines[0].Label != "a" || obj.Lines[1].Label != "b" {
			t.Errorf("obj detail = %+v, want sorted a/b lines", obj)
		}
		for _, line := range obj.Lines {
			if line.Editable {
				t.Errorf("obj detail line = %+v, want read-only", line)
			}
		}
		flag, err := ed.Detail(editor.SectionID("flag"), "flag")
		if err != nil {
			t.Fatalf("Detail() error = %v", err)
		}
		if len(flag.Lines) != 1 || flag.Lines[0].Value != "true" || !flag.Lines[0].Bool || flag.Lines[0].Editable {
			t.Errorf("flag detail = %+v, want read-only true bool", flag)
		}
	})
}

func TestAddRemoveEntry_GitMaster(t *testing.T) {
	ed, _, _ := loadEditor(t)
	if err := ed.AddEntry(editor.SectionGitMaster, "signing"); err == nil {
		t.Error("AddEntry(git_master) error = nil, want error")
	}
	for _, key := range []string{"commit_footer", "include_co_authored_by", "ghost"} {
		if err := ed.RemoveEntry(editor.SectionGitMaster, key); err == nil {
			t.Errorf("RemoveEntry(git_master, %q) error = nil, want error", key)
		}
	}
}

func TestReload_RefreshesSections(t *testing.T) {
	path := writeConfig(t, "{\n  \"telemetry\": {\n    \"enabled\": false,\n  },\n}\n")
	ed, err := editor.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if _, found := sectionByID(ed.Sections(), editor.SectionID("$schema")); found {
		t.Fatal("schema section present before the external change")
	}
	if err := os.WriteFile(path, []byte("{\n  \"$schema\": \"https://example.invalid/v2\",\n  \"git_master\": {\n    \"commit_footer\": true,\n  },\n}\n"), 0o644); err != nil {
		t.Fatalf("writing external change: %v", err)
	}
	if err := ed.Reload(); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	sections := ed.Sections()
	schema, found := sectionByID(sections, editor.SectionID("$schema"))
	if !found {
		t.Fatal("schema section missing after Reload")
	}
	if len(schema.Entries) != 1 || schema.Entries[0].Value != "https://example.invalid/v2" {
		t.Errorf("schema entries = %+v, want the new URL", schema.Entries)
	}
	git := mustSection(t, sections, editor.SectionGitMaster)
	if git.Entries[0].Value != "true" || git.Entries[1].Value != "false" {
		t.Errorf("git entries = %+v, want commit_footer true and co-authored false", git.Entries)
	}
	if len(ed.DirtyPaths()) != 0 {
		t.Errorf("DirtyPaths() = %v, want clean after reload", ed.DirtyPaths())
	}
}
