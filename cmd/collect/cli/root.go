package cli

import (
	"os"
	"strings"

	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "collect [url]",
		Args:         cobra.MinimumNArgs(1),
		Short:        "Run a collector",
		Long:         `Run a collector and output the results.`,
		SilenceUsage: true,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			logger.SetQuiet(v.GetBool("quiet"))
			return runCollect(v, args[0])
		},
	}

	cobra.OnInitialize(initConfig)

	cmd.AddCommand(VersionCmd())

	cmd.Flags().StringSlice("redactors", []string{}, "names of the additional redactors to use")
	cmd.Flags().Bool("redact", true, "enable/disable default redactions")
	cmd.Flags().String("format", "json", "output format, one of json or raw.")
	cmd.Flags().String("collector-image", "", "the full name of the collector image to use")
	cmd.Flags().String("collector-pull-policy", "", "the pull policy of the collector image")
	cmd.Flags().String("selector", "", "selector (label query) to filter remote collection nodes on.")
	cmd.Flags().Bool("collect-without-permissions", false, "always generate a support bundle, even if it some require additional permissions")

	// hidden in favor of the `insecure-skip-tls-verify` flag
	cmd.Flags().Bool("allow-insecure-connections", false, "when set, do not verify TLS certs when retrieving spec and reporting results")
	cmd.Flags().MarkHidden("allow-insecure-connections")

	viper.BindPFlags(cmd.Flags())

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
	viper.SetEnvPrefix("TROUBLESHOOT")
	viper.AutomaticEnv()
}
