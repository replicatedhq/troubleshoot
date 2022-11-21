package cli

import (
	"log"
	"os"

	preflightcli "github.com/replicatedhq/troubleshoot/cmd/preflight/cli"
	troubleshootcli "github.com/replicatedhq/troubleshoot/cmd/troubleshoot/cli"
	"github.com/spf13/cobra"

	"github.com/spf13/cobra/doc"
)

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docsgen",
		Short: "Generate markdown docs for the commands in this project",
	}
	preflight := preflightcli.RootCmd()
	troubleshoot := troubleshootcli.RootCmd()
	commands := []*cobra.Command{preflight, troubleshoot}

	for _, command := range commands {
		err := doc.GenMarkdownTree(command, "./docs")
		if err != nil {
			log.Fatal(err)
		}
	}

	return cmd
}

func InitAndExecute() {
	if err := RootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
