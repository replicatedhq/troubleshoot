package cli

import (
	"os"
	"strings"

	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preflight [url]",
		Args:  cobra.MinimumNArgs(1),
		Short: "Run and retrieve preflight checks in a cluster",
		Long: `A preflight check is a set of validations that can and should be run to ensure
that a cluster meets the requirements to run an application.`,
		SilenceUsage: true,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()
			return runPreflights(v, args[0])
		},
	}

	cobra.OnInitialize(initConfig)

	cmd.AddCommand(VersionCmd())

	cmd.Flags().Bool("interactive", true, "interactive preflights")
	cmd.Flags().String("format", "human", "output format, one of human, json, yaml. only used when interactive is set to false")
	cmd.Flags().String("collector-image", "", "the full name of the collector image to use")
	cmd.Flags().String("collector-pullpolicy", "", "the pull policy of the collector image")
	cmd.Flags().Bool("collect-without-permissions", true, "always run preflight checks even if some require permissions that preflight does not have")
	cmd.Flags().String("selector", "", "selector (label query) to filter remote collection nodes on.")
	cmd.Flags().String("since-time", "", "force pod logs collectors to return logs after a specific date (RFC3339)")
	cmd.Flags().String("since", "", "force pod logs collectors to return logs newer than a relative duration like 5s, 2m, or 3h.")

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	k8sutil.AddFlags(cmd.Flags())

	return cmd
}

func InitAndExecute() {
	if err := RootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func initConfig() {
	viper.SetEnvPrefix("PREFLIGHT")
	viper.AutomaticEnv()
}
