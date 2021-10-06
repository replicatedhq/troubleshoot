package cli

import (
	"fmt"

	"github.com/replicatedhq/troubleshoot/pkg/version"
	"github.com/spf13/cobra"
)

func VersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the current version and exit",
		Long:  `Print the current version and exit`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Replicated Collect %s\n", version.Version())

			return nil
		},
	}
	return cmd
}
