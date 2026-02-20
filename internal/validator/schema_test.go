package validator

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chrishham/helm-values-checker/internal/model"
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
	findings, err := validateSchema(user, schema, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
	findings, err := validateSchema(user, schema, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
	findings, err := validateSchema(user, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
	findingsWithout, err := validateSchema(user, schema, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
	findingsWith, err := validateSchema(user, schema, nil, schemaTypes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, f := range findingsWith {
		if f.Severity == 0 && f.KeyPath == "replicaCount" {
			t.Error("expected type error to be filtered when schema types provided")
		}
	}
}

func TestContainsExternalRef(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		wantRef  string
	}{
		{
			name:    "fragment ref is allowed",
			json:    `{"$ref": "#/definitions/foo"}`,
			wantRef: "",
		},
		{
			name:    "http ref is blocked",
			json:    `{"properties": {"x": {"$ref": "http://evil.com/schema.json"}}}`,
			wantRef: "http://evil.com/schema.json",
		},
		{
			name:    "file ref is blocked",
			json:    `{"properties": {"x": {"$ref": "file:///etc/passwd"}}}`,
			wantRef: "file:///etc/passwd",
		},
		{
			name:    "nested external ref is blocked",
			json:    `{"properties": {"a": {"properties": {"b": {"$ref": "https://evil.com/s.json"}}}}}`,
			wantRef: "https://evil.com/s.json",
		},
		{
			name:    "no ref at all",
			json:    `{"type": "object", "properties": {"x": {"type": "string"}}}`,
			wantRef: "",
		},
		{
			name:    "ref in array item",
			json:    `{"oneOf": [{"$ref": "http://example.com/a.json"}]}`,
			wantRef: "http://example.com/a.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var parsed interface{}
			if err := json.Unmarshal([]byte(tt.json), &parsed); err != nil {
				t.Fatalf("invalid test JSON: %v", err)
			}
			got := containsExternalRef(parsed)
			if got != tt.wantRef {
				t.Errorf("containsExternalRef() = %q, want %q", got, tt.wantRef)
			}
		})
	}
}

func TestValidateSchema_ExternalRefBlocked(t *testing.T) {
	schema := []byte(`{
		"type": "object",
		"properties": {
			"x": {"$ref": "http://evil.com/schema.json"}
		}
	}`)

	user := parseYAML(t, `x: value`)
	findings, err := validateSchema(user, schema, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != model.SeverityError {
		t.Errorf("expected SeverityError, got %d", findings[0].Severity)
	}
	if !strings.Contains(findings[0].Message, "external $ref") {
		t.Errorf("expected message about external $ref, got: %s", findings[0].Message)
	}
}

func TestValidateSchema_FragmentRefAllowed(t *testing.T) {
	schema := []byte(`{
		"type": "object",
		"definitions": {
			"name": {"type": "string"}
		},
		"properties": {
			"x": {"$ref": "#/definitions/name"}
		}
	}`)

	user := parseYAML(t, `x: hello`)
	findings, err := validateSchema(user, schema, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, f := range findings {
		if strings.Contains(f.Message, "external $ref") {
			t.Errorf("fragment $ref should not be blocked, got: %s", f.Message)
		}
	}
}

func TestValidateSchema_MalformedSchemaReturnsError(t *testing.T) {
	schema := []byte(`{not valid json`)
	user := parseYAML(t, `x: value`)
	_, err := validateSchema(user, schema, nil, nil)
	if err == nil {
		t.Error("expected error for malformed schema, got nil")
	}
}
