package validator

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func parseYAML(t *testing.T, s string) *yaml.Node {
	t.Helper()
	node := &yaml.Node{}
	if err := yaml.Unmarshal([]byte(s), node); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return node.Content[0]
	}
	return node
}

func TestDetectUnknownKeys_NoUnknowns(t *testing.T) {
	defaults := parseYAML(t, `
image:
  repository: nginx
  tag: latest
replicaCount: 1
`)
	user := parseYAML(t, `
image:
  repository: myapp
replicaCount: 2
`)
	findings := detectUnknownKeys(user, defaults, nil, nil, nil, "", nil)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestDetectUnknownKeys_WithUnknowns(t *testing.T) {
	defaults := parseYAML(t, `
image:
  repository: nginx
  tag: latest
replicaCount: 1
`)
	user := parseYAML(t, `
image:
  repository: myapp
  regsitry: docker.io
replicaCount: 2
unknownKey: true
`)
	findings := detectUnknownKeys(user, defaults, nil, nil, nil, "", nil)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d: %v", len(findings), findings)
	}

	// Check first finding is for image.regsitry (with suggestion)
	if findings[0].KeyPath != "image.regsitry" {
		t.Errorf("expected keyPath 'image.regsitry', got %q", findings[0].KeyPath)
	}
	if findings[0].Suggestion == "" {
		t.Errorf("expected a suggestion for 'regsitry'")
	}

	// Check second finding is for unknownKey
	if findings[1].KeyPath != "unknownKey" {
		t.Errorf("expected keyPath 'unknownKey', got %q", findings[1].KeyPath)
	}
}

func TestDetectUnknownKeys_WithIgnore(t *testing.T) {
	defaults := parseYAML(t, `
image:
  repository: nginx
`)
	user := parseYAML(t, `
image:
  repository: myapp
  unknownField: value
customKey: true
`)
	findings := detectUnknownKeys(user, defaults, nil, nil, []string{"image.*", "customKey"}, "", nil)
	if len(findings) != 0 {
		t.Errorf("expected no findings with ignore patterns, got %d: %v", len(findings), findings)
	}
}

func TestDetectUnknownKeys_WithSubchart(t *testing.T) {
	defaults := parseYAML(t, `
replicaCount: 1
`)
	subDefaults := map[string]*yaml.Node{
		"redis": parseYAML(t, `
enabled: true
replicas: 1
`),
	}
	user := parseYAML(t, `
replicaCount: 2
redis:
  enabled: true
  replicas: 3
  unknownSubKey: false
`)
	findings := detectUnknownKeys(user, defaults, nil, subDefaults, nil, "", nil)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for subchart unknown key, got %d: %v", len(findings), findings)
	}
	if findings[0].KeyPath != "redis.unknownSubKey" {
		t.Errorf("expected keyPath 'redis.unknownSubKey', got %q", findings[0].KeyPath)
	}
}

func TestDetectUnknownKeys_EmptyMapDefault(t *testing.T) {
	defaults := parseYAML(t, `
podSecurityContext: {}
securityContext: {}
annotations: {}
image:
  repository: nginx
`)
	user := parseYAML(t, `
podSecurityContext:
  runAsUser: 1000
  fsGroup: 2000
securityContext:
  runAsNonRoot: true
  readOnlyRootFilesystem: true
annotations:
  sidecar.istio.io/inject: "true"
  service.beta.kubernetes.io/azure-load-balancer-internal: "true"
image:
  repository: myapp
`)
	findings := detectUnknownKeys(user, defaults, nil, nil, nil, "", nil)
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty map defaults, got %d:", len(findings))
		for _, f := range findings {
			t.Errorf("  - %s", f.KeyPath)
		}
	}
}

func TestDetectUnknownKeys_EmptyMapDefault_StillCatchesUnknownSiblings(t *testing.T) {
	defaults := parseYAML(t, `
podSecurityContext: {}
replicaCount: 1
`)
	user := parseYAML(t, `
podSecurityContext:
  runAsUser: 1000
replicaCount: 2
completelyUnknown: true
`)
	findings := detectUnknownKeys(user, defaults, nil, nil, nil, "", nil)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for sibling unknown key, got %d: %v", len(findings), findings)
	}
	if findings[0].KeyPath != "completelyUnknown" {
		t.Errorf("expected keyPath 'completelyUnknown', got %q", findings[0].KeyPath)
	}
}

func TestDetectUnknownKeys_DeepSuggestion(t *testing.T) {
	defaults := parseYAML(t, `
config:
  basicAuth:
    jwtSecret: ""
  security:
    cors:
      allowedOrigins: "*"
  userOrgCreationDisabled: true
`)
	allPaths := collectAllPaths(defaults, "")
	user := parseYAML(t, `
config:
  cors:
    allowedOrigins: "https://example.com"
  jwtSecret: "secret123"
  orgCreationDisabled: true
`)
	findings := detectUnknownKeys(user, defaults, nil, nil, nil, "", allPaths)
	if len(findings) != 3 {
		t.Fatalf("expected 3 findings, got %d: %v", len(findings), findings)
	}

	// config.cors should suggest config.security.cors
	if findings[0].Suggestion != "config.security.cors" {
		t.Errorf("expected suggestion 'config.security.cors', got %q", findings[0].Suggestion)
	}

	// config.jwtSecret should suggest config.basicAuth.jwtSecret
	if findings[1].Suggestion != "config.basicAuth.jwtSecret" {
		t.Errorf("expected suggestion 'config.basicAuth.jwtSecret', got %q", findings[1].Suggestion)
	}

	// config.orgCreationDisabled should suggest config.userOrgCreationDisabled
	if findings[2].Suggestion != "config.userOrgCreationDisabled" {
		t.Errorf("expected suggestion 'config.userOrgCreationDisabled', got %q", findings[2].Suggestion)
	}
}

func TestFindClosestKey(t *testing.T) {
	candidates := map[string]bool{
		"repository": true,
		"registry":   true,
		"tag":        true,
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"regsitry", "registry"},
		{"repositroy", "repository"},
		{"tga", "tag"},
		{"completelyDifferent", ""},
	}

	for _, tt := range tests {
		result := findClosestKey(tt.input, candidates)
		if result != tt.expected {
			t.Errorf("findClosestKey(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		match   bool
	}{
		{"image.*", "image.repository", true},
		{"image.*", "image.tag", true},
		{"image.*", "service.port", false},
		{"global.**", "global.imageRegistry", true},
		{"global.**", "global.sub.deep", true},
		{"exact.key", "exact.key", true},
		{"exact.key", "exact.other", false},
	}

	for _, tt := range tests {
		result := matchGlob(tt.pattern, tt.path)
		if result != tt.match {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.pattern, tt.path, result, tt.match)
		}
	}
}
