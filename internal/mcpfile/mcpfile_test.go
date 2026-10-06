package mcpfile_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/itokun99/lazyomo/internal/mcpfile"
)

const fixturePath = "testdata/sample.json"

// copyFixture copies the synthetic fixture into a temp dir and returns the
// copy's path plus the original bytes.
func copyFixture(t *testing.T) (string, []byte) {
	t.Helper()
	original, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), "mcp.json")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("copying fixture: %v", err)
	}
	return path, original
}

// loadCopy copies the fixture and loads the copy.
func loadCopy(t *testing.T) (*mcpfile.Session, string, []byte) {
	t.Helper()
	path, original := copyFixture(t)
	session, err := mcpfile.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return session, path, original
}

// loadContent loads a session over a temp file holding content.
func loadContent(t *testing.T, content string) (*mcpfile.Session, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mcp.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	session, err := mcpfile.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return session, path
}

func jsonEqual(a, b any) bool {
	ab, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(ab) == string(bb)
}

func mustServer(t *testing.T, session *mcpfile.Session, name string) mcpfile.Server {
	t.Helper()
	server, ok := session.Server(name)
	if !ok {
		t.Fatalf("Server(%q) not found", name)
	}
	return server
}

// backups returns the sorted backup paths of path.
func backups(t *testing.T, path string) []string {
	t.Helper()
	found, err := filepath.Glob(path + ".bak.*")
	if err != nil {
		t.Fatalf("globbing backups: %v", err)
	}
	sort.Strings(found)
	return found
}

// exportArtifact copies path and its backups into MCPFILE_QA_ARTIFACT_DIR
// when that variable is set, so manual-QA shell checks can run against the
// real saved bytes. It is a no-op otherwise.
func exportArtifact(t *testing.T, path, name string) {
	t.Helper()
	dir := os.Getenv("MCPFILE_QA_ARTIFACT_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating QA artifact dir: %v", err)
	}
	copyFile := func(src, dst string) {
		t.Helper()
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("reading QA artifact %s: %v", src, err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			t.Fatalf("writing QA artifact %s: %v", dst, err)
		}
	}
	copyFile(path, filepath.Join(dir, name+".mcp.json"))
	for i, b := range backups(t, path) {
		copyFile(b, filepath.Join(dir, fmt.Sprintf("%s.backup-%d", name, i+1)))
	}
}

func TestLoadRejectsNonStrictJSON(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{name: "line comment", content: "{\n  // comment\n  \"mcpServers\": {}\n}\n"},
		{name: "block comment", content: "{\n  /* comment */\n  \"mcpServers\": {}\n}\n"},
		{name: "trailing comma object", content: "{\n  \"mcpServers\": {},\n}\n"},
		{name: "trailing comma array", content: "{\n  \"mcpServers\": {\"a\": {\"args\": [\"x\",]}}\n}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "mcp.json")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("writing config: %v", err)
			}
			_, err := mcpfile.Load(path)
			if err == nil {
				t.Fatal("Load() error = nil, want strict-JSON rejection")
			}
			if !strings.Contains(err.Error(), "strict JSON") {
				t.Errorf("Load() error = %q, want it to mention strict JSON", err)
			}
		})
	}

	t.Run("valid file loads", func(t *testing.T) {
		path, _ := copyFixture(t)
		if _, err := mcpfile.Load(path); err != nil {
			t.Fatalf("Load() error = %v", err)
		}
	})
}

func TestLoadRejectsMalformedShape(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{name: "root not object", content: "[1, 2, 3]\n"},
		{name: "mcpServers not object", content: "{\"mcpServers\": []}\n"},
		{name: "server not object", content: "{\"mcpServers\": {\"x\": \"y\"}}\n"},
		{name: "settings not object", content: "{\"settings\": 5}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "mcp.json")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("writing config: %v", err)
			}
			_, err := mcpfile.Load(path)
			if err == nil {
				t.Fatal("Load() error = nil, want shape rejection")
			}
			if !strings.Contains(err.Error(), "must be an object") {
				t.Errorf("Load() error = %q, want it to mention an object requirement", err)
			}
		})
	}
}

func TestNoOpRoundTrip(t *testing.T) {
	session, path, original := loadCopy(t)
	if _, err := session.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}
	if !bytes.Equal(saved, original) {
		t.Errorf("saved bytes differ from original:\ngot:\n%s\nwant:\n%s", saved, original)
	}
	if !json.Valid(saved) {
		t.Errorf("saved bytes are not strict JSON:\n%s", saved)
	}
	bs := backups(t, path)
	if len(bs) != 1 {
		t.Fatalf("backup count = %d %v, want 1", len(bs), bs)
	}
	bak, err := os.ReadFile(bs[0])
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if !bytes.Equal(bak, original) {
		t.Errorf("backup differs from original:\ngot:\n%s\nwant:\n%s", bak, original)
	}
}

