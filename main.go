package main

import (
	"errors"
	"os"

	"github.com/chrishham/helm-values-checker/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		var exitErr *cmd.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}
		os.Exit(3)
	}
}
