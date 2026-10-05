package editor

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	maxAliasLength       = 32
	maxDisplayNameLength = 64
)

// aliasPattern is the key rule from spec section 1: ^[a-zA-Z0-9_-]+$.
var aliasPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// modelRefPattern matches <provider>/<model-id> with an optional :suffix,
// for example github-copilot/claude-sonnet-5 or adacode/gpt-5-3:max.
var modelRefPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*(/[A-Za-z0-9][A-Za-z0-9._-]*)+(:[A-Za-z0-9][A-Za-z0-9._-]*)?$`)

// chainSuffixPattern matches the optional :suffix of a chain element.
var chainSuffixPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// reasoningValues is the reasoning enum from spec section 1.
var reasoningValues = map[string]struct{}{
	"low":    {},
	"medium": {},
	"high":   {},
	"max":    {},
}

// knownCategories is the fixed set of ten category names ("Unknown
// category names are rejected, the set is fixed at 10").
var knownCategories = map[string]struct{}{
	"architect":          {},
	"artistry":           {},
	"deep-high":          {},
	"deep-low":           {},
	"quick":              {},
	"ultrabrain":         {},
	"unspecified-high":   {},
	"unspecified-low":    {},
	"visual-engineering": {},
	"writing":            {},
}

// validateKey enforces the alias key rules: ^[a-zA-Z0-9_-]+$, at most 32
// characters, unique within its section (the caller checks uniqueness).
func validateKey(key string) error {
	if len(key) == 0 || len(key) > maxAliasLength || !aliasPattern.MatchString(key) {
		return fmt.Errorf("invalid key %q: want 1-%d characters matching %s", key, maxAliasLength, aliasPattern.String())
	}
	return nil
}

func validReasoning(value string) bool {
	_, ok := reasoningValues[value]
	return ok
}

func validModelRef(value string) bool {
	return modelRefPattern.MatchString(value)
}

// validChainElement accepts a full <provider>/<model-id>[:suffix] reference
// or an alias-style <alias>[:suffix] reference: chains may name either, and
// alias deletes are blocked while a chain still names the alias.
func validChainElement(value string) bool {
	if validModelRef(value) {
		return true
	}
	core, suffix, hasSuffix := strings.Cut(value, ":")
	if hasSuffix && !chainSuffixPattern.MatchString(suffix) {
		return false
	}
	return len(core) <= maxAliasLength && aliasPattern.MatchString(core)
}

func validateDisplayName(value string) error {
	if value == "" {
		return fmt.Errorf("invalid display_name: must not be empty")
	}
	if utf8.RuneCountInString(value) > maxDisplayNameLength {
		return fmt.Errorf("invalid display_name %q: at most %d characters", value, maxDisplayNameLength)
	}
	return nil
}

// parseChain parses the string form of a chain: either a JSON array of
// strings or a comma-separated list. Elements are validated and kept
// verbatim in order (no deduplication).
func parseChain(value string) ([]string, error) {
	trimmed := strings.TrimSpace(value)
	var elements []string
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal([]byte(trimmed), &elements); err != nil {
			return nil, fmt.Errorf("invalid chain %q: %w", value, err)
		}
	} else {
		if trimmed == "" {
			return nil, fmt.Errorf("invalid chain: must contain at least one <provider>/<model-id>[:suffix] or <alias>[:suffix] entry")
		}
		parts := strings.Split(trimmed, ",")
		elements = make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				return nil, fmt.Errorf("invalid chain %q: empty entry", value)
			}
			elements = append(elements, part)
		}
	}
	if len(elements) == 0 {
		return nil, fmt.Errorf("invalid chain: must contain at least one <provider>/<model-id>[:suffix] or <alias>[:suffix] entry")
	}
	for i, element := range elements {
		if !validChainElement(element) {
			return nil, fmt.Errorf("invalid chain entry %q at index %d: want <provider>/<model-id>[:suffix] or <alias>[:suffix]", element, i)
		}
	}
	return elements, nil
}
