package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	providersModelsFixture = `{
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

	providersStoreFixture = `{
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

func providersFixture(t *testing.T) (modelsPath, storePath string) {
	t.Helper()
	dir := t.TempDir()
	modelsPath = filepath.Join(dir, "agent", "models.json")
	storePath = filepath.Join(dir, "agent", "models-store.json")
	writeFixture(t, modelsPath, providersModelsFixture)
	writeFixture(t, storePath, providersStoreFixture)
	return modelsPath, storePath
}

func newProvidersModel(t *testing.T, modelsPath, storePath string) *Model {
	t.Helper()
	m := newTestModel(newFakeWorkspace(newFakeEditor()))
	m.setDerivedPaths(modelsPath, storePath)
	m.refresh()
	return m
}

func TestProvidersConnectionsRendersCounts(t *testing.T) {
	modelsPath, storePath := providersFixture(t)
	m := newProvidersModel(t, modelsPath, storePath)
	m = selectSideRow(t, m, "Connections")
	if got := len(m.providerRows); got != 2 {
		t.Fatalf("provider rows = %d, want 2", got)
	}
	if got := m.activePath(); got != modelsPath {
		t.Errorf("activePath() = %q, want %q", got, modelsPath)
	}
	if m.providers.Path != modelsPath {
		t.Errorf("providers.Path = %q, want %q", m.providers.Path, modelsPath)
	}
	view := m.View()
	for _, want := range []string{"acme", "beta", "Acme", "3 models", "2 models", "derived", "refreshed"} {
		if !strings.Contains(view, want) {
			t.Errorf("connections view missing %q:\n%s", want, view)
		}
	}
}

func TestProvidersCatalogRendersCounts(t *testing.T) {
	modelsPath, storePath := providersFixture(t)
	m := newProvidersModel(t, modelsPath, storePath)
	m = selectSideRow(t, m, "Catalog")
	if got := len(m.catalogRows); got != 5 {
		t.Fatalf("catalog rows = %d, want 5", got)
	}
	if got := m.activePath(); got != storePath {
		t.Errorf("activePath() = %q, want %q", got, storePath)
	}
	if m.catalog.Path != storePath {
		t.Errorf("catalog.Path = %q, want %q", m.catalog.Path, storePath)
	}
	_ = modelsPath
	view := m.View()
	for _, want := range []string{"acme/code-large", "beta/model-x", "Code Large", "derived", "refreshed"} {
		if !strings.Contains(view, want) {
			t.Errorf("catalog view missing %q:\n%s", want, view)
		}
	}
	// The full source path lives in the snapshot and status; the pane
	// width truncates it in View (density contract), so assert the
	// provenance header prefix instead of the whole path.
	m = sendKeys(t, m, "enter")
	view = m.View()
	for _, want := range []string{"checkedAt", "2026-10-06"} {
		if !strings.Contains(view, want) {
			t.Errorf("catalog detail missing %q:\n%s", want, view)
		}
	}
}

func TestProvidersReadOnlyRejected(t *testing.T) {
	modelsPath, storePath := providersFixture(t)
	for _, label := range []string{"Connections", "Catalog"} {
		m := newProvidersModel(t, modelsPath, storePath)
		m = selectSideRow(t, m, label)
		for _, key := range []string{"e", "a", "d", " "} {
			m = sendKeys(t, m, key)
			if m.status != "read-only section" {
				t.Errorf("%s key %q status = %q, want read-only section", label, key, m.status)
			}
			if m.overlay != OverlayNone {
				t.Errorf("%s key %q opened overlay %v", label, key, m.overlay)
			}
		}
		if got := m.dirtyCount(); got != 0 {
			t.Errorf("%s dirty = %d, want 0", label, got)
		}
	}
}

func TestProvidersMissingEmptyState(t *testing.T) {
	dir := t.TempDir()
	m := newProvidersModel(t, filepath.Join(dir, "models.json"), filepath.Join(dir, "models-store.json"))
	m = selectSideRow(t, m, "Connections")
	if view := m.View(); !strings.Contains(view, "unavailable") {
		t.Errorf("connections missing-file view must say unavailable:\n%s", view)
	}
	m = selectSideRow(t, m, "Catalog")
	if view := m.View(); !strings.Contains(view, "unavailable") {
		t.Errorf("catalog missing-file view must say unavailable:\n%s", view)
	}
	for _, key := range []string{"e", "a", "d", " "} {
		m = sendKeys(t, m, key)
		if m.overlay != OverlayNone {
			t.Errorf("missing-file key %q opened overlay %v", key, m.overlay)
		}
	}
}

func TestProvidersMalformedNoCrash(t *testing.T) {
	dir := t.TempDir()
	modelsPath := filepath.Join(dir, "models.json")
	storePath := filepath.Join(dir, "models-store.json")
	writeFixture(t, modelsPath, `{not json`)
	writeFixture(t, storePath, `{"acme": {"models": `)
	m := newProvidersModel(t, modelsPath, storePath)
	m = selectSideRow(t, m, "Connections")
	if view := m.View(); !strings.Contains(view, "unavailable") {
		t.Errorf("malformed connections view must say unavailable:\n%s", view)
	}
	m = selectSideRow(t, m, "Catalog")
	if view := m.View(); !strings.Contains(view, "unavailable") {
		t.Errorf("malformed catalog view must say unavailable:\n%s", view)
	}
}

func TestProvidersRereadReflectsChange(t *testing.T) {
	modelsPath, storePath := providersFixture(t)
	m := newProvidersModel(t, modelsPath, storePath)
	m = selectSideRow(t, m, "Connections")
	if len(m.providerRows) != 2 {
		t.Fatalf("first provider rows = %d, want 2", len(m.providerRows))
	}
	changed := `{"providers": {"solo": {"name": "Solo", "api": "openai-completions", "baseUrl": "https://solo.test", "models": [{"id": "only-1"}]}}}`
	if err := os.WriteFile(modelsPath, []byte(changed), 0o644); err != nil {
		t.Fatalf("rewrite models.json: %v", err)
	}
	m.refresh()
	if len(m.providerRows) != 1 {
		t.Fatalf("rebuilt provider rows = %d, want 1", len(m.providerRows))
	}
	if !strings.Contains(m.View(), "solo") {
		t.Errorf("rebuilt connections view missing solo:\n%s", m.View())
	}
	cchanged := `{"solo": {"models": [{"id": "only-1", "name": "Only 1"}], "checkedAt": "2026-10-06T02:00:00Z", "etag": "etag-solo"}}`
	if err := os.WriteFile(storePath, []byte(cchanged), 0o644); err != nil {
		t.Fatalf("rewrite models-store.json: %v", err)
	}
	m = selectSideRow(t, m, "Catalog")
	if len(m.catalogRows) != 1 {
		t.Fatalf("rebuilt catalog rows = %d, want 1", len(m.catalogRows))
	}
	if !strings.Contains(m.View(), "solo/only-1") {
		t.Errorf("rebuilt catalog view missing solo/only-1:\n%s", m.View())
	}
}
