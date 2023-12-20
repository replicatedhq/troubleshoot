package cli

import (
	"fmt"
	"strings"

	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"github.com/replicatedhq/troubleshoot/pkg/oci"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func OciFetchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "oci-fetch [URI]",
		Args:  cobra.MinimumNArgs(1),
		Short: "Fetch a preflight from an OCI registry and print it to standard out",
		PreRun: func(cmd *cobra.Command, args []string) {
			v := viper.GetViper()
			v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
			v.BindPFlags(cmd.Flags())

			logger.SetupLogger(v)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			uri := args[0]
			data, err := oci.PullPreflightFromOCI(uri)
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		},
	}

	// Initialize klog flags
	logger.InitKlogFlags(cmd)

	return cmd
}
