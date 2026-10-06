package derived_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	if _, err := derived.LoadTools(filepath.Join(dir, "no-cache.json")); err == nil {
		t.Error("LoadTools(missing) = nil, want an error")
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
	badTools := writeDerivedFile(t, dir, "mcp-cache.json", `{"servers": {"acme": {"tools": `)
	if _, err := derived.LoadTools(badTools); err == nil {
		t.Error("LoadTools(malformed) = nil, want an error")
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

const (
	epochCatalogFixture = `{
  "acme": {
    "models": [
      {"id": "code-large", "name": "Code Large", "cost": {"input": 0.01}, "contextWindow": 200000},
      {"id": "extra-9", "name": "Extra 9", "cost": {"input": 0.02}, "contextWindow": 100000},
      {"id": "other-1", "name": "Other 1"}
    ],
    "checkedAt": 1789027833963,
    "lastModified": 1788948060000,
    "etag": "etag-acme"
  },
  "beta": {
    "models": [
      {"id": "model-x", "name": "Model X", "cost": {"input": 0.03}, "contextWindow": 50000},
      {"id": "model-y", "name": "Model Y"}
    ],
    "checkedAt": 1789027833963,
    "lastModified": 1788948060000,
    "etag": "etag-beta"
  }
}`

	malformedCatalogFixture = `{
  "acme": {
    "models": [
      {"id": "code-large", "name": "Code Large"},
      "junk-entry",
      {"id": "extra-9", "name": "Extra 9"}
    ],
    "checkedAt": 1789027833963,
    "etag": "etag-acme"
  }
}`

	malformedProvidersFixture = `{
  "providers": {
    "acme": {
      "name": "Acme",
      "api": "openai-completions",
      "baseUrl": 12345,
      "models": [{"id": "code-large"}]
    }
  }
}`
)

func TestLoadCatalogEpochTimestamps(t *testing.T) {
	dir := t.TempDir()
	path := writeDerivedFile(t, dir, "models-store.json", epochCatalogFixture)

	snap, err := derived.LoadCatalog(path)
	if err != nil {
		t.Fatalf("LoadCatalog() error = %v", err)
	}
	if len(snap.Providers) != 2 {
		t.Fatalf("catalog providers = %d, want 2 (numeric timestamps must not drop providers)", len(snap.Providers))
	}
	if snap.TotalModels != 5 {
		t.Fatalf("TotalModels = %d, want 5", snap.TotalModels)
	}
	byName := map[string]derived.CatalogProvider{}
	for _, p := range snap.Providers {
		byName[p.Name] = p
	}
	acme := byName["acme"]
	if !strings.Contains(acme.CheckedAt, "1789027833963") {
		t.Errorf("acme CheckedAt = %q, want the epoch value", acme.CheckedAt)
	}
	if len(acme.Models) != 3 {
		t.Fatalf("acme models = %d, want 3", len(acme.Models))
	}
	if acme.Models[0].Cost == "" {
		t.Error("object cost must render best-effort, got empty")
	}
	if acme.Models[0].ContextWindow != 200000 {
		t.Errorf("ContextWindow = %d, want 200000", acme.Models[0].ContextWindow)
	}
	t.Logf("epoch catalog: providers=2 total=5 checkedAt=%q", acme.CheckedAt)
}

func TestLoadCatalogMalformedFieldSurvives(t *testing.T) {
	dir := t.TempDir()
	path := writeDerivedFile(t, dir, "models-store.json", malformedCatalogFixture)

	snap, err := derived.LoadCatalog(path)
	if err != nil {
		t.Fatalf("LoadCatalog() error = %v", err)
	}
	if len(snap.Providers) != 1 {
		t.Fatalf("catalog providers = %d, want 1 (one bad field must not drop the entry)", len(snap.Providers))
	}
	acme := snap.Providers[0]
	if acme.Name != "acme" {
		t.Fatalf("provider = %q, want acme", acme.Name)
	}
	if len(acme.Models) != 2 {
		t.Fatalf("acme models = %d, want 2 (junk element skipped, valid kept)", len(acme.Models))
	}
	if snap.TotalModels != 2 {
		t.Errorf("TotalModels = %d, want 2", snap.TotalModels)
	}
}

func TestLoadProvidersMalformedFieldSurvives(t *testing.T) {
	dir := t.TempDir()
	path := writeDerivedFile(t, dir, "models.json", malformedProvidersFixture)

	snap, err := derived.LoadProviders(path)
	if err != nil {
		t.Fatalf("LoadProviders() error = %v", err)
	}
	if len(snap.Providers) != 1 {
		t.Fatalf("providers = %d, want 1 (numeric baseUrl must not drop the entry)", len(snap.Providers))
	}
	if snap.Providers[0].ModelCount != 1 {
		t.Errorf("ModelCount = %d, want 1", snap.Providers[0].ModelCount)
	}
}

const (
	toolsFixture = `{
  "servers": {
    "acme": {
      "configHash": "hash-acme",
      "fetchedAt": 1789027833963,
      "tools": [
        {"name": "create-issue", "description": "Create an issue", "inputSchema": {"type": "object"}},
        {"name": "list-issues", "description": "List issues"}
      ]
    },
    "beta": {
      "configHash": "hash-beta",
      "fetchedAt": 1789027900000,
      "tools": [
        {"name": "search", "description": "Search documents"}
      ]
    }
  }
}`

	toolsZeroFixture = `{
  "servers": {
    "empty": {
      "configHash": "hash-empty",
      "fetchedAt": 1789027833963,
      "tools": []
    }
  }
}`

	toolsJunkFixture = `{
  "servers": {
    "acme": {
      "configHash": "hash-acme",
      "fetchedAt": 1789027833963,
      "tools": [
        {"name": "real", "description": "kept"},
        "junk-string",
        42,
        {"description": "no name"},
        {"name": "also-real"}
      ]
    }
  }
}`

	toolsStringTimeFixture = `{
  "servers": {
    "acme": {
      "configHash": "hash-acme",
      "fetchedAt": "2026-10-06T00:00:00Z",
      "tools": []
    }
  }
}`
)

func TestLoadToolsCounts(t *testing.T) {
	dir := t.TempDir()
	path := writeDerivedFile(t, dir, "mcp-cache.json", toolsFixture)

	snap, err := derived.LoadTools(path)
	if err != nil {
		t.Fatalf("LoadTools() error = %v", err)
	}
	if len(snap.Servers) != 2 {
		t.Fatalf("servers = %d, want 2", len(snap.Servers))
	}
	if snap.TotalTools != 3 {
		t.Fatalf("TotalTools = %d, want 3", snap.TotalTools)
	}
	acme := snap.Servers[0]
	if acme.Name != "acme" {
		t.Fatalf("first server = %q, want acme (sorted)", acme.Name)
	}
	if acme.ToolCount != 2 {
		t.Errorf("acme ToolCount = %d, want 2", acme.ToolCount)
	}
	if acme.FetchedAt != "1789027833963" {
		t.Errorf("acme FetchedAt = %q, want the epoch number literally", acme.FetchedAt)
	}
	if len(acme.Tools) != 2 {
		t.Fatalf("acme tools = %d, want 2", len(acme.Tools))
	}
	if acme.Tools[0].Name != "create-issue" || acme.Tools[0].Description != "Create an issue" {
		t.Errorf("acme tool 0 = %+v", acme.Tools[0])
	}
	if acme.Tools[1].Name != "list-issues" || acme.Tools[1].Description != "List issues" {
		t.Errorf("acme tool 1 = %+v", acme.Tools[1])
	}
	beta := snap.Servers[1]
	if beta.Name != "beta" || beta.ToolCount != 1 {
		t.Fatalf("beta = %+v, want 1 tool", beta)
	}
	if beta.Tools[0].Name != "search" || beta.Tools[0].Description != "Search documents" {
		t.Errorf("beta tool 0 = %+v", beta.Tools[0])
	}
	if snap.Path != path {
		t.Errorf("Path = %q, want %q", snap.Path, path)
	}
	if snap.RefreshedAt.IsZero() {
		t.Error("RefreshedAt is zero, want file mtime")
	}
	t.Logf("tools: servers=2 total=3 acmeFetchedAt=%q", acme.FetchedAt)
}

func TestLoadToolsZeroToolsServer(t *testing.T) {
	dir := t.TempDir()
	path := writeDerivedFile(t, dir, "mcp-cache.json", toolsZeroFixture)

	snap, err := derived.LoadTools(path)
	if err != nil {
		t.Fatalf("LoadTools() error = %v", err)
	}
	if len(snap.Servers) != 1 {
		t.Fatalf("servers = %d, want 1 (zero-tool server must be kept)", len(snap.Servers))
	}
	if snap.Servers[0].ToolCount != 0 {
		t.Errorf("ToolCount = %d, want 0", snap.Servers[0].ToolCount)
	}
	if len(snap.Servers[0].Tools) != 0 {
		t.Errorf("Tools = %d entries, want 0", len(snap.Servers[0].Tools))
	}
	if snap.TotalTools != 0 {
		t.Errorf("TotalTools = %d, want 0", snap.TotalTools)
	}
}

func TestLoadToolsJunkEntriesSkipped(t *testing.T) {
	dir := t.TempDir()
	path := writeDerivedFile(t, dir, "mcp-cache.json", toolsJunkFixture)

	snap, err := derived.LoadTools(path)
	if err != nil {
		t.Fatalf("LoadTools() error = %v", err)
	}
	if len(snap.Servers) != 1 {
		t.Fatalf("servers = %d, want 1 (junk tools must not drop the server)", len(snap.Servers))
	}
	acme := snap.Servers[0]
	if acme.ToolCount != 2 {
		t.Fatalf("ToolCount = %d, want 2 (junk entries skipped)", acme.ToolCount)
	}
	if acme.Tools[0].Name != "real" || acme.Tools[1].Name != "also-real" {
		t.Errorf("tools = %+v, want real then also-real", acme.Tools)
	}
	if snap.TotalTools != 2 {
		t.Errorf("TotalTools = %d, want 2", snap.TotalTools)
	}
}

func TestLoadToolsStringFetchedAt(t *testing.T) {
	dir := t.TempDir()
	path := writeDerivedFile(t, dir, "mcp-cache.json", toolsStringTimeFixture)

	snap, err := derived.LoadTools(path)
	if err != nil {
		t.Fatalf("LoadTools() error = %v", err)
	}
	if snap.Servers[0].FetchedAt != "2026-10-06T00:00:00Z" {
		t.Errorf("FetchedAt = %q, want the ISO string", snap.Servers[0].FetchedAt)
	}
}

func TestLoadToolsRebuildReflectsChange(t *testing.T) {
	dir := t.TempDir()
	path := writeDerivedFile(t, dir, "mcp-cache.json", toolsFixture)
	first, err := derived.LoadTools(path)
	if err != nil {
		t.Fatalf("first LoadTools() error = %v", err)
	}
	if first.TotalTools != 3 {
		t.Fatalf("first TotalTools = %d, want 3", first.TotalTools)
	}
	changed := `{"servers": {"solo": {"fetchedAt": 1789027833963, "tools": [{"name": "only", "description": "Only tool"}]}}}`
	if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
		t.Fatalf("rewrite mcp-cache.json: %v", err)
	}
	second, err := derived.LoadTools(path)
	if err != nil {
		t.Fatalf("second LoadTools() error = %v", err)
	}
	if len(second.Servers) != 1 || second.Servers[0].Name != "solo" {
		t.Fatalf("rebuilt servers = %+v, want single solo", second.Servers)
	}
	if second.TotalTools != 1 {
		t.Fatalf("rebuilt TotalTools = %d, want 1", second.TotalTools)
	}
}

func TestLoadToolsPerfGuard(t *testing.T) {
	path := filepath.Join("testdata", "mcp-cache-large.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat fixture: %v", err)
	}
	if info.Size() < 1_000_000 {
		t.Fatalf("fixture size = %d, want ~1.1 MB", info.Size())
	}
	start := time.Now()
	snap, err := derived.LoadTools(path)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("LoadTools() error = %v", err)
	}
	if len(snap.Servers) != 16 {
		t.Fatalf("servers = %d, want 16", len(snap.Servers))
	}
	if snap.TotalTools != 463 {
		t.Fatalf("TotalTools = %d, want 463", snap.TotalTools)
	}
	if elapsed > 150*time.Millisecond {
		t.Fatalf("LoadTools took %v, want < 150 ms", elapsed)
	}
	t.Logf("perf: %d servers / %d tools from %d bytes in %v", len(snap.Servers), snap.TotalTools, info.Size(), elapsed)
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
	toolsPath := writeDerivedFile(t, dir, "mcp-cache.json", toolsFixture)
	beforeTools, err := os.ReadFile(toolsPath)
	if err != nil {
		t.Fatalf("reading tools fixture: %v", err)
	}
	if _, err := derived.LoadTools(toolsPath); err != nil {
		t.Fatalf("LoadTools() error = %v", err)
	}
	afterProviders, _ := os.ReadFile(providersPath)
	afterCatalog, _ := os.ReadFile(catalogPath)
	afterTools, _ := os.ReadFile(toolsPath)
	if string(beforeProviders) != string(afterProviders) {
		t.Error("LoadProviders modified models.json")
	}
	if string(beforeCatalog) != string(afterCatalog) {
		t.Error("LoadCatalog modified models-store.json")
	}
	if string(beforeTools) != string(afterTools) {
		t.Error("LoadTools modified mcp-cache.json")
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
