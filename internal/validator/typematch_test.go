package validator

import (
	"testing"
)

func TestDetectTypeMismatches_NoMismatches(t *testing.T) {
	defaults := parseYAML(t, `
replicaCount: 1
image:
  repository: nginx
  tag: latest
enabled: true
`)
	user := parseYAML(t, `
replicaCount: 3
image:
  repository: myapp
  tag: "v1.0"
enabled: false
`)
	findings := detectTypeMismatches(user, defaults, nil, "", nil)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestDetectTypeMismatches_StringForInt(t *testing.T) {
	defaults := parseYAML(t, `
replicaCount: 1
`)
	user := parseYAML(t, `
replicaCount: "three"
`)
	findings := detectTypeMismatches(user, defaults, nil, "", nil)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].KeyPath != "replicaCount" {
		t.Errorf("expected keyPath 'replicaCount', got %q", findings[0].KeyPath)
	}
}

func TestDetectTypeMismatches_BoolForString(t *testing.T) {
	defaults := parseYAML(t, `
name: "default"
`)
	user := parseYAML(t, `
name: true
`)
	findings := detectTypeMismatches(user, defaults, nil, "", nil)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestDetectTypeMismatches_NullDefault(t *testing.T) {
	defaults := parseYAML(t, `
customValue: null
`)
	user := parseYAML(t, `
customValue: "anything-goes"
`)
	findings := detectTypeMismatches(user, defaults, nil, "", nil)
	if len(findings) != 0 {
		t.Errorf("expected no findings for null default, got %d: %v", len(findings), findings)
	}
}

func TestDetectTypeMismatches_IntFloatCompatible(t *testing.T) {
	defaults := parseYAML(t, `
ratio: 1.5
`)
	user := parseYAML(t, `
ratio: 2
`)
	findings := detectTypeMismatches(user, defaults, nil, "", nil)
	if len(findings) != 0 {
		t.Errorf("expected no findings for int/float compat, got %d: %v", len(findings), findings)
	}
}

func TestDetectTypeMismatches_Nested(t *testing.T) {
	defaults := parseYAML(t, `
config:
  settings:
    timeout: 30s
    retries: 3
`)
	user := parseYAML(t, `
config:
  settings:
    timeout: 30s
    retries: "three"
`)
	findings := detectTypeMismatches(user, defaults, nil, "", nil)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for nested type mismatch, got %d: %v", len(findings), findings)
	}
	if findings[0].KeyPath != "config.settings.retries" {
		t.Errorf("expected keyPath 'config.settings.retries', got %q", findings[0].KeyPath)
	}
}

func TestDetectTypeMismatches_ResourceQuantityAllowsIntForString(t *testing.T) {
	defaults := parseYAML(t, `
resources:
  limits:
    cpu: 1000m
    memory: 2Gi
  requests:
    cpu: 200m
    memory: 256Mi
`)
	user := parseYAML(t, `
resources:
  limits:
    cpu: 10
    memory: 4096
  requests:
    cpu: 2
    memory: 512
`)
	findings := detectTypeMismatches(user, defaults, nil, "", nil)
	if len(findings) != 0 {
		t.Errorf("expected no findings for resource quantity int/string compat, got %d:", len(findings))
		for _, f := range findings {
			t.Errorf("  - %s: %s", f.KeyPath, f.Message)
		}
	}
}

func TestDetectTypeMismatches_ResourceQuantityNestedInComponent(t *testing.T) {
	defaults := parseYAML(t, `
clickhouse:
  statefulSet:
    resources:
      limits:
        cpu: 1000m
        memory: 2Gi
`)
	user := parseYAML(t, `
clickhouse:
  statefulSet:
    resources:
      limits:
        cpu: 10
        memory: 4096
`)
	findings := detectTypeMismatches(user, defaults, nil, "", nil)
	if len(findings) != 0 {
		t.Errorf("expected no findings for deeply nested resource quantity, got %d:", len(findings))
		for _, f := range findings {
			t.Errorf("  - %s: %s", f.KeyPath, f.Message)
		}
	}
}

func TestDetectTypeMismatches_EmptyMapDefault(t *testing.T) {
	defaults := parseYAML(t, `
podSecurityContext: {}
replicaCount: 1
`)
	user := parseYAML(t, `
podSecurityContext:
  runAsUser: 1000
  fsGroup: "2000"
replicaCount: 2
`)
	findings := detectTypeMismatches(user, defaults, nil, "", nil)
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty map default children, got %d:", len(findings))
		for _, f := range findings {
			t.Errorf("  - %s: %s", f.KeyPath, f.Message)
		}
	}
}

