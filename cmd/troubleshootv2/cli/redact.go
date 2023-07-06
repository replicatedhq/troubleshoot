package cli

import (
	"github.com/spf13/cobra"
)

func RedactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "redact",
		Short: "Redacts an existing support bundle given a troubleshoot spec with redactors",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	return cmd
}
