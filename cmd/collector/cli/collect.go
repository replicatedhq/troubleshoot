package cli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func Server() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "start the collector server",
		Long:  `...`,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlag("port", cmd.Flags().Lookup("port"))
		},
		RunE: func(cmd *cobra.Command, args []string) error {

			return nil
		},
	}

	cmd.Flags().Int("port", 8000, "port to bind to")

	viper.BindPFlags(cmd.Flags())

	return cmd
}
