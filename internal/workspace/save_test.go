package workspace_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/itokun99/lazyomo/internal/editor"
	"github.com/itokun99/lazyomo/internal/workspace"
)

// fakeSession is a minimal workspace.Session double that really writes bytes:
// Save copies the current file to a sidecar backup and writes the pending
// content, clearing dirty only on success (mirroring editor semantics).
// Reload re-reads the file from disk and drops pending edits.
type fakeSession struct {
	path     string
	dirty    []string
	pending  []byte
	saveErr  error
	saved    int
	reloaded int
}

func newFakeSession(path, pending string) *fakeSession {
	return &fakeSession{path: path, dirty: []string{"/changed"}, pending: []byte(pending)}
}

func (f *fakeSession) Path() string         { return f.path }
func (f *fakeSession) DirtyPaths() []string { return append([]string(nil), f.dirty...) }

func (f *fakeSession) Save() (string, error) {
	if f.saveErr != nil {
		return "", f.saveErr
	}
	backup := ""
	if orig, err := os.ReadFile(f.path); err == nil {
		backup = f.path + ".bak.fake"
		if err := os.WriteFile(backup, orig, 0o644); err != nil {
			return "", err
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.WriteFile(f.path, f.pending, 0o644); err != nil {
		return "", err
	}
	f.dirty = nil
	f.saved++
	return backup, nil
}

func (f *fakeSession) Reload() error {
	data, err := os.ReadFile(f.path)
	if err != nil {
		return err
	}
	f.pending = data
	f.dirty = nil
	f.reloaded++
	return nil
}

func (f *fakeSession) edit(content string) {
	f.pending = []byte(content)
	f.dirty = []string{"/changed"}
}

// twoFakeRegistry returns a registry with the user and mcp layers present,
// both sessions backed by fakes holding pending edits.
func twoFakeRegistry(t *testing.T) (*fixture, *workspace.Registry, *fakeSession, *fakeSession) {
	t.Helper()
	f := newFixture(t)
	f.write(t, f.userPath(), validUserConfig)
	f.write(t, f.mcpPath(), validMCPConfig)
	reg := f.registry()
	user := newFakeSession(f.userPath(), `{"models": {"k3": {"model": "acme/other"}}}`)
	mcp := newFakeSession(f.mcpPath(), `{"mcpServers": {"fetch": {"type": "stdio", "command": "npx", "enabled": false}}}`)
	if err := reg.AttachSession("user", user); err != nil {
		t.Fatalf("AttachSession(user): %v", err)
	}
	if err := reg.AttachSession("mcp", mcp); err != nil {
		t.Fatalf("AttachSession(mcp): %v", err)
	}
	return f, reg, user, mcp
}

func resultByID(t *testing.T, res workspace.SaveResult, id string) workspace.FileResult {
	t.Helper()
	for _, fr := range res.Files {
		if fr.SourceID == id {
			return fr
		}
	}
	t.Fatalf("no FileResult for %q in %#v", id, res.Files)
	return workspace.FileResult{}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestSaveAll(t *testing.T) {
	t.Run("two dirty fakes both saved with two backups", func(t *testing.T) {
		f, reg, user, mcp := twoFakeRegistry(t)

		res := reg.SaveAll()

		if len(res.Files) != 2 {
			t.Fatalf("SaveAll Files = %#v, want 2 entries", res.Files)
		}
		if res.Files[0].SourceID != "user" || res.Files[1].SourceID != "mcp" {
			t.Errorf("SaveAll order = %q,%q, want user,mcp (registry order)",
				res.Files[0].SourceID, res.Files[1].SourceID)
		}
		for _, id := range []string{"user", "mcp"} {
			fr := resultByID(t, res, id)
			if fr.Err != nil {
				t.Errorf("%s Err = %v, want nil", id, fr.Err)
			}
			if fr.Stale {
				t.Errorf("%s Stale = true, want false", id)
			}
			if fr.Backup == "" {
				t.Fatalf("%s Backup is empty, want a backup path", id)
			}
			backup, err := os.ReadFile(fr.Backup)
			if err != nil {
				t.Errorf("read %s backup: %v", id, err)
			}
			_ = backup
		}
		// Backups hold the pre-save bytes; files hold the pending bytes.
		if got := readFile(t, resultByID(t, res, "user").Backup); got != validUserConfig {
			t.Errorf("user backup = %q, want original %q", got, validUserConfig)
		}
		if got := readFile(t, resultByID(t, res, "mcp").Backup); got != validMCPConfig {
			t.Errorf("mcp backup = %q, want original %q", got, validMCPConfig)
		}
		if got := readFile(t, f.userPath()); !strings.Contains(got, "acme/other") {
			t.Errorf("user file = %q, want pending content", got)
		}
		if got := readFile(t, f.mcpPath()); !strings.Contains(got, `"enabled": false`) {
			t.Errorf("mcp file = %q, want pending content", got)
		}
		if dirty := user.DirtyPaths(); len(dirty) != 0 {
			t.Errorf("user dirty after save = %v, want cleared", dirty)
		}
		if dirty := mcp.DirtyPaths(); len(dirty) != 0 {
			t.Errorf("mcp dirty after save = %v, want cleared", dirty)
		}
		if user.saved != 1 || mcp.saved != 1 {
			t.Errorf("save counts = %d,%d, want 1,1", user.saved, mcp.saved)
		}
		t.Logf("saved 2 files, backups: %s %s", res.Files[0].Backup, res.Files[1].Backup)
	})

	t.Run("stale file refused while the other still saves", func(t *testing.T) {
		f, reg, user, mcp := twoFakeRegistry(t)
		tampered := `{"mcpServers": {"fetch": {"type": "stdio", "command": "tampered"}}}`
		if err := os.WriteFile(f.mcpPath(), []byte(tampered), 0o644); err != nil {
			t.Fatalf("tampering mcp file: %v", err)
		}

		res := reg.SaveAll()

		userRes := resultByID(t, res, "user")
		if userRes.Err != nil || userRes.Stale {
			t.Errorf("user result = %+v, want a clean save", userRes)
		}
		if user.saved != 1 {
			t.Errorf("user saved = %d, want 1 despite the stale sibling", user.saved)
		}
		stale := resultByID(t, res, "mcp")
		if !stale.Stale {
			t.Errorf("mcp Stale = false, want true after on-disk tamper")
		}
		if stale.Err == nil || !strings.Contains(stale.Err.Error(), "stale") {
			t.Errorf("mcp Err = %v, want a stale error", stale.Err)
		}
		if got := readFile(t, f.mcpPath()); got != tampered {
			t.Errorf("mcp file = %q, want tampered bytes untouched", got)
		}
		if dirty := mcp.DirtyPaths(); len(dirty) == 0 {
			t.Error("mcp dirty cleared on stale refusal, want preserved")
		}
		if mcp.saved != 0 {
			t.Errorf("mcp saved = %d, want 0", mcp.saved)
		}
		if _, err := os.Stat(f.mcpPath() + ".bak.fake"); !os.IsNotExist(err) {
			t.Error("stale file got a backup, want none (bytes untouched)")
		}
		t.Logf("stale refused: %v; user still saved", stale.Err)
	})

	t.Run("save error continues and keeps dirty", func(t *testing.T) {
		f, reg, user, mcp := twoFakeRegistry(t)
		mcp.saveErr = errors.New("boom: disk full")

		res := reg.SaveAll()

		failed := resultByID(t, res, "mcp")
		if failed.Err == nil || !strings.Contains(failed.Err.Error(), "boom") {
			t.Errorf("mcp Err = %v, want the save error", failed.Err)
		}
		if failed.Stale {
			t.Errorf("mcp Stale = true, want false for a plain save error")
		}
		if dirty := mcp.DirtyPaths(); len(dirty) == 0 {
			t.Error("mcp dirty cleared on save failure, want preserved")
		}
		if user.saved != 1 {
			t.Errorf("user saved = %d, want 1 (continue-on-error)", user.saved)
		}
		if got := readFile(t, f.mcpPath()); got != validMCPConfig {
			t.Errorf("mcp file = %q, want original bytes untouched", got)
		}
	})

	t.Run("clean files are never written", func(t *testing.T) {
		f := newFixture(t)
		f.write(t, f.userPath(), validUserConfig)
		f.write(t, f.mcpPath(), validMCPConfig)
		reg := f.registry()
		user := newFakeSession(f.userPath(), validUserConfig)
		user.dirty = nil
		mcp := newFakeSession(f.mcpPath(), validMCPConfig)
		mcp.dirty = nil
		if err := reg.AttachSession("user", user); err != nil {
			t.Fatalf("AttachSession(user): %v", err)
		}
		if err := reg.AttachSession("mcp", mcp); err != nil {
			t.Fatalf("AttachSession(mcp): %v", err)
		}
		before, err := os.ReadFile(f.userPath())
		if err != nil {
			t.Fatalf("read user file: %v", err)
		}

		res := reg.SaveAll()

		if len(res.Files) != 0 {
			t.Errorf("SaveAll Files = %#v, want none for clean sessions", res.Files)
		}
		if user.saved != 0 || mcp.saved != 0 {
			t.Errorf("save counts = %d,%d, want 0,0", user.saved, mcp.saved)
		}
		if after := readFile(t, f.userPath()); after != string(before) {
			t.Errorf("clean file rewritten: %q", after)
		}
	})

	t.Run("real editor session saves through the seam", func(t *testing.T) {
		f := newFixture(t)
		f.write(t, f.userPath(), validUserConfig)
		reg := f.registry()
		src, ok := reg.Source("user")
		if !ok {
			t.Fatal("user source missing")
		}
		ed, ok := src.Session.(*editor.Editor)
		if !ok {
			t.Fatalf("user session = %T, want *editor.Editor", src.Session)
		}
		if err := ed.SetScalar("/models/k3/model", "acme/other"); err != nil {
			t.Fatalf("SetScalar: %v", err)
		}

		res := reg.SaveAll()

		fr := resultByID(t, res, "user")
		if fr.Err != nil {
			t.Fatalf("user Err = %v", fr.Err)
		}
		if fr.Backup == "" {
			t.Error("user Backup empty, want the editor backup path")
		} else if _, err := os.Stat(fr.Backup); err != nil {
			t.Errorf("backup not on disk: %v", err)
		}
		if got := readFile(t, f.userPath()); !strings.Contains(got, "acme/other") {
			t.Errorf("user file = %q, want edited content", got)
		}
		if dirty := ed.DirtyPaths(); len(dirty) != 0 {
			t.Errorf("editor dirty after save = %v, want cleared", dirty)
		}
	})
}

func TestSaveAllAttach(t *testing.T) {
	f := newFixture(t)
	f.write(t, f.userPath(), validUserConfig)
	reg := f.registry()

	if err := reg.AttachSession("nope", newFakeSession(filepath.Join(t.TempDir(), "x"), "")); err == nil {
		t.Error("AttachSession(unknown) = nil, want an error")
	}
	if err := reg.AttachSession("user", nil); err == nil {
		t.Error("AttachSession(nil) = nil, want an error")
	}
	other := newFakeSession(filepath.Join(t.TempDir(), "elsewhere.jsonc"), "")
	if err := reg.AttachSession("user", other); err == nil {
		t.Error("AttachSession(path mismatch) = nil, want an error")
	}
}

func TestReloadRefreshesStaleSnapshot(t *testing.T) {
	f, reg, _, mcp := twoFakeRegistry(t)
	tampered := `{"mcpServers": {"fetch": {"type": "stdio", "command": "tampered"}}}`
	if err := os.WriteFile(f.mcpPath(), []byte(tampered), 0o644); err != nil {
		t.Fatalf("tampering mcp file: %v", err)
	}
	if res := reg.SaveAll(); !resultByID(t, res, "mcp").Stale {
		t.Fatal("mcp not stale before reload, test setup broken")
	}

	if err := reg.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	// Reload drops the pending edit onto the tampered bytes; re-apply an
	// edit and the save must succeed against the refreshed snapshot.
	mcp.edit(`{"mcpServers": {"fetch": {"type": "stdio", "command": "npx", "enabled": true}}}`)

	res := reg.SaveAll()
	fr := resultByID(t, res, "mcp")
	if fr.Err != nil || fr.Stale {
		t.Errorf("mcp result after reload = %+v, want a clean save", fr)
	}
	if got := readFile(t, f.mcpPath()); !strings.Contains(got, `"enabled": true`) {
		t.Errorf("mcp file = %q, want re-applied content", got)
	}
	if mcp.reloaded != 1 {
		t.Errorf("mcp reloaded = %d, want 1", mcp.reloaded)
	}
}
