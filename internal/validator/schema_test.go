package validator

import (
	"testing"
)

func TestExtractSchemaKeys(t *testing.T) {
	schema := []byte(`{
		"type": "object",
		"properties": {
			"replicaCount": {"type": "integer"},
			"image": {
				"type": "object",
				"properties": {
					"repository": {"type": "string"},
					"tag": {"type": "string"}
				}
			},
			"extraConfig": {"type": "object"}
		}
	}`)

	keys := extractSchemaKeys(schema)
	expected := []string{"replicaCount", "image", "image.repository", "image.tag", "extraConfig"}
	for _, k := range expected {
		if !keys[k] {
			t.Errorf("expected schema key %q to be present", k)
		}
	}
}

func TestValidateSchema_RequiredFields(t *testing.T) {
	schema := []byte(`{
		"type": "object",
		"required": ["name"],
		"properties": {
			"name": {"type": "string", "minLength": 1}
		}
	}`)

	user := parseYAML(t, `
other: value
`)
	findings := validateSchema(user, schema, nil, nil)
	found := false
	for _, f := range findings {
		if f.Severity == 0 { // SeverityError
			found = true
		}
	}
	if !found {
		t.Error("expected error finding for missing required field 'name'")
	}
}

func TestValidateSchema_DeprecatedKeys(t *testing.T) {
	schema := []byte(`{
		"type": "object",
		"properties": {
			"oldSetting": {
				"type": "string",
				"deprecated": true,
				"description": "use newSetting instead"
			}
		}
	}`)

	user := parseYAML(t, `
oldSetting: "some-value"
`)
	findings := validateSchema(user, schema, nil, nil)
	found := false
	for _, f := range findings {
		if f.Severity == 1 { // SeverityWarning
			found = true
		}
	}
	if !found {
		t.Error("expected warning finding for deprecated key 'oldSetting'")
	}
}

func TestValidateSchema_NoSchema(t *testing.T) {
	user := parseYAML(t, `
anything: goes
`)
	findings := validateSchema(user, nil, nil, nil)
	if len(findings) != 0 {
		t.Errorf("expected no findings with nil schema, got %d", len(findings))
	}
}

func TestExtractSchemaTypes(t *testing.T) {
	schema := []byte(`{
		"type": "object",
		"properties": {
			"replicaCount": {"type": "integer"},
			"label": {"type": ["string", "null"]},
			"config": {
				"type": "object",
				"properties": {
					"timeout": {"type": "number"}
				}
			}
		}
	}`)

	types := extractSchemaTypes(schema)

	tests := []struct {
		path     string
		expected []string
	}{
		{"replicaCount", []string{"integer"}},
		{"label", []string{"string", "null"}},
		{"config", []string{"object"}},
		{"config.timeout", []string{"number"}},
	}

	for _, tt := range tests {
		got, ok := types[tt.path]
		if !ok {
			t.Errorf("expected types for %q to be present", tt.path)
			continue
		}
		if len(got) != len(tt.expected) {
			t.Errorf("types[%q]: expected %v, got %v", tt.path, tt.expected, got)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("types[%q][%d]: expected %q, got %q", tt.path, i, tt.expected[i], got[i])
			}
		}
	}
}

func TestValidateSchema_TypeErrorsFiltered(t *testing.T) {
	schema := []byte(`{
		"type": "object",
		"properties": {
			"replicaCount": {"type": "integer"}
		}
	}`)

	user := parseYAML(t, `
replicaCount: "not-a-number"
`)

	// Without schema types: invalid_type error should appear
	findingsWithout := validateSchema(user, schema, nil, nil)
	foundType := false
	for _, f := range findingsWithout {
		if f.Severity == 0 && f.KeyPath == "replicaCount" {
			foundType = true
		}
	}
	if !foundType {
		t.Error("expected type error without schema types filter")
	}

	// With schema types: invalid_type error should be filtered
	schemaTypes := SchemaTypeMap{"replicaCount": {"integer"}}
	findingsWith := validateSchema(user, schema, nil, schemaTypes)
	for _, f := range findingsWith {
		if f.Severity == 0 && f.KeyPath == "replicaCount" {
			t.Error("expected type error to be filtered when schema types provided")
		}
	}
}