func TestServerSurface(t *testing.T) {
	session, _, _ := loadCopy(t)

	names := make([]string, 0)
	for _, s := range session.Servers() {
		names = append(names, s.Name)
	}
	if got := strings.Join(names, ","); got != "alpha,beta,delta,gamma" {
		t.Errorf("Servers() names = %s, want alpha,beta,delta,gamma (sorted)", got)
	}

	alpha := mustServer(t, session, "alpha")
	if alpha.Type != "stdio" {
		t.Errorf("alpha.Type = %q, want stdio", alpha.Type)
	}
	if !alpha.Enabled {
		t.Errorf("alpha.Enabled = false, want true")
	}
	if alpha.Preview() != "npx" {
		t.Errorf("alpha.Preview() = %q, want npx", alpha.Preview())
	}
	if !jsonEqual(alpha.Fields["args"], []string{"-y", "@example/alpha"}) {
		t.Errorf("alpha fields args = %#v", alpha.Fields["args"])
	}

	beta := mustServer(t, session, "beta")
	if beta.Type != "http" {
		t.Errorf("beta.Type = %q, want http", beta.Type)
	}
	if beta.Preview() != "https://beta.example.com/mcp" {
		t.Errorf("beta.Preview() = %q", beta.Preview())
	}

	gamma := mustServer(t, session, "gamma")
	if gamma.Type != "stdio" {
		t.Errorf("gamma.Type = %q, want stdio (inferred from command)", gamma.Type)
	}
	if gamma.Enabled {
		t.Errorf("gamma.Enabled = true, want false")
	}
	if gamma.Fields["customNote"] != "unknown field preserved" {
		t.Errorf("gamma unknown field = %#v, want preserved", gamma.Fields["customNote"])
	}

	delta := mustServer(t, session, "delta")
	if delta.Type != "http" {
		t.Errorf("delta.Type = %q, want http (inferred from url)", delta.Type)
	}
	if !delta.Enabled {
		t.Errorf("delta.Enabled = false, want true (absent enabled defaults to true)")
	}
	if delta.Preview() != "https://delta.example.com/mcp" {
		t.Errorf("delta.Preview() = %q", delta.Preview())
	}

	if _, ok := session.Server("missing"); ok {
		t.Errorf("Server(%q) ok = true, want false", "missing")
	}
	if session.Path() == "" {
		t.Errorf("Path() is empty")
	}

	settings := session.Settings()
	if settings == nil {
		t.Fatalf("Settings() = nil, want the settings object")
	}
	if settings["toolPrefix"] != "mcp" {
		t.Errorf("settings toolPrefix = %#v, want mcp", settings["toolPrefix"])
	}
	if !jsonEqual(settings["searchThreshold"], 0.5) {
		t.Errorf("settings searchThreshold = %#v, want 0.5", settings["searchThreshold"])
	}
}

func TestSettingsReadOnlySurviveEdits(t *testing.T) {
	session, path, _ := loadCopy(t)
	before := session.Settings()

	if err := session.AddServer("epsilon", "stdio"); err != nil {
		t.Fatalf("AddServer() error = %v", err)
	}
	if err := session.SetField("epsilon", "command", "uvx"); err != nil {
		t.Fatalf("SetField() error = %v", err)
	}
	if _, err := session.ToggleEnabled("epsilon"); err != nil {
		t.Fatalf("ToggleEnabled() error = %v", err)
	}
	if _, err := session.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}
	if !strings.Contains(string(saved), `"toolPrefix": "mcp"`) {
		t.Errorf("saved bytes lost the settings block:\n%s", saved)
	}
	reloaded, err := mcpfile.Load(path)
	if err != nil {
		t.Fatalf("reload error = %v", err)
	}
	if !jsonEqual(reloaded.Settings(), before) {
		t.Errorf("settings after edits = %#v, want %#v", reloaded.Settings(), before)
	}
}

func TestSetFieldAccepted(t *testing.T) {
	tests := []struct {
		name        string
		server      string
		field       string
		value       any
		want        any
		rawContains string
	}{
		{name: "string field", server: "alpha", field: "command", value: "node", want: "node"},
		{name: "bool field", server: "alpha", field: "enabled", value: false, want: false},
		{name: "type field", server: "gamma", field: "type", value: "http", want: "http"},
		{name: "number field fractional", server: "alpha", field: "idleTimeoutMin", value: 10.5, want: 10.5},
		{name: "number field int", server: "alpha", field: "requestTimeoutMs", value: 2500, want: 2500},
		{name: "string array", server: "alpha", field: "includeTools", value: []string{"read*"}, want: []string{"read*"}},
		{name: "string array any", server: "alpha", field: "excludeTools", value: []any{"write*", "del*"}, want: []string{"write*", "del*"}},
		{name: "string map", server: "alpha", field: "env", value: map[string]string{"TOKEN": "${TOKEN:-def}"}, want: map[string]string{"TOKEN": "${TOKEN:-def}"}, rawContains: "${TOKEN:-def}"},
		{name: "string map any", server: "beta", field: "headers", value: map[string]any{"X-Key": "abc"}, want: map[string]any{"X-Key": "abc"}},
		{name: "bool or array bool", server: "alpha", field: "directTools", value: true, want: true},
		{name: "bool or array list", server: "alpha", field: "directTools", value: []string{"tool_a"}, want: []string{"tool_a"}},
		{name: "auth string", server: "beta", field: "auth", value: "bearer", want: "bearer"},
		{name: "auth false", server: "beta", field: "auth", value: false, want: false},
		{name: "object field", server: "beta", field: "oauth", value: map[string]any{"clientId": "abc"}, want: map[string]any{"clientId": "abc"}},
		{name: "interpolation url", server: "beta", field: "url", value: "https://x.example.com/${PATH:-mcp}", want: "https://x.example.com/${PATH:-mcp}", rawContains: "${PATH:-mcp}"},
		{name: "leading spaces without bang", server: "alpha", field: "command", value: " npx foo", want: " npx foo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, _, _ := loadCopy(t)
			if err := session.SetField(tt.server, tt.field, tt.value); err != nil {
				t.Fatalf("SetField(%q, %q, %#v) error = %v", tt.server, tt.field, tt.value, err)
			}
			got := mustServer(t, session, tt.server).Fields[tt.field]
			if !jsonEqual(got, tt.want) {
				t.Errorf("field %q = %#v, want %#v", tt.field, got, tt.want)
			}
			if tt.rawContains != "" && !strings.Contains(string(session.Bytes()), tt.rawContains) {
				t.Errorf("in-memory bytes lost %q:\n%s", tt.rawContains, session.Bytes())
			}
		})
	}
}

