package cli

import (
	"github.com/spf13/cobra"
)

func AnalyzeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyse an existing support bundle given a troubleshoot spec with analysers",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	return cmd
}
