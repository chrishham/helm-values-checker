package validator

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/chrishham/helm-values-checker/internal/chart"
)

func testdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata")
}

func TestValidate_GoodValues(t *testing.T) {
	chartPath := filepath.Join(testdataDir(), "test-chart")
	resolved, err := chart.Resolve(chartPath, "")
	if err != nil {
		t.Fatalf("failed to resolve chart: %v", err)
	}
	defer resolved.Cleanup()

	result, err := Validate(filepath.Join(testdataDir(), "good-values.yaml"), resolved, nil)
	if err != nil {
		t.Fatalf("validation error: %v", err)
	}

	if result.HasErrors() {
		t.Errorf("expected no errors for good values, got: %v", result.Errors())
	}
}

func TestValidate_BadValues(t *testing.T) {
	chartPath := filepath.Join(testdataDir(), "test-chart")
	resolved, err := chart.Resolve(chartPath, "")
	if err != nil {
		t.Fatalf("failed to resolve chart: %v", err)
	}
	defer resolved.Cleanup()

	result, err := Validate(filepath.Join(testdataDir(), "bad-values.yaml"), resolved, nil)
	if err != nil {
		t.Fatalf("validation error: %v", err)
	}

	errors := result.Errors()
	if len(errors) == 0 {
		t.Fatal("expected errors for bad values, got none")
	}

	// Should find: image.regsitry (unknown), service.tyep (unknown),
	// unknownKey (unknown), replicaCount type mismatch, persistence.enabled type mismatch
	foundUnknown := false
	foundTypeMismatch := false
	for _, f := range errors {
		if f.KeyPath == "image.regsitry" {
			foundUnknown = true
		}
		if f.KeyPath == "replicaCount" {
			foundTypeMismatch = true
		}
	}
	if !foundUnknown {
		t.Error("expected to find unknown key 'image.regsitry'")
	}
	if !foundTypeMismatch {
		t.Error("expected to find type mismatch for 'replicaCount'")
	}
}

func TestValidate_SubchartValues(t *testing.T) {
	chartPath := filepath.Join(testdataDir(), "test-chart")
	resolved, err := chart.Resolve(chartPath, "")
	if err != nil {
		t.Fatalf("failed to resolve chart: %v", err)
	}
	defer resolved.Cleanup()

	result, err := Validate(filepath.Join(testdataDir(), "subchart-values.yaml"), resolved, nil)
	if err != nil {
		t.Fatalf("validation error: %v", err)
	}

	found := false
	for _, f := range result.Errors() {
		if f.KeyPath == "mysubchart.unknownSubKey" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error for 'mysubchart.unknownSubKey', findings: %v", result.Findings)
	}
}

func TestValidate_SchemaValidation(t *testing.T) {
	chartPath := filepath.Join(testdataDir(), "test-chart-with-schema")
	resolved, err := chart.Resolve(chartPath, "")
	if err != nil {
		t.Fatalf("failed to resolve chart: %v", err)
	}
	defer resolved.Cleanup()

	result, err := Validate(filepath.Join(testdataDir(), "schema-bad-values.yaml"), resolved, nil)
	if err != nil {
		t.Fatalf("validation error: %v", err)
	}

	// Should have deprecated warning for oldSetting
	foundDeprecated := false
	for _, f := range result.Warnings() {
		if f.KeyPath == "oldSetting" {
			foundDeprecated = true
		}
	}
	if !foundDeprecated {
		t.Errorf("expected deprecated warning for 'oldSetting', findings: %v", result.Findings)
	}
}

func TestValidate_SchemaTypeFallback(t *testing.T) {
	chartPath := filepath.Join(testdataDir(), "test-chart-with-schema")
	resolved, err := chart.Resolve(chartPath, "")
	if err != nil {
		t.Fatalf("failed to resolve chart: %v", err)
	}
	defer resolved.Cleanup()

	result, err := Validate(filepath.Join(testdataDir(), "schema-type-bad-values.yaml"), resolved, nil)
	if err != nil {
		t.Fatalf("validation error: %v", err)
	}

	// Should catch maxRetries type mismatch (default is null, schema says integer|null, user provides string)
	foundTypeMismatch := false
	for _, f := range result.Errors() {
		if f.KeyPath == "maxRetries" {
			foundTypeMismatch = true
		}
	}
	if !foundTypeMismatch {
		t.Errorf("expected type mismatch error for 'maxRetries', findings: %v", result.Findings)
	}

	// Should NOT have a duplicate invalid_type error from gojsonschema
	typeErrorCount := 0
	for _, f := range result.Errors() {
		if f.KeyPath == "maxRetries" {
			typeErrorCount++
		}
	}
	if typeErrorCount > 1 {
		t.Errorf("expected exactly 1 error for 'maxRetries', got %d (duplicate type errors not filtered)", typeErrorCount)
	}
}
