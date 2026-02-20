package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/chrishham/helm-values-checker/internal/model"
)

func TestPrintText_NoIssues(t *testing.T) {
	result := &model.ValidationResult{
		ValuesFile:   "values.yaml",
		ChartName:    "test-chart",
		ChartVersion: "1.0.0",
	}

	var buf bytes.Buffer
	PrintText(result, &buf)
	output := buf.String()

	if !strings.Contains(output, "No issues found") {
		t.Errorf("expected 'No issues found' in output, got:\n%s", output)
	}
}

func TestPrintText_WithErrors(t *testing.T) {
	result := &model.ValidationResult{
		ValuesFile:   "values.yaml",
		ChartName:    "test-chart",
		ChartVersion: "1.0.0",
		Findings: []model.Finding{
			{
				Severity: model.SeverityError,
				Line:     5,
				KeyPath:  "image.regsitry",
				Message:  `Unknown key "image.regsitry"`,
				Suggestion: "image.registry",
			},
		},
	}

	var buf bytes.Buffer
	PrintText(result, &buf)
	output := buf.String()

	if !strings.Contains(output, "ERRORS (1)") {
		t.Errorf("expected 'ERRORS (1)' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "line 5") {
		t.Errorf("expected 'line 5' in output, got:\n%s", output)
	}
}

func TestToJSON(t *testing.T) {
	result := &model.ValidationResult{
		ValuesFile:   "values.yaml",
		ChartName:    "test-chart",
		ChartVersion: "1.0.0",
		Findings: []model.Finding{
			{Severity: model.SeverityError, Line: 5, KeyPath: "a.b", Message: "err"},
			{Severity: model.SeverityWarning, Line: 10, KeyPath: "c.d", Message: "warn"},
		},
	}

	j := ToJSON(result)
	if j.ErrorCount != 1 {
		t.Errorf("expected 1 error, got %d", j.ErrorCount)
	}
	if j.WarningCount != 1 {
		t.Errorf("expected 1 warning, got %d", j.WarningCount)
	}
}
