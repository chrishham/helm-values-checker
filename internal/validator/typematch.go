package validator

import (
	"fmt"
	"strings"

	"github.com/chrishham/helm-values-checker/internal/model"
	"gopkg.in/yaml.v3"
)

// jsonSchemaTypeToYAMLTags maps a JSON Schema type name to the set of
// compatible YAML short tags.
func jsonSchemaTypeToYAMLTags(jsonType string) []string {
	switch jsonType {
	case "string":
		return []string{"!!str"}
	case "integer":
		return []string{"!!int"}
	case "number":
		return []string{"!!int", "!!float"}
	case "boolean":
		return []string{"!!bool"}
	case "null":
		return []string{"!!null"}
	case "array":
		return []string{"!!seq"}
	case "object":
		return []string{"!!map"}
	default:
		return nil
	}
}

// schemaTypesCompatible checks if a YAML tag is compatible with any of the
// allowed JSON Schema types. Returns whether compatible and the full set
// of allowed YAML tags (for error messages).
func schemaTypesCompatible(userTag string, schemaTypes []string) (bool, []string) {
	var allowedTags []string
	for _, st := range schemaTypes {
		allowedTags = append(allowedTags, jsonSchemaTypeToYAMLTags(st)...)
	}

	for _, tag := range allowedTags {
		if typesCompatible(userTag, tag) {
			return true, allowedTags
		}
	}
	return false, allowedTags
}

// friendlyTypes formats a slice of YAML tags as a human-readable string
// (e.g., "int, float, or null").
func friendlyTypes(tags []string) string {
	// Deduplicate
	seen := make(map[string]bool)
	var unique []string
	for _, t := range tags {
		ft := friendlyType(t)
		if !seen[ft] {
			seen[ft] = true
			unique = append(unique, ft)
		}
	}

	switch len(unique) {
	case 0:
		return "unknown"
	case 1:
		return unique[0]
	case 2:
		return unique[0] + " or " + unique[1]
	default:
		return strings.Join(unique[:len(unique)-1], ", ") + ", or " + unique[len(unique)-1]
	}
}

// detectTypeMismatches walks matching keys between user and default trees
// and reports type mismatches. When schemaTypes is non-nil, it is used as
// a fallback for keys whose default is null or absent.
func detectTypeMismatches(userNode, defaultsNode *yaml.Node, ignoreKeys []string, path string, schemaTypes SchemaTypeMap) []model.Finding {
	var findings []model.Finding

	if userNode == nil || defaultsNode == nil {
		return findings
	}

	if userNode.Kind != yaml.MappingNode || defaultsNode.Kind != yaml.MappingNode {
		return findings
	}

	for i := 0; i+1 < len(userNode.Content); i += 2 {
		keyNode := userNode.Content[i]
		valNode := userNode.Content[i+1]
		key := keyNode.Value
		fullPath := joinPath(path, key)

		if matchesIgnore(fullPath, ignoreKeys) {
			continue
		}

		defaultVal := getValueForKey(defaultsNode, key)
		if defaultVal == nil {
			// Key not in defaults — check schema types if available
			if schemaTypes != nil {
				if allowedTypes, ok := schemaTypes[fullPath]; ok {
					if valNode.ShortTag() != "!!null" {
						compatible, allowedTags := schemaTypesCompatible(valNode.ShortTag(), allowedTypes)
						if !compatible && !(isResourceQuantityPath(fullPath) && isStringIntMismatch(valNode.ShortTag(), allowedTags[0])) {
							findings = append(findings, model.Finding{
								Severity: model.SeverityError,
								Line:     valNode.Line,
								KeyPath:  fullPath,
								Message:  fmt.Sprintf("Type mismatch at %q: expected %s, got %s (%q)", fullPath, friendlyTypes(allowedTags), friendlyType(valNode.ShortTag()), valNode.Value),
							})
						}
					}
				}
			}
			continue
		}

		// Resolve aliases
		if defaultVal.Kind == yaml.AliasNode && defaultVal.Alias != nil {
			defaultVal = defaultVal.Alias
		}
		if valNode.Kind == yaml.AliasNode && valNode.Alias != nil {
			valNode = valNode.Alias
		}

		// Null default — check schema types if available, otherwise accept any type
		if defaultVal.ShortTag() == "!!null" {
			if schemaTypes != nil {
				if allowedTypes, ok := schemaTypes[fullPath]; ok {
					if valNode.ShortTag() != "!!null" {
						compatible, allowedTags := schemaTypesCompatible(valNode.ShortTag(), allowedTypes)
						if !compatible && !(isResourceQuantityPath(fullPath) && isStringIntMismatch(valNode.ShortTag(), allowedTags[0])) {
							findings = append(findings, model.Finding{
								Severity: model.SeverityError,
								Line:     valNode.Line,
								KeyPath:  fullPath,
								Message:  fmt.Sprintf("Type mismatch at %q: expected %s, got %s (%q)", fullPath, friendlyTypes(allowedTags), friendlyType(valNode.ShortTag()), valNode.Value),
							})
						}
					}
				}
			}
			continue
		}

		// User explicitly sets null — always valid
		if valNode.ShortTag() == "!!null" {
			continue
		}

		// Recurse into nested mappings
		if defaultVal.Kind == yaml.MappingNode && valNode.Kind == yaml.MappingNode {
			// Empty mapping default means "accept any structure" (e.g., podSecurityContext: {})
			if len(defaultVal.Content) == 0 {
				continue
			}
			findings = append(findings, detectTypeMismatches(valNode, defaultVal, ignoreKeys, fullPath, schemaTypes)...)
			continue
		}

		// Sequence comparison
		if defaultVal.Kind == yaml.SequenceNode && valNode.Kind == yaml.SequenceNode {
			findings = append(findings, checkSequence(valNode, defaultVal, ignoreKeys, fullPath, schemaTypes)...)
			continue
		}

		// Kubernetes resource quantities (cpu, memory, etc.) accept both strings and numbers
		if isResourceQuantityPath(fullPath) && isStringIntMismatch(valNode.ShortTag(), defaultVal.ShortTag()) {
			continue
		}

		// Type comparison for scalars
		if !typesCompatible(valNode.ShortTag(), defaultVal.ShortTag()) {
			findings = append(findings, model.Finding{
				Severity: model.SeverityError,
				Line:     valNode.Line,
				KeyPath:  fullPath,
				Message:  fmt.Sprintf("Type mismatch at %q: expected %s, got %s (%q)", fullPath, friendlyType(defaultVal.ShortTag()), friendlyType(valNode.ShortTag()), valNode.Value),
			})
			continue
		}

		// Kind mismatch (e.g., user provides scalar where mapping expected)
		if defaultVal.Kind != valNode.Kind && defaultVal.Kind != yaml.ScalarNode && valNode.Kind != yaml.ScalarNode {
			findings = append(findings, model.Finding{
				Severity: model.SeverityError,
				Line:     valNode.Line,
				KeyPath:  fullPath,
				Message:  fmt.Sprintf("Type mismatch at %q: expected %s, got %s", fullPath, kindName(defaultVal.Kind), kindName(valNode.Kind)),
			})
		}
	}

	return findings
}

