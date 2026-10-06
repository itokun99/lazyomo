package workspace_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/itokun99/lazyomo/internal/editor"
	"github.com/itokun99/lazyomo/internal/fuzzy"
	"github.com/itokun99/lazyomo/internal/workspace"
)

const (
	candidatesStoreFixture = `{
  "acme": {
    "models": [
      {"id": "code-large", "provider": "acme"},
      {"id": "other-1", "provider": "acme"}
    ],
    "checkedAt": "2026-10-06T00:00:00Z"
  },
  "beta": {
    "models": [
      {"id": "model-x", "provider": "beta"}
    ],
    "checkedAt": "2026-10-06T00:00:00Z"
  }
}`

	candidatesModelsFixture = `{
  "providers": {
    "acme": {
      "name": "Acme",
      "api": "openai-completions",
      "baseUrl": "https://api.acme.test/v1",
      "models": [
        {"id": "code-large"},
        {"id": "extra-9"}
      ]
    },
    "gamma": {
      "name": "Gamma",
      "api": "openai-completions",
      "baseUrl": "https://api.gamma.test/v1",
      "models": [
        {"id": "g-1"}
      ]
    }
  }
}`

	candidatesDocFixture = `{
  "models": {
    "k3": {"model": "acme/code-large", "reasoning": "max"},
    "sonnet": {"model": "acme/sonnet-5"}
  },
  "agents": {
    "sisyphus": {"model": "k3", "models": ["k3:max", "acme/code-large"]},
    "other": {"model": "acme/other-1"}
  },
  "categories": {
    "quick": {"model": "sonnet", "models": ["sonnet:high", "beta/model-x"]}
  },
  "model_profiles": {
    "daily": {"display_name": "Daily", "models": ["k3", "acme/code-large:low"]}
  },
  "model_profile": "acme/pinned-1"
}`
)

func writeCandidatesFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func loadCandidatesEditor(t *testing.T, content string) *editor.Editor {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "omo.jsonc")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write omo.jsonc: %v", err)
	}
	ed, err := editor.Load(path)
	if err != nil {
		t.Fatalf("editor.Load: %v", err)
	}
	return ed
}

func candidateValues(got []workspace.Candidate) []string {
	out := make([]string, len(got))
	for i, c := range got {
		out[i] = c.Value
	}
	return out
}

