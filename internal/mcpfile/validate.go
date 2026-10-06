package mcpfile

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
)

// FieldKind describes the JSON shape accepted for one server field.
type FieldKind int

// Field kinds of the runtime ServerSchema.
const (
	FieldString FieldKind = iota
	FieldBool
	FieldBoolOrStrings
	FieldNumber
	FieldStrings
	FieldStringMap
	FieldObject
	FieldStringOrFalse
)

// FieldSpec describes one known server field. Suggestions lists the values
// the runtime documents for enum-like fields; other values are still
// accepted so schema drift never blocks an edit.
type FieldSpec struct {
	Name        string
	Kind        FieldKind
	Suggestions []string
}

// serverFields lists the runtime ServerSchema fields in schema order.
var serverFields = []FieldSpec{
	{Name: "type", Kind: FieldString, Suggestions: []string{"stdio", "http"}},
	{Name: "url", Kind: FieldString},
	{Name: "command", Kind: FieldString},
	{Name: "args", Kind: FieldStrings},
	{Name: "env", Kind: FieldStringMap},
	{Name: "cwd", Kind: FieldString},
	{Name: "headers", Kind: FieldStringMap},
	{Name: "auth", Kind: FieldStringOrFalse, Suggestions: []string{"bearer", "oauth"}},
	{Name: "bearerTokenEnv", Kind: FieldString},
	{Name: "oauth", Kind: FieldObject},
	{Name: "enabled", Kind: FieldBool},
	{Name: "lifecycle", Kind: FieldString, Suggestions: []string{"lazy", "eager", "keep-alive"}},
	{Name: "idleTimeoutMin", Kind: FieldNumber},
	{Name: "requestTimeoutMs", Kind: FieldNumber},
	{Name: "connectTimeoutMs", Kind: FieldNumber},
	{Name: "startupTimeoutMs", Kind: FieldNumber},
	{Name: "includeTools", Kind: FieldStrings},
	{Name: "excludeTools", Kind: FieldStrings},
	{Name: "directTools", Kind: FieldBoolOrStrings},
	{Name: "exposure", Kind: FieldString, Suggestions: []string{"auto", "direct", "search", "proxy"}},
	{Name: "logLevel", Kind: FieldString, Suggestions: []string{"debug", "info", "notice", "warning", "error", "critical", "alert", "emergency"}},
}

// Fields returns copies of the known server field specs in schema order.
func Fields() []FieldSpec {
	specs := make([]FieldSpec, len(serverFields))
	for i, spec := range serverFields {
		specs[i] = FieldSpec{Name: spec.Name, Kind: spec.Kind, Suggestions: append([]string(nil), spec.Suggestions...)}
	}
	return specs
}

func fieldSpec(name string) (FieldSpec, bool) {
	for _, spec := range serverFields {
		if spec.Name == name {
			return spec, true
		}
	}
	return FieldSpec{}, false
}

// fieldError prefixes a value error with its config location.
func fieldError(server, field string, err error) error {
	return fmt.Errorf("mcpServers.%s.%s: %w", server, field, err)
}

// endpointViolation mirrors the runtime's endpoint rule and its exact
// message text: an enabled server needs a non-blank command (stdio) or url
// (http). Disabled servers are exempt.
func endpointViolation(name string, server map[string]any) error {
	if enabled, ok := server["enabled"].(bool); ok && !enabled {
		return nil
	}
	if effectiveType(server) == "http" {
		if url, ok := server["url"].(string); !ok || strings.TrimSpace(url) == "" {
			return fmt.Errorf("mcpServers.%s.url: Required for enabled http server", name)
		}
		return nil
	}
	if command, ok := server["command"].(string); !ok || strings.TrimSpace(command) == "" {
		return fmt.Errorf("mcpServers.%s.command: Required for enabled stdio server", name)
	}
	return nil
}

// checkInterpolation rejects the two string forms the runtime refuses to
// interpret: a leading "!" (shell execution) and "$(" (command
// substitution). ${VAR} and ${VAR:-default} stay untouched. The "!"
// check trims leading whitespace first, mirroring senpi's
// value.trimStart().startsWith("!"); the "$(" check is a plain
// substring match in senpi (value.includes("$(")), so it stays
// trim-insensitive here too.
func checkInterpolation(value string) error {
	if strings.HasPrefix(strings.TrimLeftFunc(value, unicode.IsSpace), "!") || strings.Contains(value, "$(") {
		return fmt.Errorf(`values starting with "!" or containing "$(" are rejected`)
	}
	return nil
}

