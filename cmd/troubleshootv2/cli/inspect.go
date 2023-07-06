package cli

import (
	"github.com/spf13/cobra"
)

func InspectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Launches a k8s API server from an existing support bundle which one can inspect using kubectl",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	return cmd
}