func TestSetFieldRejected(t *testing.T) {
	tests := []struct {
		name    string
		server  string
		field   string
		value   any
		wantMsg string
	}{
		{name: "unknown field", server: "gamma", field: "customNote", value: "x", wantMsg: "unknown field"},
		{name: "bool expected", server: "alpha", field: "enabled", value: "yes", wantMsg: "expected boolean"},
		{name: "string expected", server: "alpha", field: "command", value: 42, wantMsg: "expected string"},
		{name: "array expected", server: "alpha", field: "args", value: "-y", wantMsg: "expected array of strings"},
		{name: "array element type", server: "alpha", field: "args", value: []any{"ok", 7}, wantMsg: "expected array of strings"},
		{name: "map expected", server: "alpha", field: "env", value: []string{"A=B"}, wantMsg: "expected string map"},
		{name: "map value type", server: "alpha", field: "env", value: map[string]any{"A": 1}, wantMsg: "expected string map"},
		{name: "number expected", server: "alpha", field: "idleTimeoutMin", value: "30", wantMsg: "expected number"},
		{name: "object expected", server: "beta", field: "oauth", value: []string{"x"}, wantMsg: "expected object"},
		{name: "directTools type", server: "alpha", field: "directTools", value: "yes", wantMsg: "expected boolean or array of strings"},
		{name: "auth type", server: "beta", field: "auth", value: true, wantMsg: "expected string or false"},
		{name: "type enum", server: "alpha", field: "type", value: "sse", wantMsg: `want "stdio" or "http"`},
		{name: "bang prefix", server: "alpha", field: "command", value: "!rm -rf /", wantMsg: `starting with "!"`},
		{name: "bang after leading spaces", server: "alpha", field: "command", value: "  !cmd", wantMsg: `starting with "!"`},
		{name: "bang after leading tab", server: "alpha", field: "command", value: "\t!cmd", wantMsg: `starting with "!"`},
		{name: "dollar paren after leading spaces", server: "alpha", field: "command", value: "  echo $(id)", wantMsg: `containing "$("`},
		{name: "dollar paren", server: "alpha", field: "command", value: "echo $(id)", wantMsg: `containing "$("`},
		{name: "bang in array", server: "alpha", field: "args", value: []string{"safe", "!boom"}, wantMsg: `starting with "!"`},
		{name: "dollar paren in map", server: "alpha", field: "env", value: map[string]string{"X": "a$(b)"}, wantMsg: `containing "$("`},
		{name: "bang in url", server: "beta", field: "url", value: "!http://x", wantMsg: `starting with "!"`},
		{name: "enabled http needs url", server: "beta", field: "url", value: "", wantMsg: "mcpServers.beta.url: Required for enabled http server"},
		{name: "enabled http whitespace url", server: "beta", field: "url", value: "   ", wantMsg: "mcpServers.beta.url: Required for enabled http server"},
		{name: "clearing url falls back to stdio", server: "delta", field: "url", value: "", wantMsg: "mcpServers.delta.command: Required for enabled stdio server"},
		{name: "enabled stdio needs command", server: "alpha", field: "command", value: "", wantMsg: "mcpServers.alpha.command: Required for enabled stdio server"},
		{name: "server not found", server: "nope", field: "command", value: "x", wantMsg: `mcpServers.nope: server not found`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, _, _ := loadCopy(t)
			before := append([]byte(nil), session.Bytes()...)
			err := session.SetField(tt.server, tt.field, tt.value)
			if err == nil {
				t.Fatalf("SetField(%q, %q, %#v) error = nil, want rejection", tt.server, tt.field, tt.value)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("SetField() error = %q, want it to contain %q", err, tt.wantMsg)
			}
			if after := session.Bytes(); !bytes.Equal(before, after) {
				t.Errorf("rejected SetField mutated the document:\ngot:\n%s\nwant:\n%s", after, before)
			}
		})
	}
}

