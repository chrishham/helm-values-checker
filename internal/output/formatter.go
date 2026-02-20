package output

import (
	"fmt"
	"io"
	"regexp"

	"github.com/chrishham/helm-values-checker/internal/model"
	"github.com/fatih/color"
)

// ansiEscapeRe matches ANSI escape sequences (CSI and OSC).
var ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x1b\x07]*[\x07]`)

// sanitize strips ANSI escape sequences and control characters (0x00-0x1F)
// from s, preserving only newlines and tabs.
func sanitize(s string) string {
	s = ansiEscapeRe.ReplaceAllString(s, "")
	buf := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b < 0x20 && b != '\n' && b != '\t' {
			continue
		}
		buf = append(buf, b)
	}
	return string(buf)
}

// PrintText writes a human-readable validation report to w.
func PrintText(result *model.ValidationResult, w io.Writer) {
	header := color.New(color.Bold)
	header.Fprintf(w, "Validating %s against %s", sanitize(result.ValuesFile), sanitize(result.ChartName))
	if result.ChartVersion != "" {
		header.Fprintf(w, " (%s)", sanitize(result.ChartVersion))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w)

	errors := result.Errors()
	warnings := result.Warnings()

	if len(errors) > 0 {
		errHeader := color.New(color.FgRed, color.Bold)
		errHeader.Fprintf(w, "ERRORS (%d)\n", len(errors))
		for _, f := range errors {
			fmt.Fprintf(w, "  ")
			color.New(color.FgRed).Fprintf(w, "line %d", f.Line)
			fmt.Fprintf(w, ": %s", sanitize(f.Message))
			if f.Suggestion != "" {
				color.New(color.FgYellow).Fprintf(w, " (did you mean %q?)", sanitize(f.Suggestion))
			}
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w)
	}

	if len(warnings) > 0 {
		warnHeader := color.New(color.FgYellow, color.Bold)
		warnHeader.Fprintf(w, "WARNINGS (%d)\n", len(warnings))
		for _, f := range warnings {
			fmt.Fprintf(w, "  ")
			color.New(color.FgYellow).Fprintf(w, "line %d", f.Line)
			fmt.Fprintf(w, ": %s", sanitize(f.Message))
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w)
	}

	summaryColor := color.New(color.Bold)
	if len(errors) == 0 && len(warnings) == 0 {
		color.New(color.FgGreen, color.Bold).Fprintln(w, "No issues found.")
	} else {
		summaryColor.Fprintf(w, "Summary: %d error(s), %d warning(s)\n", len(errors), len(warnings))
	}
}
