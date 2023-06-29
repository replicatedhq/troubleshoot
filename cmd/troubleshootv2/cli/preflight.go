package cli

import (
	"github.com/spf13/cobra"
)

func PreflightCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preflight",
		Short: "Runs preflight checks",
		Long:  "Runs preflight checks defined in a troubleshoot spec against a cluster or node.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	return cmd
}