func TestToggleEnabled(t *testing.T) {
	t.Run("flips exactly one field", func(t *testing.T) {
		content := "{\n  \"mcpServers\": {\n    \"solo\": {\n      \"type\": \"http\",\n      \"url\": \"https://solo.example.com/mcp\",\n      \"enabled\": false\n    }\n  }\n}\n"
		session, _ := loadContent(t, content)
		before := append([]byte(nil), session.Bytes()...)

		enabled, err := session.ToggleEnabled("solo")
		if err != nil {
			t.Fatalf("ToggleEnabled() error = %v", err)
		}
		if !enabled {
			t.Errorf("ToggleEnabled() = false, want true")
		}
		want := bytes.Replace(before, []byte(`"enabled": false`), []byte(`"enabled": true`), 1)
		if got := session.Bytes(); !bytes.Equal(got, want) {
			t.Errorf("toggle changed more than one field:\ngot:\n%s\nwant:\n%s", got, want)
		}

		enabled, err = session.ToggleEnabled("solo")
		if err != nil {
			t.Fatalf("ToggleEnabled() error = %v", err)
		}
		if enabled {
			t.Errorf("ToggleEnabled() = true, want false")
		}
		if got := session.Bytes(); !bytes.Equal(got, before) {
			t.Errorf("double toggle did not restore bytes:\ngot:\n%s\nwant:\n%s", got, before)
		}
	})

	t.Run("absent enabled defaults true and is written", func(t *testing.T) {
		session, _ := loadContent(t, "{\"mcpServers\": {\"ghost\": {\"type\": \"stdio\", \"command\": \"npx\"}}}\n")
		enabled, err := session.ToggleEnabled("ghost")
		if err != nil {
			t.Fatalf("ToggleEnabled() error = %v", err)
		}
		if enabled {
			t.Errorf("ToggleEnabled() = true, want false")
		}
		if !strings.Contains(string(session.Bytes()), `"enabled": false`) {
			t.Errorf("toggle did not write enabled:\n%s", session.Bytes())
		}
	})

	t.Run("rejects enabling stdio without command", func(t *testing.T) {
		session, _ := loadContent(t, "{\"mcpServers\": {\"broken\": {\"type\": \"stdio\", \"enabled\": false}}}\n")
		before := append([]byte(nil), session.Bytes()...)
		_, err := session.ToggleEnabled("broken")
		if err == nil {
			t.Fatal("ToggleEnabled() error = nil, want rejection")
		}
		if want := "mcpServers.broken.command: Required for enabled stdio server"; !strings.Contains(err.Error(), want) {
			t.Errorf("ToggleEnabled() error = %q, want it to contain %q", err, want)
		}
		if got := session.Bytes(); !bytes.Equal(got, before) {
			t.Errorf("rejected toggle mutated the document")
		}
	})

	t.Run("rejects enabling http without url", func(t *testing.T) {
		session, _ := loadContent(t, "{\"mcpServers\": {\"broken\": {\"type\": \"http\", \"enabled\": false}}}\n")
		_, err := session.ToggleEnabled("broken")
		if err == nil {
			t.Fatal("ToggleEnabled() error = nil, want rejection")
		}
		if want := "mcpServers.broken.url: Required for enabled http server"; !strings.Contains(err.Error(), want) {
			t.Errorf("ToggleEnabled() error = %q, want it to contain %q", err, want)
		}
	})

	t.Run("missing server", func(t *testing.T) {
		session, _, _ := loadCopy(t)
		_, err := session.ToggleEnabled("nope")
		if err == nil {
			t.Fatal("ToggleEnabled() error = nil, want rejection")
		}
		if want := `mcpServers.nope: server not found`; !strings.Contains(err.Error(), want) {
			t.Errorf("ToggleEnabled() error = %q, want it to contain %q", err, want)
		}
	})
}