// checkSequence validates elements in a user sequence against the first element
// of the default sequence as a template.
func checkSequence(userSeq, defaultSeq *yaml.Node, ignoreKeys []string, path string, schemaTypes SchemaTypeMap) []model.Finding {
	var findings []model.Finding

	if len(defaultSeq.Content) == 0 || len(userSeq.Content) == 0 {
		return findings
	}

	template := defaultSeq.Content[0]
	if template.Kind == yaml.AliasNode && template.Alias != nil {
		template = template.Alias
	}

	if template.Kind != yaml.MappingNode {
		return findings
	}

	for idx, elem := range userSeq.Content {
		if elem.Kind == yaml.AliasNode && elem.Alias != nil {
			elem = elem.Alias
		}
		if elem.Kind == yaml.MappingNode {
			elemPath := fmt.Sprintf("%s[%d]", path, idx)
			findings = append(findings, detectUnknownKeys(elem, template, nil, nil, ignoreKeys, elemPath, nil)...)
			findings = append(findings, detectTypeMismatches(elem, template, ignoreKeys, elemPath, schemaTypes)...)
		}
	}

	return findings
}

// typesCompatible checks if two yaml tags are compatible types.
func typesCompatible(userTag, defaultTag string) bool {
	if userTag == defaultTag {
		return true
	}

	// Int and float are compatible
	numericTags := map[string]bool{
		"!!int":   true,
		"!!float": true,
	}
	if numericTags[userTag] && numericTags[defaultTag] {
		return true
	}

	return false
}

func friendlyType(tag string) string {
	switch tag {
	case "!!str":
		return "string"
	case "!!int":
		return "int"
	case "!!float":
		return "float"
	case "!!bool":
		return "bool"
	case "!!null":
		return "null"
	case "!!seq":
		return "list"
	case "!!map":
		return "map"
	default:
		return tag
	}
}

// isResourceQuantityPath returns true if the path looks like a Kubernetes
// resource quantity field (e.g., resources.limits.cpu, resources.requests.memory).
func isResourceQuantityPath(path string) bool {
	parts := strings.Split(path, ".")
	if len(parts) < 3 {
		return false
	}
	// Check for ...resources.{limits,requests}.{cpu,memory,ephemeral-storage,storage,...}
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] == "resources" && (parts[i+1] == "limits" || parts[i+1] == "requests") {
			return true
		}
	}
	return false
}

// isStringIntMismatch returns true if one tag is string and the other is int or float.
func isStringIntMismatch(tagA, tagB string) bool {
	numeric := map[string]bool{"!!int": true, "!!float": true}
	return (tagA == "!!str" && numeric[tagB]) || (tagB == "!!str" && numeric[tagA])
}

func kindName(k yaml.Kind) string {
	switch k {
	case yaml.ScalarNode:
		return "scalar"
	case yaml.MappingNode:
		return "mapping"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.AliasNode:
		return "alias"
	default:
		return "unknown"
	}
}
