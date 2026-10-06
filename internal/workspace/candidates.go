package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/itokun99/lazyomo/internal/editor"
	"github.com/itokun99/lazyomo/internal/fuzzy"
)

type Candidate struct {
	Value    string
	Group    int
	Provider string
	Usages   []string
}

func (c Candidate) Fuzzy() fuzzy.Candidate {
	return fuzzy.NewCandidate(c.Value, c.Group)
}

func ToFuzzy(cands []Candidate) []fuzzy.Candidate {
	out := make([]fuzzy.Candidate, 0, len(cands))
	for _, c := range cands {
		out = append(out, c.Fuzzy())
	}
	return out
}

func CatalogCandidates(storePath, modelsPath string) []Candidate {
	var storeData, modelsData []byte
	if storePath != "" {
		if data, err := os.ReadFile(storePath); err == nil {
			storeData = data
		}
	}
	if modelsPath != "" {
		if data, err := os.ReadFile(modelsPath); err == nil {
			modelsData = data
		}
	}
	return catalogFromBytes(storeData, modelsData)
}

func DocumentCandidates(ed *editor.Editor) []Candidate {
	if ed == nil {
		return nil
	}
	aliases, aliasUsages, refUsages, refProviders := harvestDocument(ed)
	return dedupeAndSort(aliases, aliasUsages, refUsages, refProviders, nil)
}

func BuildCandidates(storePath, modelsPath string, ed *editor.Editor) []Candidate {
	catalog := CatalogCandidates(storePath, modelsPath)
	if ed == nil {
		return catalog
	}
	aliases, aliasUsages, refUsages, refProviders := harvestDocument(ed)
	return dedupeAndSort(aliases, aliasUsages, refUsages, refProviders, catalog)
}

func catalogFromBytes(storeData, modelsData []byte) []Candidate {
	var catalog []Candidate
	if refs, err := parseStoreData(storeData); err == nil {
		catalog = append(catalog, refs...)
	}
	if refs, err := parseModelsData(modelsData); err == nil {
		catalog = append(catalog, refs...)
	}
	return dedupeAndSort(nil, nil, nil, nil, catalog)
}

func parseStoreData(data []byte) ([]Candidate, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, fmt.Errorf("parsing models-store.json: empty document")
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("parsing models-store.json: %w", err)
	}
	providers := top
	if raw, ok := top["providers"]; ok {
		var wrapped map[string]json.RawMessage
		if err := json.Unmarshal(raw, &wrapped); err == nil {
			providers = wrapped
		}
	}
	keys := make([]string, 0, len(providers))
	for k := range providers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []Candidate
	for _, providerKey := range keys {
		if providerKey == "providers" {
			continue
		}
		var entry struct {
			Models []struct {
				ID       string `json:"id"`
				Provider string `json:"provider"`
			} `json:"models"`
		}
		if err := json.Unmarshal(providers[providerKey], &entry); err != nil {
			continue
		}
		for _, m := range entry.Models {
			id := stripSuffix(strings.TrimSpace(m.ID))
			if id == "" || strings.Contains(id, "/") {
				continue
			}
			provider := strings.TrimSpace(m.Provider)
			if provider == "" {
				provider = providerKey
			}
			if provider == "" || strings.Contains(provider, "/") || strings.Contains(provider, ":") {
				continue
			}
			out = append(out, Candidate{
				Value:    provider + "/" + id,
				Group:    fuzzy.GroupRef,
				Provider: provider,
			})
		}
	}
	return out, nil
}