func TestAddServer(t *testing.T) {
	session, path, _ := loadCopy(t)

	if err := session.AddServer("epsilon", "http"); err != nil {
		t.Fatalf("AddServer() error = %v", err)
	}
	eps := mustServer(t, session, "epsilon")
	if eps.Type != "http" {
		t.Errorf("epsilon.Type = %q, want http", eps.Type)
	}
	if eps.Enabled {
		t.Errorf("epsilon.Enabled = true, want false after AddServer")
	}

	if _, err := session.ToggleEnabled("epsilon"); err == nil {
		t.Fatal("ToggleEnabled() on a fresh http server error = nil, want url requirement")
	} else if want := "mcpServers.epsilon.url: Required for enabled http server"; !strings.Contains(err.Error(), want) {
		t.Errorf("ToggleEnabled() error = %q, want %q", err, want)
	}

	if err := session.SetField("epsilon", "url", "https://epsilon.example.com/mcp"); err != nil {
		t.Fatalf("SetField() error = %v", err)
	}
	if enabled, err := session.ToggleEnabled("epsilon"); err != nil || !enabled {
		t.Fatalf("ToggleEnabled() = %v, %v; want true, nil", enabled, err)
	}

	if err := session.AddServer("epsilon", "stdio"); err == nil {
		t.Fatal("duplicate AddServer() error = nil, want rejection")
	} else if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("duplicate AddServer() error = %q", err)
	}
	if err := session.AddServer("   ", "stdio"); err == nil {
		t.Fatal("blank AddServer() error = nil, want rejection")
	} else if !strings.Contains(err.Error(), "must not be empty") {
		t.Errorf("blank AddServer() error = %q", err)
	}
	if err := session.AddServer("bad", "ws"); err == nil {
		t.Fatal("bad type AddServer() error = nil, want rejection")
	} else if !strings.Contains(err.Error(), `want "stdio" or "http"`) {
		t.Errorf("bad type AddServer() error = %q", err)
	}

	// Server names are arbitrary strings; JSON pointers must be escaped.
	if err := session.AddServer("x/y~z", "stdio"); err != nil {
		t.Fatalf("AddServer(escaped name) error = %v", err)
	}
	if err := session.SetField("x/y~z", "command", "uvx"); err != nil {
		t.Fatalf("SetField(escaped name) error = %v", err)
	}
	if enabled, err := session.ToggleEnabled("x/y~z"); err != nil || !enabled {
		t.Fatalf("ToggleEnabled(escaped name) = %v, %v; want true, nil", enabled, err)
	}
	if _, err := session.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}
	if !json.Valid(saved) {
		t.Errorf("saved bytes are not strict JSON:\n%s", saved)
	}
	if !strings.Contains(string(saved), `"x/y~z"`) {
		t.Errorf("saved bytes lost the escaped name:\n%s", saved)
	}

	reloaded, err := mcpfile.Load(path)
	if err != nil {
		t.Fatalf("reload error = %v", err)
	}
	for _, name := range []string{"epsilon", "x/y~z"} {
		server := mustServer(t, reloaded, name)
		if !server.Enabled {
			t.Errorf("%s.Enabled = false after save/reload, want true", name)
		}
	}
	if err := session.RemoveServer("x/y~z"); err != nil {
		t.Fatalf("RemoveServer() error = %v", err)
	}
	if _, ok := session.Server("x/y~z"); ok {
		t.Errorf("Server(x/y~z) still present after RemoveServer")
	}
}

func TestAddServerCreatesMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	session, err := mcpfile.Load(path)
	if err != nil {
		t.Fatalf("Load(missing) error = %v", err)
	}
	if session.Path() != path {
		t.Errorf("Path() = %q, want %q", session.Path(), path)
	}
	if got := session.Servers(); len(got) != 0 {
		t.Errorf("Servers() = %v, want empty", got)
	}
	if session.Settings() != nil {
		t.Errorf("Settings() = %v, want nil", session.Settings())
	}

	// A save before any mutation must not create the file.
	if _, err := session.Save(); err != nil {
		t.Fatalf("Save() on empty session error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("Save() on empty session created the file (stat err = %v)", err)
	}

	if err := session.AddServer("first", "stdio"); err != nil {
		t.Fatalf("AddServer() error = %v", err)
	}
	if err := session.SetField("first", "command", "npx"); err != nil {
		t.Fatalf("SetField() error = %v", err)
	}
	if enabled, err := session.ToggleEnabled("first"); err != nil || !enabled {
		t.Fatalf("ToggleEnabled() = %v, %v; want true, nil", enabled, err)
	}
	if _, err := session.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}
	if !json.Valid(saved) {
		t.Errorf("saved bytes are not strict JSON:\n%s", saved)
	}
	reloaded, err := mcpfile.Load(path)
	if err != nil {
		t.Fatalf("reload error = %v", err)
	}
	first := mustServer(t, reloaded, "first")
	if first.Type != "stdio" || !first.Enabled || first.Preview() != "npx" {
		t.Errorf("reloaded first = %+v, want enabled stdio with command npx", first)
	}

	if bs := backups(t, path); len(bs) != 0 {
		t.Errorf("backups for a freshly created file = %v, want none", bs)
	}

	exportArtifact(t, path, "add-server-create")
}

func TestAddServerAdoptsExternallyCreatedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	session, err := mcpfile.Load(path)
	if err != nil {
		t.Fatalf("Load(missing) error = %v", err)
	}

	external := []byte("{\n  \"mcpServers\": {\n    \"external\": {\n      \"type\": \"stdio\",\n      \"command\": \"uvx\"\n    }\n  }\n}\n")
	if err := os.WriteFile(path, external, 0o644); err != nil {
		t.Fatalf("external write: %v", err)
	}

	if err := session.AddServer("late", "stdio"); err != nil {
		t.Fatalf("AddServer() error = %v", err)
	}
	if _, ok := session.Server("external"); !ok {
		t.Errorf("AddServer clobbered the externally created file")
	}
	if _, err := session.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	reloaded, err := mcpfile.Load(path)
	if err != nil {
		t.Fatalf("reload error = %v", err)
	}
	if _, ok := reloaded.Server("external"); !ok {
		t.Errorf("external server lost after save")
	}
	if _, ok := reloaded.Server("late"); !ok {
		t.Errorf("added server missing after save")
	}
	bs := backups(t, path)
	if len(bs) != 1 {
		t.Fatalf("backup count = %d %v, want 1 (external content preserved)", len(bs), bs)
	}
	bak, err := os.ReadFile(bs[0])
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if !bytes.Equal(bak, external) {
		t.Errorf("backup does not hold the external bytes")
	}
}