func candidateByValue(t *testing.T, got []workspace.Candidate, value string) workspace.Candidate {
	t.Helper()
	for _, c := range got {
		if c.Value == value {
			return c
		}
	}
	t.Fatalf("candidate %q not found in %v", value, candidateValues(got))
	return workspace.Candidate{}
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func TestCandidatesCatalogSorted(t *testing.T) {
	dir := t.TempDir()
	storePath := writeCandidatesFile(t, dir, "models-store.json", candidatesStoreFixture)
	modelsPath := writeCandidatesFile(t, dir, "models.json", candidatesModelsFixture)

	got := workspace.CatalogCandidates(storePath, modelsPath)
	want := []string{
		"acme/code-large",
		"acme/extra-9",
		"acme/other-1",
		"beta/model-x",
		"gamma/g-1",
	}
	if !reflect.DeepEqual(candidateValues(got), want) {
		t.Fatalf("CatalogCandidates() values = %v, want %v", candidateValues(got), want)
	}
	for _, c := range got {
		if c.Group != fuzzy.GroupRef {
			t.Errorf("candidate %q Group = %d, want GroupRef %d", c.Value, c.Group, fuzzy.GroupRef)
		}
	}
	// Logged for evidence: struct-level assertion above is the truth.
	t.Logf("catalog candidates: %v", candidateValues(got))
}

func TestCandidatesCatalogDedupeFirstWins(t *testing.T) {
	dir := t.TempDir()
	// acme/code-large appears in both files; it must appear exactly once.
	storePath := writeCandidatesFile(t, dir, "models-store.json", candidatesStoreFixture)
	modelsPath := writeCandidatesFile(t, dir, "models.json", candidatesModelsFixture)

	got := workspace.CatalogCandidates(storePath, modelsPath)
	count := 0
	for _, c := range got {
		if c.Value == "acme/code-large" {
			count++
			if c.Provider != "acme" {
				t.Errorf("acme/code-large Provider = %q, want %q", c.Provider, "acme")
			}
		}
	}
	if count != 1 {
		t.Fatalf("acme/code-large appears %d times, want exactly once (first wins)", count)
	}
}

func TestCandidatesDocumentAliasesCarryUsages(t *testing.T) {
	ed := loadCandidatesEditor(t, candidatesDocFixture)
	got := workspace.DocumentCandidates(ed)

	k3 := candidateByValue(t, got, "k3")
	if k3.Group != fuzzy.GroupAlias {
		t.Errorf("k3 Group = %d, want GroupAlias %d", k3.Group, fuzzy.GroupAlias)
	}
	for _, want := range []string{
		"/agents/sisyphus/model",
		"/agents/sisyphus/models/0",
		"/model_profiles/daily/models/0",
	} {
		if !containsString(k3.Usages, want) {
			t.Errorf("k3 Usages = %v, want to contain %q", k3.Usages, want)
		}
	}

	sonnet := candidateByValue(t, got, "sonnet")
	if sonnet.Group != fuzzy.GroupAlias {
		t.Errorf("sonnet Group = %d, want GroupAlias %d", sonnet.Group, fuzzy.GroupAlias)
	}
	for _, want := range []string{
		"/categories/quick/model",
		"/categories/quick/models/0",
	} {
		if !containsString(sonnet.Usages, want) {
			t.Errorf("sonnet Usages = %v, want to contain %q", sonnet.Usages, want)
		}
	}
	t.Logf("document aliases: k3=%v sonnet=%v", k3.Usages, sonnet.Usages)
}

func TestCandidatesDocumentRefsSuffixStripped(t *testing.T) {
	ed := loadCandidatesEditor(t, candidatesDocFixture)
	got := workspace.DocumentCandidates(ed)

	// :suffix forms must harvest stripped, never with the suffix.
	for _, c := range got {
		for _, u := range c.Usages {
			_ = u
		}
		if c.Group == fuzzy.GroupRef {
			for _, bad := range []string{"k3:max", "sonnet:high", "acme/code-large:low"} {
				if c.Value == bad {
					t.Errorf("candidate value %q still carries :suffix, want stripped", bad)
				}
			}
		}
	}
	// The stripped ref carries merged usages from every occurrence.
	codeLarge := candidateByValue(t, got, "acme/code-large")
	for _, want := range []string{
		"/models/k3/model",
		"/agents/sisyphus/models/1",
		"/model_profiles/daily/models/1",
	} {
		if !containsString(codeLarge.Usages, want) {
			t.Errorf("acme/code-large Usages = %v, want to contain %q", codeLarge.Usages, want)
		}
	}
	if codeLarge.Provider != "acme" {
		t.Errorf("acme/code-large Provider = %q, want %q", codeLarge.Provider, "acme")
	}
	t.Logf("acme/code-large usages: %v", codeLarge.Usages)
}

func TestCandidatesBuildUnionOrder(t *testing.T) {
	dir := t.TempDir()
	storePath := writeCandidatesFile(t, dir, "models-store.json", candidatesStoreFixture)
	modelsPath := writeCandidatesFile(t, dir, "models.json", candidatesModelsFixture)
	ed := loadCandidatesEditor(t, candidatesDocFixture)

	got := workspace.BuildCandidates(storePath, modelsPath, ed)
	want := []string{
		"k3",
		"sonnet",
		"acme/code-large",
		"acme/extra-9",
		"acme/other-1",
		"acme/pinned-1",
		"acme/sonnet-5",
		"beta/model-x",
		"gamma/g-1",
	}
	if !reflect.DeepEqual(candidateValues(got), want) {
		t.Fatalf("BuildCandidates() values = %v, want %v", candidateValues(got), want)
	}
	// Catalog+document duplicate merges usages instead of duplicating.
	codeLarge := candidateByValue(t, got, "acme/code-large")
	if !containsString(codeLarge.Usages, "/models/k3/model") {
		t.Errorf("merged acme/code-large Usages = %v, want document usage", codeLarge.Usages)
	}
	// Every candidate converts losslessly for fuzzy.Filter.
	fuzzyCands := workspace.ToFuzzy(got)
	if len(fuzzyCands) != len(got) {
		t.Fatalf("ToFuzzy() len = %d, want %d", len(fuzzyCands), len(got))
	}
	for i := range got {
		if fuzzyCands[i].Value != got[i].Value || fuzzyCands[i].Group != got[i].Group {
			t.Fatalf("ToFuzzy()[%d] = %+v, want Value/Group of %+v", i, fuzzyCands[i], got[i])
		}
	}
	t.Logf("built candidates: %v", candidateValues(got))
}

func TestCandidatesEmptyCatalogEmptyList(t *testing.T) {
	dir := t.TempDir()
	// Missing files → empty, no crash.
	if got := workspace.CatalogCandidates(
		filepath.Join(dir, "no-store.json"),
		filepath.Join(dir, "no-models.json"),
	); len(got) != 0 {
		t.Fatalf("CatalogCandidates() on missing files = %v, want empty", candidateValues(got))
	}
	// Empty files → empty, no crash.
	emptyStore := writeCandidatesFile(t, dir, "empty-store.json", ``)
	emptyModels := writeCandidatesFile(t, dir, "empty-models.json", `{}`)
	if got := workspace.CatalogCandidates(emptyStore, emptyModels); len(got) != 0 {
		t.Fatalf("CatalogCandidates() on empty files = %v, want empty", candidateValues(got))
	}
	// Empty catalog + empty document → empty build, no crash.
	emptyDoc := loadCandidatesEditor(t, `{}`)
	if got := workspace.BuildCandidates(emptyStore, emptyModels, emptyDoc); len(got) != 0 {
		t.Fatalf("BuildCandidates() on empty catalog+doc = %v, want empty", candidateValues(got))
	}
	// Nil editor → catalog only, still no crash.
	dir2 := t.TempDir()
	storePath := writeCandidatesFile(t, dir2, "models-store.json", candidatesStoreFixture)
	modelsPath := writeCandidatesFile(t, dir2, "models.json", candidatesModelsFixture)
	if got := workspace.BuildCandidates(storePath, modelsPath, nil); len(got) == 0 {
		t.Fatal("BuildCandidates() with nil editor returned empty, want catalog refs")
	}
}

func TestCandidatesMalformedCatalogNoCrash(t *testing.T) {
	dir := t.TempDir()
	badStore := writeCandidatesFile(t, dir, "models-store.json", `{not json`)
	badModels := writeCandidatesFile(t, dir, "models.json", `{"providers": {"a": `)
	if got := workspace.CatalogCandidates(badStore, badModels); len(got) != 0 {
		t.Fatalf("CatalogCandidates() on malformed JSON = %v, want empty", candidateValues(got))
	}
	// Malformed catalog + valid document still yields document candidates, no crash.
	ed := loadCandidatesEditor(t, candidatesDocFixture)
	got := workspace.BuildCandidates(badStore, badModels, ed)
	if len(got) == 0 {
		t.Fatal("BuildCandidates() on malformed catalog + valid doc returned empty, want document candidates")
	}
	if got := candidateByValue(t, got, "k3"); got.Group != fuzzy.GroupAlias {
		t.Fatalf("k3 Group = %d, want alias", got.Group)
	}
}

func TestCandidatesRebuildReflectsChange(t *testing.T) {
	dir := t.TempDir()
	storePath := writeCandidatesFile(t, dir, "models-store.json", candidatesStoreFixture)
	modelsPath := writeCandidatesFile(t, dir, "models.json", candidatesModelsFixture)

	first := workspace.CatalogCandidates(storePath, modelsPath)
	if len(first) == 0 {
		t.Fatal("first CatalogCandidates() is empty, want fixtures")
	}
	// stale_state: changing the file must be reflected on rebuild (no cache).
	changed := `{"providers": {"solo": {"models": [{"id": "only-1"}]}}}`
	if err := os.WriteFile(modelsPath, []byte(changed), 0o644); err != nil {
		t.Fatalf("rewrite models.json: %v", err)
	}
	second := workspace.CatalogCandidates(storePath, modelsPath)
	want := []string{"acme/code-large", "acme/other-1", "beta/model-x", "solo/only-1"}
	if !reflect.DeepEqual(candidateValues(second), want) {
		t.Fatalf("rebuilt CatalogCandidates() = %v, want %v", candidateValues(second), want)
	}
}
