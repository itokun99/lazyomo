package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func writeStaleFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("making fixture dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
}

func TestStaleQuery(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, ".omo", "omo.jsonc")
	writeStaleFixture(t, userPath, `{"models":{"k1":{"model":"acme/code-large"}}}`)
	reg := NewRegistry(RegistryConfig{HomeDir: dir, Cwd: dir, AgentDir: filepath.Join(dir, ".omo", "agent")})
	if got := reg.Stale(); len(got) != 0 {
		t.Fatalf("Stale() = %v, want empty on a fresh registry", got)
	}
	tampered := []byte(`{"models":{"k1":{"model":"acme/other"}}}`)
	if err := os.WriteFile(userPath, tampered, 0o644); err != nil {
		t.Fatalf("tampering fixture: %v", err)
	}
	stale := reg.Stale()
	if len(stale) != 1 || stale[0].SourceID != IDUser || stale[0].Path != userPath {
		t.Fatalf("Stale() = %v, want exactly the user source", stale)
	}
	if err := reg.Reload(); err != nil {
		t.Fatalf("reloading workspace: %v", err)
	}
	if got := reg.Stale(); len(got) != 0 {
		t.Fatalf("Stale() after reload = %v, want empty", got)
	}
}

func TestStaleSkipsSessionless(t *testing.T) {
	dir := t.TempDir()
	writeStaleFixture(t, filepath.Join(dir, ".omo", "omo.jsonc"), `{"models":{}}`)
	reg := NewRegistry(RegistryConfig{HomeDir: dir, Cwd: dir, AgentDir: filepath.Join(dir, ".omo", "agent")})
	if got := reg.Stale(); len(got) != 0 {
		t.Fatalf("Stale() = %v, want empty when no session can be stale", got)
	}
}
