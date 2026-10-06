package derived_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/itokun99/lazyomo/internal/derived"
)

const (
	providersFixture = `{
  "providers": {
    "acme": {
      "name": "Acme",
      "api": "openai-completions",
      "baseUrl": "https://api.acme.test/v1",
      "models": [
        {"id": "code-large"},
        {"id": "extra-9"},
        {"id": "other-1"}
      ]
    },
    "beta": {
      "name": "Beta",
      "api": "anthropic",
      "baseUrl": "https://api.beta.test/v1",
      "models": [
        {"id": "model-x"},
        {"id": "model-y"}
      ]
    }
  }
}`

	catalogFixture = `{
  "acme": {
    "models": [
      {"id": "code-large", "name": "Code Large", "cost": "0.01", "contextWindow": 200000},
      {"id": "extra-9", "name": "Extra 9", "cost": "0.02", "contextWindow": 100000},
      {"id": "other-1", "name": "Other 1"}
    ],
    "checkedAt": "2026-10-06T00:00:00Z",
    "etag": "etag-acme"
  },
  "beta": {
    "models": [
      {"id": "model-x", "name": "Model X", "cost": "0.03", "contextWindow": 50000},
      {"id": "model-y", "name": "Model Y"}
    ],
    "checkedAt": "2026-10-06T01:00:00Z",
    "etag": "etag-beta"
  }
}`
)

