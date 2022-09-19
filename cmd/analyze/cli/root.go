package cli

import (
	"os"
	"strings"

	"github.com/go-logr/logr"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "analyze [url]",
		Args:         cobra.MinimumNArgs(1),
		Short:        "Analyze a support bundle",
		Long:         `Run a series of analyzers on a support bundle archive`,
		SilenceUsage: true,
		PreRun: func(cmd *cobra.Command, args []string) {
			v := viper.GetViper()
			v.BindPFlags(cmd.Flags())

			if !v.GetBool("debug") {
				klog.SetLogger(logr.Discard())
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			logger.SetQuiet(v.GetBool("quiet"))

			return runAnalyzers(v, args[0])
		},
	}

	cobra.OnInitialize(initConfig)

	cmd.Flags().String("analyzers", "", "filename or url of the analyzers to use")
	cmd.Flags().Bool("debug", false, "enable debug logging")

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
