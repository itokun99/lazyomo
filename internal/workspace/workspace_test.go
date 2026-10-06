package workspace_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/itokun99/lazyomo/internal/editor"
	"github.com/itokun99/lazyomo/internal/workspace"
)

// Valid layer files used across the registry tests: the registry only needs
// loadable JSONC, so the contents stay minimal.
const (
	validUserConfig    = `{"models": {"k3": {"model": "acme/code-large"}}}`
	validProjectConfig = `{"agents": {"sisyphus": {"model": "acme/code-large"}}}`
	validMCPConfig     = `{"mcpServers": {"fetch": {"type": "stdio", "command": "npx"}}}`
)

// fixture is one synthetic home directory with a nested project directory and
// the agent directory, mirroring the real layout: <home>/.omo/omo.jsonc,
// <cwd>/.omo/omo.jsonc, and <agentDir>/mcp.json.
type fixture struct {
	home     string
	cwd      string
	agentDir string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	home := t.TempDir()
	f := &fixture{
		home:     home,
		cwd:      filepath.Join(home, "work", "proj"),
		agentDir: filepath.Join(home, ".omo", "agent"),
	}
	f.mkdirs(t, f.cwd)
	f.mkdirs(t, f.agentDir)
	return f
}

func (f *fixture) mkdirs(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}