func parseModelsData(data []byte) ([]Candidate, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, fmt.Errorf("parsing models.json: empty document")
	}
	var root struct {
		Providers map[string]struct {
			Models []struct {
				ID string `json:"id"`
			} `json:"models"`
		} `json:"providers"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parsing models.json: %w", err)
	}
	providers := root.Providers
	if len(providers) == 0 {
		var alt map[string]struct {
			Models []struct {
				ID string `json:"id"`
			} `json:"models"`
		}
		if err := json.Unmarshal(data, &alt); err == nil && len(alt) > 0 {
			providers = alt
		}
	}
	keys := make([]string, 0, len(providers))
	for k := range providers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []Candidate
	for _, provider := range keys {
		if provider == "" || strings.Contains(provider, "/") || strings.Contains(provider, ":") {
			continue
		}
		for _, m := range providers[provider].Models {
			id := stripSuffix(strings.TrimSpace(m.ID))
			if id == "" || strings.Contains(id, "/") {
				continue
			}
			out = append(out, Candidate{
				Value:    provider + "/" + id,
				Group:    fuzzy.GroupRef,
				Provider: provider,
			})
		}
	}
	return out, nil
}

func harvestDocument(ed *editor.Editor) (map[string]struct{}, map[string]map[string]struct{}, map[string]map[string]struct{}, map[string]string) {
	aliases := map[string]struct{}{}
	aliasUsages := map[string]map[string]struct{}{}
	refUsages := map[string]map[string]struct{}{}
	refProviders := map[string]string{}
	sections := ed.Sections()
	for _, section := range sections {
		if section.ID == editor.SectionModels {
			for _, entry := range section.Entries {
				if entry.Key == "" {
					continue
				}
				aliases[entry.Key] = struct{}{}
			}
		}
	}
	for alias := range aliases {
		aliasUsages[alias] = map[string]struct{}{}
	}
	addRef := func(raw, usage string) {
		stripped := stripSuffix(strings.TrimSpace(raw))
		if stripped == "" || !strings.Contains(stripped, "/") {
			return
		}
		provider := stripped[:strings.Index(stripped, "/")]
		if provider == "" || strings.Contains(provider, ":") {
			return
		}
		if _, ok := refUsages[stripped]; !ok {
			refUsages[stripped] = map[string]struct{}{}
			refProviders[stripped] = provider
		}
		refUsages[stripped][usage] = struct{}{}
	}
	addAliasUse := func(alias, usage string) {
		base := stripSuffix(strings.TrimSpace(alias))
		if base == "" {
			return
		}
		uses, ok := aliasUsages[base]
		if !ok {
			return
		}
		uses[usage] = struct{}{}
	}
	for _, section := range sections {
		switch section.ID {
		case editor.SectionModels:
			for _, entry := range section.Entries {
				detail, err := ed.Detail(section.ID, entry.Key)
				if err != nil {
					continue
				}
				for _, line := range detail.Lines {
					if line.Label != "model" {
						continue
					}
					if strings.TrimSpace(line.Value) == "" {
						continue
					}
					addRef(line.Value, line.Path)
				}
			}
		case editor.SectionAgents, editor.SectionCategories:
			for _, entry := range section.Entries {
				detail, err := ed.Detail(section.ID, entry.Key)
				if err != nil {
					continue
				}
				for _, line := range detail.Lines {
					switch line.Label {
					case "model":
						v := strings.TrimSpace(line.Value)
						if v == "" {
							continue
						}
						if strings.Contains(stripSuffix(v), "/") {
							addRef(v, line.Path)
						} else {
							addAliasUse(v, line.Path)
						}
					case "models":
						for i, element := range splitChain(line.Value) {
							base := stripSuffix(strings.TrimSpace(element))
							if base == "" {
								continue
							}
							ptr := line.Path + "/" + itoa(i)
							if strings.Contains(base, "/") {
								addRef(base, ptr)
							} else {
								addAliasUse(base, ptr)
							}
						}
					}
				}
			}
		case editor.SectionModelProfiles:
			for _, entry := range section.Entries {
				detail, err := ed.Detail(section.ID, entry.Key)
				if err != nil {
					continue
				}
				for _, line := range detail.Lines {
					if line.Label != "models" {
						continue
					}
					for i, element := range splitChain(line.Value) {
						base := stripSuffix(strings.TrimSpace(element))
						if base == "" {
							continue
						}
						ptr := line.Path + "/" + itoa(i)
						if strings.Contains(base, "/") {
							addRef(base, ptr)
						} else {
							addAliasUse(base, ptr)
						}
					}
				}
			}
		default:
			if string(section.ID) != "model_profile" {
				continue
			}
			for _, entry := range section.Entries {
				detail, err := ed.Detail(section.ID, entry.Key)
				if err != nil {
					continue
				}
				for _, line := range detail.Lines {
					if strings.TrimSpace(line.Value) == "" {
						continue
					}
					if strings.Contains(stripSuffix(strings.TrimSpace(line.Value)), "/") {
						addRef(line.Value, line.Path)
					}
				}
			}
		}
	}
	return aliases, aliasUsages, refUsages, refProviders
}

func dedupeAndSort(aliases map[string]struct{}, aliasUsages map[string]map[string]struct{}, refUsages map[string]map[string]struct{}, refProviders map[string]string, catalog []Candidate) []Candidate {
	merged := map[string]*Candidate{}
	remember := func(c Candidate) {
		if existing, ok := merged[c.Value]; ok {
			seen := map[string]struct{}{}
			for _, u := range existing.Usages {
				seen[u] = struct{}{}
			}
			for _, u := range c.Usages {
				if _, ok := seen[u]; !ok {
					existing.Usages = append(existing.Usages, u)
					seen[u] = struct{}{}
				}
			}
			return
		}
		cp := c
		cp.Usages = append([]string(nil), c.Usages...)
		merged[c.Value] = &cp
	}
	aliasNames := make([]string, 0, len(aliases))
	for name := range aliases {
		aliasNames = append(aliasNames, name)
	}
	sort.Strings(aliasNames)
	for _, name := range aliasNames {
		var usages []string
		for u := range aliasUsages[name] {
			usages = append(usages, u)
		}
		remember(Candidate{Value: name, Group: fuzzy.GroupAlias, Usages: usages})
	}
	for _, c := range catalog {
		if c.Value == "" {
			continue
		}
		group := c.Group
		if group != fuzzy.GroupAlias && group != fuzzy.GroupRef {
			group = fuzzy.GroupRef
		}
		remember(Candidate{Value: c.Value, Group: group, Provider: c.Provider, Usages: c.Usages})
	}
	for value, usages := range refUsages {
		provider := refProviders[value]
		if provider == "" && strings.Contains(value, "/") {
			provider = value[:strings.Index(value, "/")]
		}
		list := make([]string, 0, len(usages))
		for u := range usages {
			list = append(list, u)
		}
		remember(Candidate{Value: value, Group: fuzzy.GroupRef, Provider: provider, Usages: list})
	}
	var aliasList, refList []Candidate
	for _, c := range merged {
		if c.Group == fuzzy.GroupAlias {
			aliasList = append(aliasList, *c)
		} else {
			refList = append(refList, *c)
		}
	}
	sort.Slice(aliasList, func(i, j int) bool { return aliasList[i].Value < aliasList[j].Value })
	sort.Slice(refList, func(i, j int) bool {
		if refList[i].Provider != refList[j].Provider {
			return refList[i].Provider < refList[j].Provider
		}
		return refList[i].Value < refList[j].Value
	})
	for _, c := range merged {
		sort.Strings(c.Usages)
	}
	out := make([]Candidate, 0, len(aliasList)+len(refList))
	out = append(out, aliasList...)
	out = append(out, refList...)
	if len(out) == 0 {
		return nil
	}
	return out
}

func stripSuffix(s string) string {
	if i := strings.Index(s, ":"); i >= 0 {
		return s[:i]
	}
	return s
}

func splitChain(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var elems []string
		if err := json.Unmarshal([]byte(trimmed), &elems); err == nil {
			out := make([]string, 0, len(elems))
			for _, e := range elems {
				e = strings.TrimSpace(e)
				if e != "" {
					out = append(out, e)
				}
			}
			return out
		}
		var anys []any
		if err := json.Unmarshal([]byte(trimmed), &anys); err != nil {
			return nil
		}
		out := make([]string, 0, len(anys))
		for _, e := range anys {
			if s, ok := e.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					out = append(out, s)
				}
			}
		}
		return out
	}
	parts := strings.Split(trimmed, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}
