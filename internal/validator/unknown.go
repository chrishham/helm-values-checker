package validator

import (
	"fmt"
	"strings"

	"github.com/agnivade/levenshtein"
	"github.com/chrishham/helm-values-checker/internal/model"
	"gopkg.in/yaml.v3"
)

// collectAllPaths walks a yaml mapping tree and collects all dot-separated
// key paths. Used to suggest relocated keys.
func collectAllPaths(node *yaml.Node, prefix string) map[string]string {
	paths := make(map[string]string)
	if node == nil || node.Kind != yaml.MappingNode {
		return paths
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i].Value
		fullPath := joinPath(prefix, key)
		paths[fullPath] = key
		if node.Content[i+1].Kind == yaml.MappingNode {
			for p, leaf := range collectAllPaths(node.Content[i+1], fullPath) {
				paths[p] = leaf
			}
		}
	}
	return paths
}

// findDeepSuggestion searches the entire defaults tree for a key path that
// matches the unknown key's leaf name. It uses three strategies in priority order:
//  1. Exact leaf name at a different path (relocated key)
//  2. Close Levenshtein match (distance < 4, same as sibling matching)
//  3. Substring containment where the added/removed portion is short
//     (e.g., orgCreationDisabled → userOrgCreationDisabled)
//
// Returns the full path of the best match, or empty string if none found.
func findDeepSuggestion(unknownPath string, allPaths map[string]string) string {
	parts := strings.Split(unknownPath, ".")
	leaf := strings.ToLower(parts[len(parts)-1])

	// Track best candidates per strategy (higher priority wins)
	var exactMatch string
	levenBest := ""
	levenBestDist := 4 // threshold: must be < 4
	containBest := ""
	containBestDiff := 1000

	for path, pathLeaf := range allPaths {
		if path == unknownPath {
			continue
		}
		lowerPathLeaf := strings.ToLower(pathLeaf)

		// Strategy 1: exact leaf match at different location
		if leaf == lowerPathLeaf {
			if exactMatch == "" || len(path) < len(exactMatch) {
				exactMatch = path
			}
			continue
		}

		// Strategy 2: close Levenshtein match
		dist := levenshtein.ComputeDistance(leaf, lowerPathLeaf)
		if dist < levenBestDist {
			levenBestDist = dist
			levenBest = path
		}

		// Strategy 3: substring containment with short diff
		if strings.Contains(lowerPathLeaf, leaf) || strings.Contains(leaf, lowerPathLeaf) {
			shorter := len(leaf)
			if len(lowerPathLeaf) < shorter {
				shorter = len(lowerPathLeaf)
			}
			diff := len(lowerPathLeaf) - len(leaf)
			if diff < 0 {
				diff = -diff
			}
			// Only suggest if added/removed portion is at most half the shorter name
			if diff <= shorter/2 && diff < containBestDiff {
				containBestDiff = diff
				containBest = path
			}
		}
	}

	// Return best match by priority
	if exactMatch != "" {
		return exactMatch
	}
	if levenBest != "" {
		return levenBest
	}
	if containBest != "" {
		return containBest
	}
	return ""
}

