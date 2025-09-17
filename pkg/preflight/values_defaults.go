package preflight

import (
	"regexp"
	"strings"
)

var (
	// Matches occurrences like .Values.minio.enabled or .Values.postgres.create
	// Captures group 1 as the dotted path (e.g., "minio" or "postgres");
	// group 2 is the leaf key (enabled|create).
	valuesBoolRefRe = regexp.MustCompile(`\.Values\.([A-Za-z0-9_\.]+?)\.(enabled|create)\b`)
	// Matches general .Values.<path> references used in templates. This is a broad match
	// and will be further sanitized before being applied.
	valuesAnyRefRe = regexp.MustCompile(`\.Values\.([A-Za-z0-9_\.]+)`)
)

// SeedDefaultBooleans scans the template content for boolean-like value references
// such as .Values.<path>.enabled or .Values.<path>.create and ensures that any
// missing paths in the provided values map are initialized with a default value
// of false. This prevents nil dereference errors during Helm rendering when
// templates access nested fields on absent maps.
func SeedDefaultBooleans(templateContent string, values map[string]interface{}) {
	if values == nil {
		return
	}

	matches := valuesBoolRefRe.FindAllStringSubmatch(templateContent, -1)
	if len(matches) == 0 {
		return
	}

	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		dottedPath := m[1]
		leaf := m[2]
		// Build full key path like [minio, enabled]
		keys := append(strings.Split(dottedPath, "."), leaf)
		setNestedDefaultFalse(values, keys)
	}
}

// SeedParentMapsForValueRefs scans the template for .Values.<path> references and ensures
// that any missing parent maps along those paths are created in the provided values map.
// This prevents Helm/template evaluation errors like "nil pointer evaluating interface {}.foo"
// when the template dereferences nested maps that are absent. Only parent maps up to the
// last segment are created; leaf values are never set.
func SeedParentMapsForValueRefs(templateContent string, values map[string]interface{}) {
	if values == nil {
		return
	}

	matches := valuesAnyRefRe.FindAllStringSubmatch(templateContent, -1)
	if len(matches) == 0 {
		return
	}

	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		dotted := m[1]
		// Ignore obviously invalid or empty
		if dotted == "" {
			continue
		}
		// Split path into segments; keep only identifier segments
		rawSegs := strings.Split(dotted, ".")
		segs := make([]string, 0, len(rawSegs))
		for _, s := range rawSegs {
			if s == "" {
				continue
			}
			// Only allow [A-Za-z0-9_]
			valid := true
			for i := 0; i < len(s); i++ {
				ch := s[i]
				if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_') {
					valid = false
					break
				}
			}
			if !valid {
				break
			}
			segs = append(segs, s)
		}
		// Need at least two segments to create parents (e.g., foo.bar)
		if len(segs) < 2 {
			continue
		}
		ensureParentMaps(values, segs)
	}
}

// ensureParentMaps ensures that for the given dotted path segments, all parent maps
// (up to but not including the last segment) exist. If an existing value conflicts
// (not a map), it is replaced with a new map to allow nested lookups downstream.
func ensureParentMaps(root map[string]interface{}, segs []string) {
	cur := root
	// up to segs[len-2]
	for i := 0; i < len(segs)-1; i++ {
		k := segs[i]
		next, ok := cur[k]
		if ok {
			if m, ok := next.(map[string]interface{}); ok {
				cur = m
				continue
			}
		}
		// create/replace with a map
		m := map[string]interface{}{}
		cur[k] = m
		cur = m
	}
}

// setNestedDefaultFalse ensures that the nested path exists. If the leaf key is
// missing, it sets it to false. Existing values are left unchanged.
func setNestedDefaultFalse(root map[string]interface{}, keys []string) {
	if len(keys) == 0 {
		return
	}
	cur := root
	// Traverse/create intermediate maps for all but the last key
	for i := 0; i < len(keys)-1; i++ {
		k := keys[i]
		next, ok := cur[k]
		if !ok {
			m := map[string]interface{}{}
			cur[k] = m
			cur = m
			continue
		}
		if m, ok := next.(map[string]interface{}); ok {
			cur = m
			continue
		}
		// If the existing value is not a map, replace it with a nested map to allow
		// setting the boolean leaf without panicking.
		m := map[string]interface{}{}
		cur[k] = m
		cur = m
	}
	leafKey := keys[len(keys)-1]
	if _, exists := cur[leafKey]; !exists {
		cur[leafKey] = false
	}
}
