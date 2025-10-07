package main

import (
	"os"

	analyzecli "github.com/replicatedhq/troubleshoot/cmd/analyze/cli"
)

func main() {
	if err := analyzecli.RootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
