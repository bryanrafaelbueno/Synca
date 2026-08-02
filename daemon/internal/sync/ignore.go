package sync

import (
	"path/filepath"
	"strings"
)

// IgnoreRules is an immutable set of normalized folder-name ignore
// patterns. Construction is side-effect free; a rule value can be shared.
type IgnoreRules struct {
	patterns []string
}

// NewIgnoreRules builds normalized rules from raw patterns. Whitespace
// and surrounding separators are trimmed, empty and "." patterns are
// dropped, and duplicates collapse, mirroring config normalization.
func NewIgnoreRules(patterns []string) IgnoreRules {
	normalized := make([]string, 0, len(patterns))
	seen := make(map[string]struct{}, len(patterns))
	for _, raw := range patterns {
		pattern := strings.TrimSpace(raw)
		pattern = strings.Trim(pattern, `/\`)
		if pattern == "" {
			continue
		}
		pattern = filepath.Clean(pattern)
		if pattern == "." {
			continue
		}
		if _, ok := seen[pattern]; ok {
			continue
		}
		seen[pattern] = struct{}{}
		normalized = append(normalized, pattern)
	}
	return IgnoreRules{patterns: normalized}
}

// Empty reports whether the rule set contains no patterns.
func (r IgnoreRules) Empty() bool {
	return len(r.patterns) == 0
}

// Matches reports whether the root-relative path is ignored: any path
// component equals a pattern, or the path starts at a pattern component
// boundary. Patterns may contain sub-paths such as "a/b"; sub-path
// patterns match only from the start of the relative path.
func (r IgnoreRules) Matches(rel string) bool {
	rel = filepath.Clean(rel)
	if rel == "." || len(r.patterns) == 0 {
		return false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	for _, pattern := range r.patterns {
		for _, part := range parts {
			if part == pattern {
				return true
			}
		}
		if rel == pattern || strings.HasPrefix(rel, pattern+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
