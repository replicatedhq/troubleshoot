package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/replicatedhq/troubleshoot/pkg/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func Server() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "server",
		Short:  "run the http server",
		Hidden: true,
		Long:   `...`,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlag("port", cmd.Flags().Lookup("port"))
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			server.ServePreflight(context.Background(), fmt.Sprintf(":%d", v.GetInt("port")))

			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt)

			select {
			case <-c:
				return nil
			}
		},
	}

	cmd.Flags().Int("port", 8000, "port to listen on")

	viper.BindPFlags(cmd.Flags())

	return cmd
}
