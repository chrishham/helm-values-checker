package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "helm-values-checker",
	Short: "Validate Helm values files against chart defaults and schema",
	Long: `helm-values-checker compares your values file against a Helm chart's
default values and optional JSON schema to detect unknown keys, type
mismatches, missing required fields, and deprecated keys.

Install as a Helm plugin to use as: helm values-checker validate`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