func checkInterpolationAll(values []string) error {
	for _, value := range values {
		if err := checkInterpolation(value); err != nil {
			return err
		}
	}
	return nil
}

// checkInterpolationValue walks nested JSON values so interpolation limits
// apply to every string an edit introduces, in a deterministic order.
func checkInterpolationValue(value any) error {
	switch v := value.(type) {
	case string:
		return checkInterpolation(v)
	case []any:
		for _, item := range v {
			if err := checkInterpolationValue(item); err != nil {
				return err
			}
		}
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if err := checkInterpolationValue(v[key]); err != nil {
				return err
			}
		}
	}
	return nil
}

// normalizeValue type-checks value against the field's kind and returns the
// canonical JSON form to store.
func normalizeValue(spec FieldSpec, name string, value any) (any, error) {
	fail := func(want string) error {
		return fmt.Errorf("mcpServers.%s.%s: expected %s", name, spec.Name, want)
	}
	switch spec.Kind {
	case FieldString:
		s, ok := value.(string)
		if !ok {
			return nil, fail("string")
		}
		if err := checkInterpolation(s); err != nil {
			return nil, fieldError(name, spec.Name, err)
		}
		if spec.Name == "type" && s != "stdio" && s != "http" {
			return nil, fmt.Errorf("mcpServers.%s.type: want %q or %q, got %q", name, "stdio", "http", s)
		}
		return s, nil
	case FieldBool:
		b, ok := value.(bool)
		if !ok {
			return nil, fail("boolean")
		}
		return b, nil
	case FieldBoolOrStrings:
		if b, ok := value.(bool); ok {
			return b, nil
		}
		items, err := stringSlice(value)
		if err != nil {
			return nil, fail("boolean or array of strings")
		}
		if err := checkInterpolationAll(items); err != nil {
			return nil, fieldError(name, spec.Name, err)
		}
		return items, nil
	case FieldNumber:
		switch v := value.(type) {
		case int:
			return v, nil
		case int64:
			return v, nil
		case float64:
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, fail("number")
			}
			return v, nil
		default:
			return nil, fail("number")
		}
	case FieldStrings:
		items, err := stringSlice(value)
		if err != nil {
			return nil, fail("array of strings")
		}
		if err := checkInterpolationAll(items); err != nil {
			return nil, fieldError(name, spec.Name, err)
		}
		return items, nil
	case FieldStringMap:
		m, err := stringMap(value)
		if err != nil {
			return nil, fail("string map")
		}
		keys := make([]string, 0, len(m))
		for key := range m {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if err := checkInterpolation(m[key]); err != nil {
				return nil, fieldError(name, spec.Name, err)
			}
		}
		return m, nil
	case FieldObject:
		switch v := value.(type) {
		case map[string]any:
			if err := checkInterpolationValue(v); err != nil {
				return nil, fieldError(name, spec.Name, err)
			}
			return v, nil
		case map[string]string:
			converted := make(map[string]any, len(v))
			for key, item := range v {
				if err := checkInterpolation(item); err != nil {
					return nil, fieldError(name, spec.Name, err)
				}
				converted[key] = item
			}
			return converted, nil
		default:
			return nil, fail("object")
		}
	case FieldStringOrFalse:
		switch v := value.(type) {
		case string:
			if err := checkInterpolation(v); err != nil {
				return nil, fieldError(name, spec.Name, err)
			}
			return v, nil
		case bool:
			if v {
				return nil, fail("string or false")
			}
			return false, nil
		default:
			return nil, fail("string or false")
		}
	}
	return nil, fail("value")
}

func stringSlice(value any) ([]string, error) {
	switch v := value.(type) {
	case []string:
		return append([]string(nil), v...), nil
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("element is not a string")
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("value is not an array of strings")
	}
}

func stringMap(value any) (map[string]string, error) {
	switch v := value.(type) {
	case map[string]string:
		out := make(map[string]string, len(v))
		for key, item := range v {
			out[key] = item
		}
		return out, nil
	case map[string]any:
		out := make(map[string]string, len(v))
		for key, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("value for %q is not a string", key)
			}
			out[key] = s
		}
		return out, nil
	default:
		return nil, fmt.Errorf("value is not a string map")
	}
}

// cloneServer copies an entry for prospective-state checks so they cannot
// mutate the decoded view owned by the document.
func cloneServer(raw map[string]any) map[string]any {
	out := make(map[string]any, len(raw))
	for key, value := range raw {
		out[key] = value
	}
	return out
}