func TestRemoveServer(t *testing.T) {
	session, path, _ := loadCopy(t)
	if err := session.RemoveServer("gamma"); err != nil {
		t.Fatalf("RemoveServer() error = %v", err)
	}
	if _, ok := session.Server("gamma"); ok {
		t.Errorf("gamma still present after RemoveServer")
	}
	if got := len(session.Servers()); got != 3 {
		t.Errorf("Servers() count = %d, want 3", got)
	}
	if _, err := session.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	reloaded, err := mcpfile.Load(path)
	if err != nil {
		t.Fatalf("reload error = %v", err)
	}
	if _, ok := reloaded.Server("gamma"); ok {
		t.Errorf("gamma still present after save/reload")
	}
	for _, name := range []string{"alpha", "beta", "delta"} {
		mustServer(t, reloaded, name)
	}
	if err := session.RemoveServer("gamma"); err == nil {
		t.Fatal("RemoveServer(missing) error = nil, want rejection")
	} else if want := `mcpServers.gamma: server not found`; !strings.Contains(err.Error(), want) {
		t.Errorf("RemoveServer(missing) error = %q, want %q", err, want)
	}
}

func TestRenameServer(t *testing.T) {
	session, path, _ := loadCopy(t)
	before := mustServer(t, session, "gamma")

	result, err := session.RenameServer("gamma", "gamma-two")
	if err != nil {
		t.Fatalf("RenameServer() error = %v", err)
	}
	if !result.OrphanAuth {
		t.Errorf("RenameServer() OrphanAuth = false, want true")
	}
	if result.Warning == "" {
		t.Errorf("RenameServer() Warning is empty, want orphan-auth warning metadata")
	}
	if result.OldName != "gamma" || result.NewName != "gamma-two" {
		t.Errorf("RenameServer() = %+v, want names gamma -> gamma-two", result)
	}
	after := mustServer(t, session, "gamma-two")
	if !jsonEqual(after.Fields, before.Fields) {
		t.Errorf("renamed fields = %#v, want %#v", after.Fields, before.Fields)
	}
	if after.Enabled != before.Enabled || after.Type != before.Type {
		t.Errorf("renamed server changed enabled/type: %+v vs %+v", after, before)
	}
	if _, ok := session.Server("gamma"); ok {
		t.Errorf("old name still present after rename")
	}

	if _, err := session.RenameServer("gamma-two", "alpha"); err == nil {
		t.Fatal("RenameServer(duplicate) error = nil, want rejection")
	} else if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("RenameServer(duplicate) error = %q", err)
	}
	if _, err := session.RenameServer("gamma-two", "   "); err == nil {
		t.Fatal("RenameServer(blank) error = nil, want rejection")
	} else if !strings.Contains(err.Error(), "must not be empty") {
		t.Errorf("RenameServer(blank) error = %q", err)
	}
	if _, err := session.RenameServer("nope", "x"); err == nil {
		t.Fatal("RenameServer(missing) error = nil, want rejection")
	} else if want := `mcpServers.nope: server not found`; !strings.Contains(err.Error(), want) {
		t.Errorf("RenameServer(missing) error = %q, want %q", err, want)
	}

	if _, err := session.RenameServer("gamma-two", "a/b~c"); err != nil {
		t.Fatalf("RenameServer(escaped) error = %v", err)
	}
	if _, err := session.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	reloaded, err := mcpfile.Load(path)
	if err != nil {
		t.Fatalf("reload error = %v", err)
	}
	renamed := mustServer(t, reloaded, "a/b~c")
	if renamed.Fields["customNote"] != "unknown field preserved" {
		t.Errorf("renamed server lost the unknown field: %#v", renamed.Fields)
	}
	if renamed.Enabled {
		t.Errorf("renamed server Enabled = true, want false")
	}
}

func TestStaleStateExternalWrite(t *testing.T) {
	session, path, _ := loadCopy(t)

	external := []byte("{\n  \"mcpServers\": {\n    \"external\": {\n      \"type\": \"stdio\",\n      \"command\": \"uvx\"\n    }\n  }\n}\n")
	if err := os.WriteFile(path, external, 0o644); err != nil {
		t.Fatalf("external write: %v", err)
	}

	if _, err := session.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}
	if !bytes.Equal(saved, session.Bytes()) {
		t.Errorf("saved bytes differ from session bytes:\ngot:\n%s\nwant:\n%s", saved, session.Bytes())
	}
	if strings.Contains(string(saved), `"external"`) {
		t.Errorf("target still holds external content:\n%s", saved)
	}

	// The external content is not silently lost: omodit's backup holds it.
	bs := backups(t, path)
	if len(bs) != 1 {
		t.Fatalf("backup count = %d %v, want 1", len(bs), bs)
	}
	bak, err := os.ReadFile(bs[0])
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if !bytes.Equal(bak, external) {
		t.Errorf("backup does not hold the external bytes:\ngot:\n%s\nwant:\n%s", bak, external)
	}
}

