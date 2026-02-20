package output

import (
	"fmt"
	"io"

	"github.com/chrishham/helm-values-checker/internal/model"
	"github.com/fatih/color"
)

// PrintText writes a human-readable validation report to w.
func PrintText(result *model.ValidationResult, w io.Writer) {
	header := color.New(color.Bold)
	header.Fprintf(w, "Validating %s against %s", result.ValuesFile, result.ChartName)
	if result.ChartVersion != "" {
		header.Fprintf(w, " (%s)", result.ChartVersion)
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
			fmt.Fprintf(w, ": %s", f.Message)
			if f.Suggestion != "" {
				color.New(color.FgYellow).Fprintf(w, " (did you mean %q?)", f.Suggestion)
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
			fmt.Fprintf(w, ": %s", f.Message)
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
