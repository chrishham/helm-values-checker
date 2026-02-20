package validator

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chrishham/helm-values-checker/internal/model"
	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v3"
)

// SchemaTypeMap maps dot-separated property paths to the allowed JSON Schema
// type strings (e.g., "maxRetries" → ["integer", "null"]).
type SchemaTypeMap map[string][]string

// extractSchemaTypes parses a JSON schema and returns a map of property paths
// to their allowed type(s).
func extractSchemaTypes(schemaBytes []byte) SchemaTypeMap {
	types := make(SchemaTypeMap)
	if len(schemaBytes) == 0 {
		return types
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		return types
	}

	walkSchemaTypes(schema, "", types)
	return types
}

func walkSchemaTypes(schema map[string]interface{}, path string, types SchemaTypeMap) {
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return
	}

	for name, v := range props {
		propDef, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		fullPath := joinPath(path, name)

		// Collect type(s) — handles both "type": "string" and "type": ["string", "null"]
		switch t := propDef["type"].(type) {
		case string:
			types[fullPath] = []string{t}
		case []interface{}:
			var typeList []string
			for _, item := range t {
				if s, ok := item.(string); ok {
					typeList = append(typeList, s)
				}
			}
			if len(typeList) > 0 {
				types[fullPath] = typeList
			}
		}

		// Recurse into nested properties
		walkSchemaTypes(propDef, fullPath, types)
	}
}

// validateSchema runs JSON Schema validation on user values, checking
// required fields and deprecated markers. When schemaTypes is non-nil,
// invalid_type errors are filtered out because the custom type checker
// handles those with better messages.
func validateSchema(userNode *yaml.Node, schemaBytes []byte, ignoreKeys []string, schemaTypes SchemaTypeMap) []model.Finding {
	var findings []model.Finding

	if len(schemaBytes) == 0 {
		return findings
	}

	// Convert user yaml.Node tree to a generic map for JSON schema validation
	var userMap interface{}
	userYAML, err := yaml.Marshal(userNode)
	if err != nil {
		return findings
	}
	if err := yaml.Unmarshal(userYAML, &userMap); err != nil {
		return findings
	}

	// JSON Schema validation for required fields
	schemaLoader := gojsonschema.NewBytesLoader(schemaBytes)
	userJSON, err := json.Marshal(userMap)
	if err != nil {
		return findings
	}
	docLoader := gojsonschema.NewBytesLoader(userJSON)

	result, err := gojsonschema.Validate(schemaLoader, docLoader)
	if err != nil {
		return findings
	}

	for _, e := range result.Errors() {
		// Skip type errors when custom type checker handles them
		if len(schemaTypes) > 0 && e.Type() == "invalid_type" {
			continue
		}

		field := e.Field()
		if field == "(root)" {
			field = ""
		}

		path := field
		if path != "" {
			path = strings.ReplaceAll(path, "/", ".")
		}

		if matchesIgnore(path, ignoreKeys) {
			continue
		}

		findings = append(findings, model.Finding{
			Severity: model.SeverityError,
			Line:     findLineForPath(userNode, path),
			KeyPath:  path,
			Message:  fmt.Sprintf("Schema validation: %s", e.Description()),
		})
	}

	// Check for deprecated keys
	findings = append(findings, checkDeprecated(userNode, schemaBytes, ignoreKeys)...)

	return findings
}

// extractSchemaKeys extracts all property paths defined in a JSON schema.
func extractSchemaKeys(schemaBytes []byte) map[string]bool {
	keys := make(map[string]bool)
	if len(schemaBytes) == 0 {
		return keys
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		return keys
	}

	walkSchemaProperties(schema, "", keys)
	return keys
}

func walkSchemaProperties(schema map[string]interface{}, path string, keys map[string]bool) {
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return
	}

	for name, v := range props {
		fullPath := joinPath(path, name)
		keys[fullPath] = true

		if propDef, ok := v.(map[string]interface{}); ok {
			walkSchemaProperties(propDef, fullPath, keys)
		}
	}
}

// checkDeprecated walks the JSON schema looking for deprecated markers
// and warns when user values set those keys.
func checkDeprecated(userNode *yaml.Node, schemaBytes []byte, ignoreKeys []string) []model.Finding {
	var findings []model.Finding

	var schema map[string]interface{}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		return findings
	}

	deprecated := findDeprecatedPaths(schema, "")
	for path, msg := range deprecated {
		if matchesIgnore(path, ignoreKeys) {
			continue
		}

		if line := findLineForPath(userNode, path); line > 0 {
			message := fmt.Sprintf("Deprecated key %q", path)
			if msg != "" {
				message += " - " + msg
			}
			findings = append(findings, model.Finding{
				Severity: model.SeverityWarning,
				Line:     line,
				KeyPath:  path,
				Message:  message,
			})
		}
	}

	return findings
}

// findDeprecatedPaths walks schema properties looking for deprecated markers.
func findDeprecatedPaths(schema map[string]interface{}, path string) map[string]string {
	result := make(map[string]string)

	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return result
	}

	for name, v := range props {
		propDef, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		fullPath := joinPath(path, name)

		if dep, ok := propDef["deprecated"].(bool); ok && dep {
			msg := ""
			if desc, ok := propDef["description"].(string); ok {
				msg = desc
			}
			result[fullPath] = msg
		}

		// Recurse into nested properties
		for k, v := range findDeprecatedPaths(propDef, fullPath) {
			result[k] = v
		}
	}

	return result
}

// findLineForPath tries to find the line number for a dot-separated path
// in the yaml.Node tree.
func findLineForPath(node *yaml.Node, path string) int {
	if path == "" {
		return node.Line
	}

	parts := strings.SplitN(path, ".", 2)
	key := parts[0]
	rest := ""
	if len(parts) > 1 {
		rest = parts[1]
	}

	if node.Kind != yaml.MappingNode {
		return 0
	}

	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			if rest == "" {
				return node.Content[i].Line
			}
			return findLineForPath(node.Content[i+1], rest)
		}
	}

	return 0
}