func TestDetectTypeMismatches_NonEmptyMapStillChecked(t *testing.T) {
	defaults := parseYAML(t, `
config:
  settings:
    name: "default"
    count: 5
`)
	user := parseYAML(t, `
config:
  settings:
    name: true
    count: 5
`)
	findings := detectTypeMismatches(user, defaults, nil, "", nil)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for non-empty map type mismatch, got %d: %v", len(findings), findings)
	}
	if findings[0].KeyPath != "config.settings.name" {
		t.Errorf("expected keyPath 'config.settings.name', got %q", findings[0].KeyPath)
	}
}

func TestDetectTypeMismatches_UserNull(t *testing.T) {
	defaults := parseYAML(t, `
name: "default"
`)
	user := parseYAML(t, `
name: null
`)
	findings := detectTypeMismatches(user, defaults, nil, "", nil)
	if len(findings) != 0 {
		t.Errorf("expected no findings for user null, got %d: %v", len(findings), findings)
	}
}

func TestDetectTypeMismatches_NullDefault_SchemaInteger_StringValue(t *testing.T) {
	defaults := parseYAML(t, `
maxRetries: null
`)
	user := parseYAML(t, `
maxRetries: "not-a-number"
`)
	schema := SchemaTypeMap{"maxRetries": {"integer", "null"}}
	findings := detectTypeMismatches(user, defaults, nil, "", schema)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].KeyPath != "maxRetries" {
		t.Errorf("expected keyPath 'maxRetries', got %q", findings[0].KeyPath)
	}
}

func TestDetectTypeMismatches_NullDefault_SchemaString_StringValue(t *testing.T) {
	defaults := parseYAML(t, `
label: null
`)
	user := parseYAML(t, `
label: "hello"
`)
	schema := SchemaTypeMap{"label": {"string"}}
	findings := detectTypeMismatches(user, defaults, nil, "", schema)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestDetectTypeMismatches_NullDefault_UnionType_IntValue(t *testing.T) {
	defaults := parseYAML(t, `
maxRetries: null
`)
	user := parseYAML(t, `
maxRetries: 5
`)
	schema := SchemaTypeMap{"maxRetries": {"integer", "null"}}
	findings := detectTypeMismatches(user, defaults, nil, "", schema)
	if len(findings) != 0 {
		t.Errorf("expected no findings for int matching integer|null, got %d: %v", len(findings), findings)
	}
}

func TestDetectTypeMismatches_NullDefault_UnionType_BoolValue(t *testing.T) {
	defaults := parseYAML(t, `
maxRetries: null
`)
	user := parseYAML(t, `
maxRetries: true
`)
	schema := SchemaTypeMap{"maxRetries": {"integer", "null"}}
	findings := detectTypeMismatches(user, defaults, nil, "", schema)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for bool vs integer|null, got %d: %v", len(findings), findings)
	}
}

func TestDetectTypeMismatches_NullDefault_NoSchema_AnyValue(t *testing.T) {
	defaults := parseYAML(t, `
customValue: null
`)
	user := parseYAML(t, `
customValue: "anything-goes"
`)
	findings := detectTypeMismatches(user, defaults, nil, "", nil)
	if len(findings) != 0 {
		t.Errorf("expected no findings for null default without schema, got %d: %v", len(findings), findings)
	}
}

func TestDetectTypeMismatches_SchemaOnly_WrongType(t *testing.T) {
	defaults := parseYAML(t, `
other: "value"
`)
	user := parseYAML(t, `
other: "value"
schemaOnly: true
`)
	schema := SchemaTypeMap{"schemaOnly": {"string"}}
	findings := detectTypeMismatches(user, defaults, nil, "", schema)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for schema-only key type mismatch, got %d: %v", len(findings), findings)
	}
	if findings[0].KeyPath != "schemaOnly" {
		t.Errorf("expected keyPath 'schemaOnly', got %q", findings[0].KeyPath)
	}
}

func TestDetectTypeMismatches_SchemaNumber_IntValue(t *testing.T) {
	defaults := parseYAML(t, `
ratio: null
`)
	user := parseYAML(t, `
ratio: 42
`)
	schema := SchemaTypeMap{"ratio": {"number"}}
	findings := detectTypeMismatches(user, defaults, nil, "", schema)
	if len(findings) != 0 {
		t.Errorf("expected no findings for int matching number schema, got %d: %v", len(findings), findings)
	}
}

func TestDetectTypeMismatches_UserNull_AlwaysOK(t *testing.T) {
	defaults := parseYAML(t, `
maxRetries: null
`)
	user := parseYAML(t, `
maxRetries: null
`)
	schema := SchemaTypeMap{"maxRetries": {"integer"}}
	findings := detectTypeMismatches(user, defaults, nil, "", schema)
	if len(findings) != 0 {
		t.Errorf("expected no findings for user null regardless of schema, got %d: %v", len(findings), findings)
	}
}
