package main

import (
	"os"

	"github.com/chrishham/helm-values-checker/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(3)
	}
}
