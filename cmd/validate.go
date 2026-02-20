package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/chrishham/helm-values-checker/internal/chart"
	"github.com/chrishham/helm-values-checker/internal/output"
	"github.com/chrishham/helm-values-checker/internal/validator"
	"github.com/spf13/cobra"
)

// ExitError is returned from runValidate to signal a non-zero exit code
// without calling os.Exit directly, ensuring deferred cleanups run.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit code %d", e.Code)
}

var (
	valuesFiles []string
	chartRef    string
	chartVersion string
	outputFormat string
	strict       bool
	ignoreKeys   []string
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate values files against a Helm chart",
	Long: `Validate one or more values files against a Helm chart's defaults
and optional JSON schema.

Checks performed:
  - Unknown keys (keys not in chart defaults or schema)
  - Type mismatches (string where int expected, etc.)
  - Required fields (from values.schema.json)
  - Deprecated keys (from values.schema.json)

Examples:
  helm-values-checker validate -f my-values.yaml --chart bitnami/postgresql
  helm-values-checker validate -f my-values.yaml --chart ./local-chart/ --strict
  helm-values-checker validate -f my-values.yaml --chart bitnami/postgresql --output json`,
	RunE: runValidate,
}

func init() {
	validateCmd.Flags().StringSliceVarP(&valuesFiles, "file", "f", nil, "Values file(s) to validate (required)")
	validateCmd.Flags().StringVar(&chartRef, "chart", "", "Chart reference: repo/name, OCI URL, or local path (required)")
	validateCmd.Flags().StringVar(&chartVersion, "version", "", "Chart version (optional, latest if omitted)")
	validateCmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text or json")
	validateCmd.Flags().BoolVar(&strict, "strict", false, "Treat warnings as errors (exit code 2)")
	validateCmd.Flags().StringSliceVar(&ignoreKeys, "ignore-keys", nil, "Key paths to ignore (glob patterns, e.g. 'global.*')")

	_ = validateCmd.MarkFlagRequired("file")
	_ = validateCmd.MarkFlagRequired("chart")

	rootCmd.AddCommand(validateCmd)
}

func runValidate(cmd *cobra.Command, args []string) error {
	// Resolve chart
	resolved, err := chart.Resolve(chartRef, chartVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return &ExitError{Code: 3}
	}
	defer resolved.Cleanup()

	// Run validation for each values file
	exitCode := 0
	for _, vf := range valuesFiles {
		result, err := validator.Validate(vf, resolved, ignoreKeys)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error validating %s: %v\n", vf, err)
			return &ExitError{Code: 3}
		}

		switch outputFormat {
		case "json":
			data, err := json.MarshalIndent(output.ToJSON(result), "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
				return &ExitError{Code: 3}
			}
			fmt.Println(string(data))
		default:
			output.PrintText(result, os.Stdout)
		}

		if result.HasErrors() {
			exitCode = 1
		} else if strict && result.HasWarnings() && exitCode < 2 {
			exitCode = 2
		}
	}

	if exitCode != 0 {
		return &ExitError{Code: exitCode}
	}
	return nil
}
