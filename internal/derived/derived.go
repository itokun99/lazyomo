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

// CachePath resolves the MCP tools cache under an agent directory. Unlike
// models.json it lives in the cache subdirectory.
func CachePath(agentDir string) string {
	return filepath.Join(agentDir, "cache", "mcp-cache.json")
}

// Tool is one MCP tool entry from the cache: the display name and the
// description the pane lists. inputSchema and annotations are carried by
// the cache but not rendered, so they are not decoded here.
type Tool struct {
	Name        string
	Description string
}

// ToolsServer groups one server's cached tools with its freshness marker.
type ToolsServer struct {
	Name      string
	ToolCount int
	Tools     []Tool
	FetchedAt string
}

// ToolsSnapshot is the loaded mcp-cache.json view: servers sorted by name,
// the total tool count, and the source path plus file mtime.
type ToolsSnapshot struct {
	Servers     []ToolsServer
	Path        string
	RefreshedAt time.Time
	TotalTools  int
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
		// Defensive per-entry decode: one oddly typed field must not drop
		// the whole provider, so every field renders best-effort.
		entry := map[string]any{}
		_ = json.Unmarshal(entries[key], &entry)
		name := stringField(entry, "name")
		if name == "" {
			name = key
		}
		providers = append(providers, Provider{
			Key:        key,
			Name:       name,
			API:        stringField(entry, "api"),
			BaseURL:    stringField(entry, "baseUrl"),
			ModelCount: len(modelItems(entry["models"])),
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
		// Defensive per-entry decode: the live cache stores epoch numbers
		// for checkedAt/lastModified and arbitrary model shapes, so every
		// field renders best-effort and no single field drops the entry.
		entry := map[string]any{}
		_ = json.Unmarshal(entries[key], &entry)
		checked := timestampField(entry, "checkedAt")
		if checked == "" {
			checked = timestampField(entry, "lastModified")
		}
		models := make([]CatalogModel, 0)
		for _, item := range modelItems(entry["models"]) {
			id := stringField(item, "id")
			if id == "" {
				continue
			}
			name := stringField(item, "name")
			if name == "" {
				name = id
			}
			models = append(models, CatalogModel{
				ID:            id,
				Name:          name,
				Cost:          formatCost(item["cost"]),
				ContextWindow: parseContextWindow(item["contextWindow"]),
			})
		}
		sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
		providers = append(providers, CatalogProvider{
			Name:      key,
			Models:    models,
			CheckedAt: checked,
			ETag:      stringField(entry, "etag"),
		})
	}
	return providers, nil
}

// LoadTools reads the MCP tools cache at path with the same empty-state
// contract as LoadProviders.
func LoadTools(path string) (ToolsSnapshot, error) {
	if strings.TrimSpace(path) == "" {
		return ToolsSnapshot{}, fmt.Errorf("loading mcp-cache.json: path is empty")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ToolsSnapshot{}, fmt.Errorf("loading mcp-cache.json: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return ToolsSnapshot{}, fmt.Errorf("loading mcp-cache.json: %w", err)
	}
	servers, err := parseTools(raw)
	if err != nil {
		return ToolsSnapshot{}, err
	}
	total := 0
	for _, s := range servers {
		total += s.ToolCount
	}
	return ToolsSnapshot{Servers: servers, Path: path, RefreshedAt: info.ModTime(), TotalTools: total}, nil
}

func parseTools(data []byte) ([]ToolsServer, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, fmt.Errorf("parsing mcp-cache.json: empty document")
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("parsing mcp-cache.json: %w", err)
	}
	entries := top
	if raw, ok := top["servers"]; ok {
		var wrapped map[string]json.RawMessage
		if err := json.Unmarshal(raw, &wrapped); err != nil {
			return nil, fmt.Errorf("parsing mcp-cache.json: %w", err)
		}
		entries = wrapped
	}
	keys := make([]string, 0, len(entries))
	for k := range entries {
		if k == "servers" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	servers := make([]ToolsServer, 0, len(keys))
	for _, key := range keys {
		// Defensive per-entry decode, same contract as the catalog: the live
		// cache stores epoch numbers for fetchedAt and arbitrary tool
		// shapes, so every field renders best-effort, junk tool entries are
		// skipped, and no single entry drops the server.
		entry := map[string]any{}
		_ = json.Unmarshal(entries[key], &entry)
		tools := make([]Tool, 0)
		for _, item := range modelItems(entry["tools"]) {
			name := stringField(item, "name")
			if name == "" {
				continue
			}
			tools = append(tools, Tool{Name: name, Description: stringField(item, "description")})
		}
		servers = append(servers, ToolsServer{
			Name:      key,
			ToolCount: len(tools),
			Tools:     tools,
			FetchedAt: timestampField(entry, "fetchedAt"),
		})
	}
	return servers, nil
}

// modelItems returns the object entries of a models list, skipping anything
// shaped otherwise; a non-list yields no items but never an error, so the
// provider entry survives with best-effort models.
func modelItems(value any) []map[string]any {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// stringField renders one entry field best-effort: strings trimmed, integral
// numbers literally, booleans literally, objects and arrays as compact JSON;
// missing or null renders empty.
func stringField(entry map[string]any, key string) string {
	if entry == nil {
		return ""
	}
	return stringify(entry[key])
}

// timestampField renders a freshness marker the live cache stores either as
// an ISO string or as epoch milliseconds.
func timestampField(entry map[string]any, key string) string {
	if entry == nil {
		return ""
	}
	if s, ok := entry[key].(string); ok {
		return strings.TrimSpace(s)
	}
	if entry[key] == nil {
		return ""
	}
	return stringify(entry[key])
}

func stringify(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
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
