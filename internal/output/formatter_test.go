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

func TestSanitize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text unchanged",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "ANSI color codes stripped",
			input: "\x1b[31mred text\x1b[0m",
			want:  "red text",
		},
		{
			name:  "null bytes stripped",
			input: "before\x00after",
			want:  "beforeafter",
		},
		{
			name:  "carriage return stripped",
			input: "line\r\nend",
			want:  "line\nend",
		},
		{
			name:  "tabs and newlines preserved",
			input: "col1\tcol2\nrow2",
			want:  "col1\tcol2\nrow2",
		},
		{
			name:  "bell character stripped",
			input: "alert\x07here",
			want:  "alerthere",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitize(tt.input)
			if got != tt.want {
				t.Errorf("sanitize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPrintText_SanitizesOutput(t *testing.T) {
	result := &model.ValidationResult{
		ValuesFile:   "values\x1b[31m.yaml",
		ChartName:    "evil\x1b[0m-chart",
		ChartVersion: "1.0.0",
		Findings: []model.Finding{
			{
				Severity:   model.SeverityError,
				Line:       1,
				KeyPath:    "key",
				Message:    "bad \x1b[31mred\x1b[0m message",
				Suggestion: "good\x1b[32m suggestion",
			},
		},
	}

	var buf bytes.Buffer
	PrintText(result, &buf)
	output := buf.String()

	if strings.Contains(output, "\x1b[31m.yaml") {
		t.Error("ANSI escape in ValuesFile was not sanitized")
	}
	if strings.Contains(output, "evil\x1b[0m") {
		t.Error("ANSI escape in ChartName was not sanitized")
	}
	if strings.Contains(output, "\x1b[31mred") {
		t.Error("ANSI escape in Message was not sanitized")
	}
	if strings.Contains(output, "\x1b[32m suggestion") {
		t.Error("ANSI escape in Suggestion was not sanitized")
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
