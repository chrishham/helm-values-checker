package output

import (
	"github.com/chrishham/helm-values-checker/internal/model"
)

// JSONOutput is the structured JSON output format.
type JSONOutput struct {
	ValuesFile   string        `json:"valuesFile"`
	ChartName    string        `json:"chartName"`
	ChartVersion string        `json:"chartVersion"`
	Errors       []JSONFinding `json:"errors"`
	Warnings     []JSONFinding `json:"warnings"`
	ErrorCount   int           `json:"errorCount"`
	WarningCount int           `json:"warningCount"`
}

// JSONFinding is a single finding in JSON format.
type JSONFinding struct {
	Line       int    `json:"line"`
	KeyPath    string `json:"keyPath"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

// ToJSON converts a ValidationResult to the JSON output structure.
func ToJSON(result *model.ValidationResult) JSONOutput {
	out := JSONOutput{
		ValuesFile:   result.ValuesFile,
		ChartName:    result.ChartName,
		ChartVersion: result.ChartVersion,
		Errors:       make([]JSONFinding, 0),
		Warnings:     make([]JSONFinding, 0),
	}

	for _, f := range result.Errors() {
		out.Errors = append(out.Errors, JSONFinding{
			Line:       f.Line,
			KeyPath:    f.KeyPath,
			Message:    f.Message,
			Suggestion: f.Suggestion,
		})
	}

	for _, f := range result.Warnings() {
		out.Warnings = append(out.Warnings, JSONFinding{
			Line:       f.Line,
			KeyPath:    f.KeyPath,
			Message:    f.Message,
			Suggestion: f.Suggestion,
		})
	}

	out.ErrorCount = len(out.Errors)
	out.WarningCount = len(out.Warnings)

	return out
}