func writeDerivedFile(t *testing.T, dir, name, content string) string {
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

func TestLoadProvidersCounts(t *testing.T) {
	dir := t.TempDir()
	path := writeDerivedFile(t, dir, "models.json", providersFixture)

	snap, err := derived.LoadProviders(path)
	if err != nil {
		t.Fatalf("LoadProviders() error = %v", err)
	}
	if len(snap.Providers) != 2 {
		t.Fatalf("providers = %d, want 2", len(snap.Providers))
	}
	byKey := map[string]derived.Provider{}
	for _, p := range snap.Providers {
		byKey[p.Key] = p
	}
	acme, ok := byKey["acme"]
	if !ok {
		t.Fatalf("provider acme missing: %+v", snap.Providers)
	}
	if acme.Name != "Acme" {
		t.Errorf("acme Name = %q, want Acme", acme.Name)
	}
	if acme.API != "openai-completions" {
		t.Errorf("acme API = %q, want openai-completions", acme.API)
	}
	if acme.BaseURL != "https://api.acme.test/v1" {
		t.Errorf("acme BaseURL = %q", acme.BaseURL)
	}
	if acme.ModelCount != 3 {
		t.Errorf("acme ModelCount = %d, want 3", acme.ModelCount)
	}
	beta, ok := byKey["beta"]
	if !ok {
		t.Fatalf("provider beta missing")
	}
	if beta.ModelCount != 2 {
		t.Errorf("beta ModelCount = %d, want 2", beta.ModelCount)
	}
	if snap.Path != path {
		t.Errorf("Path = %q, want %q", snap.Path, path)
	}
	if snap.RefreshedAt.IsZero() {
		t.Error("RefreshedAt is zero, want file mtime")
	}
	t.Logf("providers: acme=%d beta=%d", acme.ModelCount, beta.ModelCount)
}

func TestLoadCatalogCounts(t *testing.T) {
	dir := t.TempDir()
	path := writeDerivedFile(t, dir, "models-store.json", catalogFixture)

	snap, err := derived.LoadCatalog(path)
	if err != nil {
		t.Fatalf("LoadCatalog() error = %v", err)
	}
	if len(snap.Providers) != 2 {
		t.Fatalf("catalog providers = %d, want 2", len(snap.Providers))
	}
	if snap.TotalModels != 5 {
		t.Fatalf("TotalModels = %d, want 5", snap.TotalModels)
	}
	byName := map[string]derived.CatalogProvider{}
	for _, p := range snap.Providers {
		byName[p.Name] = p
	}
	acme := byName["acme"]
	if len(acme.Models) != 3 {
		t.Fatalf("acme models = %d, want 3", len(acme.Models))
	}
	if acme.CheckedAt != "2026-10-06T00:00:00Z" {
		t.Errorf("acme CheckedAt = %q", acme.CheckedAt)
	}
	if acme.ETag != "etag-acme" {
		t.Errorf("acme ETag = %q", acme.ETag)
	}
	first := acme.Models[0]
	if first.ID != "code-large" {
		t.Errorf("first model ID = %q, want code-large", first.ID)
	}
	if first.Name != "Code Large" {
		t.Errorf("first model Name = %q, want Code Large", first.Name)
	}
	if first.Cost == "" {
		t.Error("first model Cost is empty, want 0.01")
	} else if !strings.Contains(first.Cost, "0.01") {
		t.Errorf("first model Cost = %q, want to contain 0.01", first.Cost)
	}
	if first.ContextWindow == 0 {
		t.Error("first model ContextWindow is 0, want 200000")
	}
	if snap.Path != path {
		t.Errorf("Path = %q, want %q", snap.Path, path)
	}
	if snap.RefreshedAt.IsZero() {
		t.Error("RefreshedAt is zero, want file mtime")
	}
	t.Logf("catalog: providers=2 total=5")
}

func TestDerivedMissingEmptyState(t *testing.T) {
	dir := t.TempDir()
	if _, err := derived.LoadProviders(filepath.Join(dir, "no-models.json")); err == nil {
		t.Error("LoadProviders(missing) = nil, want an error")
	}
	if _, err := derived.LoadCatalog(filepath.Join(dir, "no-store.json")); err == nil {
		t.Error("LoadCatalog(missing) = nil, want an error")
	}
}

func TestDerivedMalformedNoCrash(t *testing.T) {
	dir := t.TempDir()
	badProviders := writeDerivedFile(t, dir, "models.json", `{not json`)
	if _, err := derived.LoadProviders(badProviders); err == nil {
		t.Error("LoadProviders(malformed) = nil, want an error")
	}
	badCatalog := writeDerivedFile(t, dir, "models-store.json", `{"acme": {"models": `)
	if _, err := derived.LoadCatalog(badCatalog); err == nil {
		t.Error("LoadCatalog(malformed) = nil, want an error")
	}
}

func TestDerivedRebuildReflectsChange(t *testing.T) {
	dir := t.TempDir()
	providersPath := writeDerivedFile(t, dir, "models.json", providersFixture)
	first, err := derived.LoadProviders(providersPath)
	if err != nil {
		t.Fatalf("first LoadProviders() error = %v", err)
	}
	if len(first.Providers) != 2 {
		t.Fatalf("first providers = %d, want 2", len(first.Providers))
	}
	changed := `{"providers": {"solo": {"name": "Solo", "api": "openai-completions", "baseUrl": "https://solo.test", "models": [{"id": "only-1"}]}}}`
	if err := os.WriteFile(providersPath, []byte(changed), 0o644); err != nil {
		t.Fatalf("rewrite models.json: %v", err)
	}
	second, err := derived.LoadProviders(providersPath)
	if err != nil {
		t.Fatalf("second LoadProviders() error = %v", err)
	}
	if len(second.Providers) != 1 || second.Providers[0].Key != "solo" {
		t.Fatalf("rebuilt providers = %+v, want single solo", second.Providers)
	}

	catalogPath := writeDerivedFile(t, dir, "models-store.json", catalogFixture)
	cfirst, err := derived.LoadCatalog(catalogPath)
	if err != nil {
		t.Fatalf("first LoadCatalog() error = %v", err)
	}
	if cfirst.TotalModels != 5 {
		t.Fatalf("first TotalModels = %d, want 5", cfirst.TotalModels)
	}
	cchanged := `{"solo": {"models": [{"id": "only-1", "name": "Only 1"}], "checkedAt": "2026-10-06T02:00:00Z", "etag": "etag-solo"}}`
	if err := os.WriteFile(catalogPath, []byte(cchanged), 0o644); err != nil {
		t.Fatalf("rewrite models-store.json: %v", err)
	}
	csecond, err := derived.LoadCatalog(catalogPath)
	if err != nil {
		t.Fatalf("second LoadCatalog() error = %v", err)
	}
	if csecond.TotalModels != 1 {
		t.Fatalf("rebuilt TotalModels = %d, want 1", csecond.TotalModels)
	}
}

func TestDerivedLoadIsReadOnly(t *testing.T) {
	dir := t.TempDir()
	providersPath := writeDerivedFile(t, dir, "models.json", providersFixture)
	catalogPath := writeDerivedFile(t, dir, "models-store.json", catalogFixture)
	beforeProviders, err := os.ReadFile(providersPath)
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	beforeCatalog, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	if _, err := derived.LoadProviders(providersPath); err != nil {
		t.Fatalf("LoadProviders() error = %v", err)
	}
	if _, err := derived.LoadCatalog(catalogPath); err != nil {
		t.Fatalf("LoadCatalog() error = %v", err)
	}
	afterProviders, _ := os.ReadFile(providersPath)
	afterCatalog, _ := os.ReadFile(catalogPath)
	if string(beforeProviders) != string(afterProviders) {
		t.Error("LoadProviders modified models.json")
	}
	if string(beforeCatalog) != string(afterCatalog) {
		t.Error("LoadCatalog modified models-store.json")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading dir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".bak.") {
			t.Errorf("reader created backup %q, want no writes", e.Name())
		}
	}
}
