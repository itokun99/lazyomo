// Package derived reads the read-only provider and catalog caches the omo
// runtime maintains under the agent directory: models.json (connection
// providers) and models-store.json (cached model catalogs). Readers take
// explicit paths so tests use synthetic fixtures; the TUI resolves the real
// paths from mcpfile.ResolveAgentDir. Nothing here writes, and nothing here
// reads auth.json or touches the network.
package derived

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Provider is one connection entry from models.json: the display name, the
// API flavour, the base URL, and how many models the file lists for it.
type Provider struct {
	Key        string
	Name       string
	API        string
	BaseURL    string
	ModelCount int
}

// ProvidersSnapshot is the loaded models.json view: providers sorted by key
// plus the source path and the file mtime the panes render as refreshed-at.
type ProvidersSnapshot struct {
	Providers   []Provider
	Path        string
	RefreshedAt time.Time
}

// CatalogModel is one cached model entry: the stable id, the display name,
// and the cost/context-window facts the catalog carries when present.
type CatalogModel struct {
	ID            string
	Name          string
	Cost          string
	ContextWindow int
}

// CatalogProvider groups one provider's cached models with its freshness
// markers.
type CatalogProvider struct {
	Name      string
	Models    []CatalogModel
	CheckedAt string
	ETag      string
}

// CatalogSnapshot is the loaded models-store.json view: providers sorted by
// name, the total model count, and the source path plus file mtime.
type CatalogSnapshot struct {
	Providers   []CatalogProvider
	Path        string
	RefreshedAt time.Time
	TotalModels int
}

// Paths resolves the two cache files under an agent directory.
func Paths(agentDir string) (modelsPath, storePath string) {
	return filepath.Join(agentDir, "models.json"), filepath.Join(agentDir, "models-store.json")
}

// LoadProviders reads the providers file at path. A missing, unreadable, or
// malformed file returns an error and no snapshot; callers render the
// empty state.
func LoadProviders(path string) (ProvidersSnapshot, error) {
	if strings.TrimSpace(path) == "" {
		return ProvidersSnapshot{}, fmt.Errorf("loading models.json: path is empty")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ProvidersSnapshot{}, fmt.Errorf("loading models.json: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return ProvidersSnapshot{}, fmt.Errorf("loading models.json: %w", err)
	}
	providers, err := parseProviders(raw)
	if err != nil {
		return ProvidersSnapshot{}, err
	}
	return ProvidersSnapshot{Providers: providers, Path: path, RefreshedAt: info.ModTime()}, nil
}

// LoadCatalog reads the catalog cache at path with the same empty-state
// contract as LoadProviders.
func LoadCatalog(path string) (CatalogSnapshot, error) {
	if strings.TrimSpace(path) == "" {
		return CatalogSnapshot{}, fmt.Errorf("loading models-store.json: path is empty")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return CatalogSnapshot{}, fmt.Errorf("loading models-store.json: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return CatalogSnapshot{}, fmt.Errorf("loading models-store.json: %w", err)
	}
	providers, err := parseCatalog(raw)
	if err != nil {
		return CatalogSnapshot{}, err
	}
	total := 0
	for _, p := range providers {
		total += len(p.Models)
	}
	return CatalogSnapshot{Providers: providers, Path: path, RefreshedAt: info.ModTime(), TotalModels: total}, nil
}

func parseProviders(data []byte) ([]Provider, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, fmt.Errorf("parsing models.json: empty document")
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("parsing models.json: %w", err)
	}
	entries := top
	if raw, ok := top["providers"]; ok {
		var wrapped map[string]json.RawMessage
		if err := json.Unmarshal(raw, &wrapped); err != nil {
			return nil, fmt.Errorf("parsing models.json: %w", err)
		}
		entries = wrapped
	}
	keys := make([]string, 0, len(entries))
	for k := range entries {
		if k == "providers" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	providers := make([]Provider, 0, len(keys))
	for _, key := range keys {
		var entry struct {
			Name    string `json:"name"`
			API     string `json:"api"`
			BaseURL string `json:"baseUrl"`
			Models  []struct {
				ID string `json:"id"`
			} `json:"models"`
		}
		if err := json.Unmarshal(entries[key], &entry); err != nil {
			continue
		}
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			name = key
		}
		providers = append(providers, Provider{
			Key:        key,
			Name:       name,
			API:        strings.TrimSpace(entry.API),
			BaseURL:    strings.TrimSpace(entry.BaseURL),
			ModelCount: len(entry.Models),
		})
	}
	return providers, nil
}

func parseCatalog(data []byte) ([]CatalogProvider, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, fmt.Errorf("parsing models-store.json: empty document")
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("parsing models-store.json: %w", err)
	}
	entries := top
	if raw, ok := top["providers"]; ok {
		var wrapped map[string]json.RawMessage
		if err := json.Unmarshal(raw, &wrapped); err != nil {
			return nil, fmt.Errorf("parsing models-store.json: %w", err)
		}
		entries = wrapped
	}
	keys := make([]string, 0, len(entries))
	for k := range entries {
		if k == "providers" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	providers := make([]CatalogProvider, 0, len(keys))
	for _, key := range keys {
		var entry struct {
			Models       []map[string]any `json:"models"`
			CheckedAt    string           `json:"checkedAt"`
			LastModified string           `json:"lastModified"`
			ETag         string           `json:"etag"`
		}
		if err := json.Unmarshal(entries[key], &entry); err != nil {
			continue
		}
		checked := strings.TrimSpace(entry.CheckedAt)
		if checked == "" {
			checked = strings.TrimSpace(entry.LastModified)
		}
		models := make([]CatalogModel, 0, len(entry.Models))
		for _, raw := range entry.Models {
			id, _ := raw["id"].(string)
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			name, _ := raw["name"].(string)
			name = strings.TrimSpace(name)
			if name == "" {
				name = id
			}
			models = append(models, CatalogModel{
				ID:            id,
				Name:          name,
				Cost:          formatCost(raw["cost"]),
				ContextWindow: parseContextWindow(raw["contextWindow"]),
			})
		}
		sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
		providers = append(providers, CatalogProvider{
			Name:      key,
			Models:    models,
			CheckedAt: checked,
			ETag:      strings.TrimSpace(entry.ETag),
		})
	}
	return providers, nil
}

func formatCost(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		return string(data)
	}
}

func parseContextWindow(value any) int {
	switch v := value.(type) {
	case float64:
		if v < 0 {
			return 0
		}
		return int(v)
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0
		}
		if n, err := strconv.Atoi(trimmed); err == nil && n >= 0 {
			return n
		}
		if f, err := strconv.ParseFloat(trimmed, 64); err == nil && f >= 0 {
			return int(f)
		}
		return 0
	case int:
		if v < 0 {
			return 0
		}
		return v
	default:
		return 0
	}
}