func (f *fixture) write(t *testing.T, path, content string) {
	t.Helper()
	f.mkdirs(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func (f *fixture) userPath() string    { return filepath.Join(f.home, ".omo", "omo.jsonc") }
func (f *fixture) projectPath() string { return filepath.Join(f.cwd, ".omo", "omo.jsonc") }
func (f *fixture) mcpPath() string     { return filepath.Join(f.agentDir, "mcp.json") }

func (f *fixture) writeAll(t *testing.T) {
	t.Helper()
	f.write(t, f.userPath(), validUserConfig)
	f.write(t, f.projectPath(), validProjectConfig)
	f.write(t, f.mcpPath(), validMCPConfig)
}

func (f *fixture) registry() *workspace.Registry {
	return workspace.NewRegistry(workspace.RegistryConfig{
		HomeDir:  f.home,
		Cwd:      f.cwd,
		AgentDir: f.agentDir,
	})
}

func sourceByID(t *testing.T, reg *workspace.Registry, id string) workspace.Source {
	t.Helper()
	src, ok := reg.Source(id)
	if !ok {
		t.Fatalf("Source(%q) not found; sources = %#v", id, reg.Sources())
	}
	return src
}

func diagnosticsFor(reg *workspace.Registry, sourceID string) []workspace.Diagnostic {
	var out []workspace.Diagnostic
	for _, diagnostic := range reg.Diagnostics() {
		if diagnostic.SourceID == sourceID {
			out = append(out, diagnostic)
		}
	}
	return out
}

func requireDiagnostic(t *testing.T, reg *workspace.Registry, sourceID string) {
	t.Helper()
	diagnostics := diagnosticsFor(reg, sourceID)
	if len(diagnostics) == 0 {
		t.Fatalf("no %s diagnostic; diagnostics = %#v", sourceID, reg.Diagnostics())
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Reason == "" {
			t.Errorf("%s diagnostic has an empty reason: %#v", sourceID, diagnostic)
		}
	}
}

func checkSource(t *testing.T, src workspace.Source, id, path string, kind workspace.Kind, schema workspace.Schema, writable bool) {
	t.Helper()
	if src.ID != id {
		t.Errorf("source.ID = %q, want %q", src.ID, id)
	}
	if src.Path != path {
		t.Errorf("source %s Path = %q, want %q", id, src.Path, path)
	}
	if src.Kind != kind {
		t.Errorf("source %s Kind = %q, want %q", id, src.Kind, kind)
	}
	if src.Schema != schema {
		t.Errorf("source %s Schema = %q, want %q", id, src.Schema, schema)
	}
	if src.Writable != writable {
		t.Errorf("source %s Writable = %v, want %v", id, src.Writable, writable)
	}
}

func rowByPointer(rows []workspace.Row, pointer string) (workspace.Row, bool) {
	for _, row := range rows {
		if row.Pointer == pointer {
			return row, true
		}
	}
	return workspace.Row{}, false
}

func TestRegistry(t *testing.T) {
	t.Run("all three layers present", func(t *testing.T) {
		f := newFixture(t)
		f.writeAll(t)
		reg := f.registry()

		if sources := reg.Sources(); len(sources) != 3 {
			t.Fatalf("Sources() = %#v, want 3 sources", sources)
		}
		if diagnostics := reg.Diagnostics(); len(diagnostics) != 0 {
			t.Fatalf("Diagnostics() = %#v, want none", diagnostics)
		}

		user := sourceByID(t, reg, "user")
		checkSource(t, user, "user", f.userPath(), workspace.KindUser, workspace.SchemaOmo, true)
		if user.Session == nil {
			t.Fatal("user session is nil, want the loaded editor")
		}
		if got := user.Session.Path(); got != f.userPath() {
			t.Errorf("user session Path() = %q, want %q", got, f.userPath())
		}
		ed, ok := user.Session.(*editor.Editor)
		if !ok {
			t.Fatalf("user session type = %T, want *editor.Editor", user.Session)
		}
		rows := workspace.EditorRows(user.ID, ed.Sections())
		row, found := rowByPointer(rows, "/models/k3")
		if !found {
			t.Fatalf("EditorRows() = %#v, want a /models/k3 row", rows)
		}
		if row.Provenance != workspace.ProvenanceEditable {
			t.Errorf("row /models/k3 provenance = %q, want %q", row.Provenance, workspace.ProvenanceEditable)
		}

		project := sourceByID(t, reg, "project")
		checkSource(t, project, "project", f.projectPath(), workspace.KindProject, workspace.SchemaOmo, false)
		if project.Session != nil {
			t.Errorf("project session = %v, want nil for a read-only source", project.Session)
		}

		mcp := sourceByID(t, reg, "mcp")
		checkSource(t, mcp, "mcp", f.mcpPath(), workspace.KindExternal, workspace.SchemaMCPServers, true)
		if mcp.Session != nil {
			t.Errorf("mcp session = %v, want nil until the mcpfile session is wired", mcp.Session)
		}

		// Logged for the evidence artifact: the structural assertions above
		// are the truth; these lines make the captured -v output readable.
		t.Logf("sources: %d", len(reg.Sources()))
		t.Logf("user: path=%s kind=%s schema=%s writable=%v session=%T", user.Path, user.Kind, user.Schema, user.Writable, user.Session)
		t.Logf("project: path=%s kind=%s schema=%s writable=%v session=%T", project.Path, project.Kind, project.Schema, project.Writable, project.Session)
		t.Logf("mcp: path=%s kind=%s schema=%s writable=%v session=%T", mcp.Path, mcp.Kind, mcp.Schema, mcp.Writable, mcp.Session)
		t.Logf("provenance: /models/k3=%s", row.Provenance)
	})

	t.Run("no layers present", func(t *testing.T) {
		f := newFixture(t)
		reg := f.registry()

		if sources := reg.Sources(); len(sources) != 0 {
			t.Fatalf("Sources() = %#v, want none", sources)
		}
		if diagnostics := reg.Diagnostics(); len(diagnostics) != 3 {
			t.Fatalf("Diagnostics() = %#v, want one per layer", diagnostics)
		}
		requireDiagnostic(t, reg, "user")
		requireDiagnostic(t, reg, "project")
		requireDiagnostic(t, reg, "mcp")
	})

	t.Run("missing user config yields a diagnostic", func(t *testing.T) {
		f := newFixture(t)
		f.write(t, f.projectPath(), validProjectConfig)
		f.write(t, f.mcpPath(), validMCPConfig)
		reg := f.registry()

		if _, ok := reg.Source("user"); ok {
			t.Error("user source registered without a config file")
		}
		requireDiagnostic(t, reg, "user")
		sourceByID(t, reg, "project")
		sourceByID(t, reg, "mcp")
	})

	t.Run("malformed user config yields a diagnostic", func(t *testing.T) {
		f := newFixture(t)
		f.write(t, f.userPath(), "{not jsonc")
		f.write(t, f.projectPath(), validProjectConfig)
		f.write(t, f.mcpPath(), validMCPConfig)
		reg := f.registry()

		if _, ok := reg.Source("user"); ok {
			t.Error("user source registered from a malformed config")
		}
		requireDiagnostic(t, reg, "user")
		sourceByID(t, reg, "project")
		sourceByID(t, reg, "mcp")
	})

	t.Run("unreadable user config yields a diagnostic", func(t *testing.T) {
		f := newFixture(t)
		f.mkdirs(t, f.userPath())
		f.write(t, f.projectPath(), validProjectConfig)
		f.write(t, f.mcpPath(), validMCPConfig)
		reg := f.registry()

		if _, ok := reg.Source("user"); ok {
			t.Error("user source registered from an unreadable path")
		}
		requireDiagnostic(t, reg, "user")
		sourceByID(t, reg, "project")
		sourceByID(t, reg, "mcp")
	})

	t.Run("missing project layer yields a diagnostic", func(t *testing.T) {
		f := newFixture(t)
		f.write(t, f.userPath(), validUserConfig)
		f.write(t, f.mcpPath(), validMCPConfig)
		reg := f.registry()

		if _, ok := reg.Source("project"); ok {
			t.Error("project source registered without a project config")
		}
		requireDiagnostic(t, reg, "project")
		sourceByID(t, reg, "user")
		sourceByID(t, reg, "mcp")
	})

	t.Run("malformed project config yields a diagnostic", func(t *testing.T) {
		f := newFixture(t)
		f.write(t, f.userPath(), validUserConfig)
		f.write(t, f.projectPath(), "{not jsonc")
		f.write(t, f.mcpPath(), validMCPConfig)
		reg := f.registry()

		if _, ok := reg.Source("project"); ok {
			t.Error("project source registered from a malformed config")
		}
		requireDiagnostic(t, reg, "project")
		sourceByID(t, reg, "user")
		sourceByID(t, reg, "mcp")
	})

	t.Run("unreadable project config yields a diagnostic", func(t *testing.T) {
		f := newFixture(t)
		f.write(t, f.userPath(), validUserConfig)
		f.mkdirs(t, f.projectPath())
		f.write(t, f.mcpPath(), validMCPConfig)
		reg := f.registry()

		if _, ok := reg.Source("project"); ok {
			t.Error("project source registered from an unreadable path")
		}
		requireDiagnostic(t, reg, "project")
		sourceByID(t, reg, "user")
		sourceByID(t, reg, "mcp")
	})

	t.Run("symlinked project config is skipped", func(t *testing.T) {
		f := newFixture(t)
		f.write(t, f.userPath(), validUserConfig)
		f.write(t, f.mcpPath(), validMCPConfig)
		target := filepath.Join(f.home, "elsewhere.jsonc")
		f.write(t, target, validProjectConfig)
		f.mkdirs(t, filepath.Dir(f.projectPath()))
		if err := os.Symlink(target, f.projectPath()); err != nil {
			t.Fatalf("symlinking project config: %v", err)
		}
		reg := f.registry()

		if _, ok := reg.Source("project"); ok {
			t.Error("project source registered from a symlinked config")
		}
		requireDiagnostic(t, reg, "project")
		sourceByID(t, reg, "user")
		sourceByID(t, reg, "mcp")
	})

	t.Run("symlinked nearest config loses to a farther real one", func(t *testing.T) {
		f := newFixture(t)
		f.write(t, f.userPath(), validUserConfig)
		f.write(t, f.mcpPath(), validMCPConfig)
		farther := filepath.Join(f.home, "work", ".omo", "omo.jsonc")
		f.write(t, farther, validProjectConfig)
		target := filepath.Join(f.home, "elsewhere.jsonc")
		f.write(t, target, validProjectConfig)
		f.mkdirs(t, filepath.Dir(f.projectPath()))
		if err := os.Symlink(target, f.projectPath()); err != nil {
			t.Fatalf("symlinking nearest project config: %v", err)
		}
		reg := f.registry()

		project := sourceByID(t, reg, "project")
		if project.Path != farther {
			t.Errorf("project Path = %q, want the farther real config %q", project.Path, farther)
		}
		requireDiagnostic(t, reg, "project")
	})

	t.Run("symlinked .omo directory is skipped", func(t *testing.T) {
		f := newFixture(t)
		f.write(t, f.userPath(), validUserConfig)
		f.write(t, f.mcpPath(), validMCPConfig)
		shared := filepath.Join(f.home, "shared-omo")
		f.write(t, filepath.Join(shared, "omo.jsonc"), validProjectConfig)
		if err := os.Symlink(shared, filepath.Join(f.cwd, ".omo")); err != nil {
			t.Fatalf("symlinking .omo directory: %v", err)
		}
		reg := f.registry()

		if _, ok := reg.Source("project"); ok {
			t.Error("project source registered through a symlinked .omo directory")
		}
		requireDiagnostic(t, reg, "project")
		sourceByID(t, reg, "user")
		sourceByID(t, reg, "mcp")
	})

	t.Run("missing mcp.json yields a diagnostic", func(t *testing.T) {
		f := newFixture(t)
		f.write(t, f.userPath(), validUserConfig)
		f.write(t, f.projectPath(), validProjectConfig)
		reg := f.registry()

		if _, ok := reg.Source("mcp"); ok {
			t.Error("mcp source registered without mcp.json")
		}
		requireDiagnostic(t, reg, "mcp")
		sourceByID(t, reg, "user")
		sourceByID(t, reg, "project")
	})
}

func TestRegistryRebuild(t *testing.T) {
	f := newFixture(t)
	f.write(t, f.userPath(), validUserConfig)

	first := f.registry()
	if _, ok := first.Source("mcp"); ok {
		t.Fatal("mcp source registered before mcp.json existed")
	}
	requireDiagnostic(t, first, "mcp")

	// stale_state: a rebuild after the file appears must reflect it.
	f.write(t, f.mcpPath(), validMCPConfig)
	second := f.registry()
	mcp := sourceByID(t, second, "mcp")
	checkSource(t, mcp, "mcp", f.mcpPath(), workspace.KindExternal, workspace.SchemaMCPServers, true)
	if diagnostics := diagnosticsFor(second, "mcp"); len(diagnostics) != 0 {
		t.Errorf("Diagnostics() = %#v, want no mcp diagnostic after the file appeared", second.Diagnostics())
	}
}

func TestRows(t *testing.T) {
	sections := []editor.Section{
		{ID: editor.SectionModels, Entries: []editor.Entry{{Key: "k3", Path: "/models/k3"}}},
		{ID: "readonly", ReadOnly: true, Entries: []editor.Entry{{Key: "$schema", Path: "/$schema"}}},
		{ID: editor.SectionTelemetry, Entries: []editor.Entry{{Key: "enabled", Path: "/telemetry/enabled", ReadOnly: true}}},
	}
	got := workspace.EditorRows("user", sections)
	want := []workspace.Row{
		{SourceID: "user", Pointer: "/models/k3", Provenance: workspace.ProvenanceEditable},
		{SourceID: "user", Pointer: "/$schema", Provenance: workspace.ProvenanceReadOnly},
		{SourceID: "user", Pointer: "/telemetry/enabled", Provenance: workspace.ProvenanceReadOnly},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("EditorRows() = %#v, want %#v", got, want)
	}

	detail := editor.Detail{Title: "fetch", Lines: []editor.DetailLine{
		{Label: "type", Path: "/mcpServers/fetch/type", Editable: true},
		{Label: "command", Path: "/mcpServers/fetch/command"},
	}}
	got = workspace.DetailRows("mcp", detail)
	want = []workspace.Row{
		{SourceID: "mcp", Pointer: "/mcpServers/fetch/type", Provenance: workspace.ProvenanceEditable},
		{SourceID: "mcp", Pointer: "/mcpServers/fetch/command", Provenance: workspace.ProvenanceReadOnly},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DetailRows() = %#v, want %#v", got, want)
	}

	wantDerived := workspace.Row{SourceID: "providers", Pointer: "/providers/adacode", Provenance: workspace.ProvenanceDerived}
	if row := workspace.DerivedRow("providers", "/providers/adacode"); row != wantDerived {
		t.Errorf("DerivedRow() = %#v, want %#v", row, wantDerived)
	}
}