// detectUnknownKeys walks the user values tree and reports keys not found
// in the chart defaults tree. allPaths is a pre-computed map of every
// dot-separated path in the root defaults tree (used for deep suggestions).
func detectUnknownKeys(userNode, defaultsNode *yaml.Node, schemaKeys map[string]bool, subchartDefaults map[string]*yaml.Node, ignoreKeys []string, path string, allPaths map[string]string) []model.Finding {
	var findings []model.Finding

	if userNode == nil || defaultsNode == nil {
		return findings
	}

	// Both must be mappings to compare keys
	if userNode.Kind != yaml.MappingNode {
		return findings
	}

	// Build set of known keys from defaults
	defaultKeys := mappingKeys(defaultsNode)

	// Iterate user keys (mapping nodes have alternating key/value in Content)
	for i := 0; i+1 < len(userNode.Content); i += 2 {
		keyNode := userNode.Content[i]
		valNode := userNode.Content[i+1]
		key := keyNode.Value
		fullPath := joinPath(path, key)

		// Check ignore patterns
		if matchesIgnore(fullPath, ignoreKeys) {
			continue
		}

		// Check if key is a subchart name — validate against subchart defaults
		if subDefaults, ok := subchartDefaults[key]; ok {
			if valNode.Kind == yaml.MappingNode {
				findings = append(findings, detectUnknownKeys(valNode, subDefaults, nil, nil, ignoreKeys, fullPath, allPaths)...)
			}
			continue
		}

		// Check if key exists in defaults
		if _, ok := defaultKeys[key]; !ok {
			// Also check schema-defined keys
			if schemaKeys != nil && schemaKeys[fullPath] {
				// Key is valid per schema, continue checking children
				if valNode.Kind == yaml.MappingNode {
					findings = append(findings, detectUnknownKeys(valNode, &yaml.Node{Kind: yaml.MappingNode}, schemaKeys, subchartDefaults, ignoreKeys, fullPath, allPaths)...)
				}
				continue
			}

			f := model.Finding{
				Severity: model.SeverityError,
				Line:     keyNode.Line,
				KeyPath:  fullPath,
				Message:  fmt.Sprintf("Unknown key %q", fullPath),
			}

			// Find closest match: first try siblings, then deep search
			if suggestion := findClosestKey(key, defaultKeys); suggestion != "" {
				f.Suggestion = joinPath(path, suggestion)
			} else if allPaths != nil {
				if suggestion := findDeepSuggestion(fullPath, allPaths); suggestion != "" {
					f.Suggestion = suggestion
				}
			}

			findings = append(findings, f)
			continue
		}

		// Key exists — recurse into children if both are mappings
		defaultVal := getValueForKey(defaultsNode, key)
		if valNode.Kind == yaml.MappingNode && defaultVal != nil && defaultVal.Kind == yaml.MappingNode {
			// Empty mapping default means "accept any structure" (e.g., podSecurityContext: {})
			if len(defaultVal.Content) == 0 {
				continue
			}
			findings = append(findings, detectUnknownKeys(valNode, defaultVal, schemaKeys, subchartDefaults, ignoreKeys, fullPath, allPaths)...)
		}
	}

	return findings
}

// mappingKeys extracts all keys from a yaml mapping node.
func mappingKeys(node *yaml.Node) map[string]bool {
	keys := make(map[string]bool)
	if node == nil || node.Kind != yaml.MappingNode {
		return keys
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		keys[node.Content[i].Value] = true
	}
	return keys
}

// getValueForKey returns the value node for a given key in a mapping node.
func getValueForKey(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			val := node.Content[i+1]
			// Resolve aliases
			if val.Kind == yaml.AliasNode && val.Alias != nil {
				return val.Alias
			}
			return val
		}
	}
	return nil
}

// findClosestKey returns the closest matching key using Levenshtein distance.
// Returns empty string if no close match found (threshold: distance <= 3).
func findClosestKey(key string, candidates map[string]bool) string {
	best := ""
	bestDist := 4 // threshold

	for candidate := range candidates {
		dist := levenshtein.ComputeDistance(strings.ToLower(key), strings.ToLower(candidate))
		if dist < bestDist {
			bestDist = dist
			best = candidate
		}
	}
	return best
}

func joinPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

func matchesIgnore(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchGlob(pattern, path) {
			return true
		}
	}
	return false
}

// matchGlob matches a simple glob pattern against a path.
// Supports * (matches one path segment) and ** (matches any number of segments).
func matchGlob(pattern, path string) bool {
	// Simple implementation covering the most useful cases
	if pattern == path {
		return true
	}

	patParts := strings.Split(pattern, ".")
	pathParts := strings.Split(path, ".")

	return matchGlobParts(patParts, pathParts)
}

func matchGlobParts(pattern, path []string) bool {
	pi, pa := 0, 0
	for pi < len(pattern) && pa < len(path) {
		if pattern[pi] == "**" {
			// ** matches zero or more segments
			if pi == len(pattern)-1 {
				return true
			}
			// Try matching remaining pattern against each suffix of path
			for k := pa; k <= len(path); k++ {
				if matchGlobParts(pattern[pi+1:], path[k:]) {
					return true
				}
			}
			return false
		}
		if pattern[pi] == "*" || pattern[pi] == path[pa] {
			pi++
			pa++
			continue
		}
		return false
	}
	return pi == len(pattern) && pa == len(path)
}
