package model

import "fmt"

// Severity represents the severity of a validation finding.
type Severity int

const (
	SeverityError   Severity = iota
	SeverityWarning
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "ERROR"
	case SeverityWarning:
		return "WARNING"
	default:
		return "UNKNOWN"
	}
}

// Finding represents a single validation issue found in user values.
type Finding struct {
	Severity   Severity
	Line       int
	KeyPath    string
	Message    string
	Suggestion string // "did you mean?" suggestion, if any
}

func (f Finding) String() string {
	s := fmt.Sprintf("line %d: %s", f.Line, f.Message)
	if f.Suggestion != "" {
		s += fmt.Sprintf(" (did you mean %q?)", f.Suggestion)
	}
	return s
}

// ValidationResult holds the complete result of a validation run.
type ValidationResult struct {
	ValuesFile string
	ChartName  string
	ChartVersion string
	Findings   []Finding
}

// Errors returns all findings with error severity.
func (r *ValidationResult) Errors() []Finding {
	var out []Finding
	for _, f := range r.Findings {
		if f.Severity == SeverityError {
			out = append(out, f)
		}
	}
	return out
}

// Warnings returns all findings with warning severity.
func (r *ValidationResult) Warnings() []Finding {
	var out []Finding
	for _, f := range r.Findings {
		if f.Severity == SeverityWarning {
			out = append(out, f)
		}
	}
	return out
}

// HasErrors returns true if any error-level findings exist.
func (r *ValidationResult) HasErrors() bool {
	for _, f := range r.Findings {
		if f.Severity == SeverityError {
			return true
		}
	}
	return false
}

// HasWarnings returns true if any warning-level findings exist.
func (r *ValidationResult) HasWarnings() bool {
	for _, f := range r.Findings {
		if f.Severity == SeverityWarning {
			return true
		}
	}
	return false
}