func TestStrictOutputBytes(t *testing.T) {
	session, path, original := loadCopy(t)
	if err := session.AddServer("qa-bytes", "stdio"); err != nil {
		t.Fatalf("AddServer() error = %v", err)
	}
	if err := session.SetField("qa-bytes", "command", "uvx"); err != nil {
		t.Fatalf("SetField() error = %v", err)
	}
	if _, err := session.ToggleEnabled("qa-bytes"); err != nil {
		t.Fatalf("ToggleEnabled() error = %v", err)
	}
	if _, err := session.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}
	if bytes.Equal(saved, original) {
		t.Fatalf("saved bytes unchanged after edits")
	}
	if !json.Valid(saved) {
		t.Errorf("saved bytes are not strict JSON:\n%s", saved)
	}
	if re := regexp.MustCompile(`,\s*[}\]]`); re.Match(saved) {
		t.Errorf("saved bytes contain a trailing comma: %q", re.Find(saved))
	}
	if got, want := strings.Count(string(saved), "//"), strings.Count(string(saved), "https://"); got != want {
		t.Errorf("saved bytes contain %d '//' occurrences, want %d (only https:// urls)", got, want)
	}
	if !strings.Contains(string(saved), "${ALPHA_TOKEN:-fallback}") {
		t.Errorf("saved bytes expanded or dropped ${ALPHA_TOKEN:-fallback}:\n%s", saved)
	}
}

func TestAddEditToggle(t *testing.T) {
	session, path, original := loadCopy(t)

	if err := session.AddServer("qa-mcp", "stdio"); err != nil {
		t.Fatalf("AddServer() error = %v", err)
	}
	if err := session.SetField("qa-mcp", "command", "npx"); err != nil {
		t.Fatalf("SetField(command) error = %v", err)
	}
	if err := session.SetField("qa-mcp", "args", []string{"-y", "qa-mcp"}); err != nil {
		t.Fatalf("SetField(args) error = %v", err)
	}
	if err := session.SetField("qa-mcp", "env", map[string]string{"QA_TOKEN": "${QA_TOKEN:-none}"}); err != nil {
		t.Fatalf("SetField(env) error = %v", err)
	}
	enabled, err := session.ToggleEnabled("qa-mcp")
	if err != nil {
		t.Fatalf("ToggleEnabled() error = %v", err)
	}
	if !enabled {
		t.Errorf("ToggleEnabled() = false, want true")
	}

	server := mustServer(t, session, "qa-mcp")
	if server.Type != "stdio" || !server.Enabled || server.Preview() != "npx" {
		t.Errorf("qa-mcp = %+v, want enabled stdio with command npx", server)
	}
	if !jsonEqual(server.Fields["args"], []string{"-y", "qa-mcp"}) {
		t.Errorf("qa-mcp args = %#v", server.Fields["args"])
	}
	if !jsonEqual(server.Fields["env"], map[string]string{"QA_TOKEN": "${QA_TOKEN:-none}"}) {
		t.Errorf("qa-mcp env = %#v", server.Fields["env"])
	}

	if _, err := session.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	bs := backups(t, path)
	for _, b := range bs {
		t.Logf("backup: %s", filepath.Base(b))
	}
	if len(bs) != 1 {
		t.Fatalf("backup count = %d %v, want 1", len(bs), bs)
	}
	bak, err := os.ReadFile(bs[0])
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if !bytes.Equal(bak, original) {
		t.Errorf("backup differs from the pre-save fixture")
	}

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}
	if !json.Valid(saved) {
		t.Errorf("saved bytes are not strict JSON:\n%s", saved)
	}
	if !strings.Contains(string(saved), "${QA_TOKEN:-none}") {
		t.Errorf("interpolation literal not preserved verbatim:\n%s", saved)
	}

	reloaded, err := mcpfile.Load(path)
	if err != nil {
		t.Fatalf("reload error = %v", err)
	}
	reloadedServer := mustServer(t, reloaded, "qa-mcp")
	if !reloadedServer.Enabled || reloadedServer.Preview() != "npx" {
		t.Errorf("reloaded qa-mcp = %+v", reloadedServer)
	}
	for _, name := range []string{"alpha", "beta", "gamma", "delta"} {
		mustServer(t, reloaded, name)
	}

	exportArtifact(t, path, "add-edit-toggle")
}

func TestFieldsSurface(t *testing.T) {
	specs := mcpfile.Fields()
	if len(specs) != 21 {
		t.Fatalf("Fields() = %d specs, want 21", len(specs))
	}
	byName := make(map[string]mcpfile.FieldSpec, len(specs))
	for _, spec := range specs {
		byName[spec.Name] = spec
	}
	tests := []struct {
		name        string
		kind        mcpfile.FieldKind
		suggestions []string
	}{
		{name: "type", kind: mcpfile.FieldString, suggestions: []string{"stdio", "http"}},
		{name: "command", kind: mcpfile.FieldString},
		{name: "args", kind: mcpfile.FieldStrings},
		{name: "env", kind: mcpfile.FieldStringMap},
		{name: "headers", kind: mcpfile.FieldStringMap},
		{name: "auth", kind: mcpfile.FieldStringOrFalse, suggestions: []string{"bearer", "oauth"}},
		{name: "oauth", kind: mcpfile.FieldObject},
		{name: "enabled", kind: mcpfile.FieldBool},
		{name: "lifecycle", kind: mcpfile.FieldString, suggestions: []string{"lazy", "eager", "keep-alive"}},
		{name: "idleTimeoutMin", kind: mcpfile.FieldNumber},
		{name: "exposure", kind: mcpfile.FieldString, suggestions: []string{"auto", "direct", "search", "proxy"}},
		{name: "directTools", kind: mcpfile.FieldBoolOrStrings},
		{name: "logLevel", kind: mcpfile.FieldString, suggestions: []string{"debug", "info", "notice", "warning", "error", "critical", "alert", "emergency"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, ok := byName[tt.name]
			if !ok {
				t.Fatalf("Fields() missing %q", tt.name)
			}
			if spec.Kind != tt.kind {
				t.Errorf("%s kind = %v, want %v", tt.name, spec.Kind, tt.kind)
			}
			if tt.suggestions != nil && !jsonEqual(spec.Suggestions, tt.suggestions) {
				t.Errorf("%s suggestions = %#v, want %#v", tt.name, spec.Suggestions, tt.suggestions)
			}
		})
	}
}

func TestResolveAgentDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OMO_CODING_AGENT_DIR", "/omo/agent")
	t.Setenv("SENPI_CODING_AGENT_DIR", "/senpi/agent")
	t.Setenv("PI_CODING_AGENT_DIR", "/pi/agent")

	got, err := mcpfile.ResolveAgentDir()
	if err != nil {
		t.Fatalf("ResolveAgentDir() error = %v", err)
	}
	if want := "/omo/agent"; got != want {
		t.Errorf("ResolveAgentDir() = %q, want %q", got, want)
	}

	t.Setenv("OMO_CODING_AGENT_DIR", "")
	got, err = mcpfile.ResolveAgentDir()
	if err != nil {
		t.Fatalf("ResolveAgentDir() error = %v", err)
	}
	if want := "/senpi/agent"; got != want {
		t.Errorf("ResolveAgentDir() = %q, want %q", got, want)
	}

	t.Setenv("SENPI_CODING_AGENT_DIR", "")
	got, err = mcpfile.ResolveAgentDir()
	if err != nil {
		t.Fatalf("ResolveAgentDir() error = %v", err)
	}
	if want := "/pi/agent"; got != want {
		t.Errorf("ResolveAgentDir() = %q, want %q", got, want)
	}

	t.Setenv("PI_CODING_AGENT_DIR", "")
	got, err = mcpfile.ResolveAgentDir()
	if err != nil {
		t.Fatalf("ResolveAgentDir() error = %v", err)
	}
	if want := filepath.Join(home, ".omo", "agent"); got != want {
		t.Errorf("ResolveAgentDir() = %q, want %q", got, want)
	}
}

// TestSessionSeam covers the workspace seam: dirty tracking, backup
// reporting on Save, and Reload discarding unsaved edits.
func TestSessionSeam(t *testing.T) {
	session, path, _ := loadCopy(t)

	// A freshly loaded session is clean.
	if got := session.DirtyPaths(); len(got) != 0 {
		t.Fatalf("DirtyPaths() after load = %v, want none", got)
	}

	// Edits accumulate deduplicated, sorted dirty pointers.
	if err := session.SetField("alpha", "command", "npx"); err != nil {
		t.Fatalf("SetField() error = %v", err)
	}
	if enabled, err := session.ToggleEnabled("gamma"); err != nil || !enabled {
		t.Fatalf("ToggleEnabled() = %v, %v; want true, nil", enabled, err)
	}
	got := session.DirtyPaths()
	if len(got) != 2 || got[0] != "/mcpServers/alpha/command" || got[1] != "/mcpServers/gamma/enabled" {
		t.Fatalf("DirtyPaths() = %v, want the two edited pointers sorted", got)
	}

	// Save reports the timestamped backup it created and clears the dirty set.
	before := len(backups(t, path))
	backup, err := session.Save()
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	after := backups(t, path)
	if len(after) != before+1 {
		t.Fatalf("backup count = %d, want %d", len(after), before+1)
	}
	if backup != after[len(after)-1] {
		t.Errorf("Save() backup = %q, want %q", backup, after[len(after)-1])
	}
	if got := session.DirtyPaths(); len(got) != 0 {
		t.Fatalf("DirtyPaths() after Save = %v, want none", got)
	}

	// Reload discards unsaved edits and restores the file's state.
	if err := session.SetField("alpha", "command", "uvx"); err != nil {
		t.Fatalf("SetField() error = %v", err)
	}
	if err := session.Reload(); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	if got := session.DirtyPaths(); len(got) != 0 {
		t.Fatalf("DirtyPaths() after Reload = %v, want none", got)
	}
	if server := mustServer(t, session, "alpha"); server.Preview() != "npx" {
		t.Errorf("alpha command after Reload = %q, want npx", server.Preview())
	}

	// The first save that creates a file reports no backup.
	missing := filepath.Join(t.TempDir(), "mcp.json")
	fresh, err := mcpfile.Load(missing)
	if err != nil {
		t.Fatalf("Load(missing) error = %v", err)
	}
	if err := fresh.AddServer("first", "stdio"); err != nil {
		t.Fatalf("AddServer() error = %v", err)
	}
	if err := fresh.SetField("first", "command", "npx"); err != nil {
		t.Fatalf("SetField() error = %v", err)
	}
	backup, err = fresh.Save()
	if err != nil {
		t.Fatalf("first Save() error = %v", err)
	}
	if backup != "" {
		t.Errorf("first-save backup = %q, want empty", backup)
	}
	if _, err := os.Stat(missing); err != nil {
		t.Fatalf("first save did not create the file: %v", err)
	}
}
