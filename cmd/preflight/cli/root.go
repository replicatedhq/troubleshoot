package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"github.com/replicatedhq/troubleshoot/cmd/util"
	"github.com/replicatedhq/troubleshoot/internal/traces"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"github.com/replicatedhq/troubleshoot/pkg/preflight"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
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
			v := viper.GetViper()
			v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
			v.BindPFlags(cmd.Flags())

			if !v.GetBool("debug") {
				klog.SetLogger(logr.Discard())
			}

			if err := util.StartProfiling(); err != nil {
				logger.Printf("Failed to start profiling: %v", err)
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()
			closer, err := traces.ConfigureTracing("preflight")
			if err != nil {
				// Do not fail running preflights if tracing fails
				logger.Printf("Failed to initialize open tracing provider: %v", err)
			} else {
				defer closer()
			}

			err = preflight.RunPreflights(v.GetBool("interactive"), v.GetString("output"), v.GetString("format"), args)
			if v.GetBool("debug") {
				fmt.Printf("\n%s", traces.GetExporterInstance().GetSummary())
			}
			return err
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			if err := util.StopProfiling(); err != nil {
				logger.Printf("Failed to stop profiling: %v", err)
			}
		},
	}

	cobra.OnInitialize(initConfig)

	cmd.AddCommand(VersionCmd())
	preflight.AddFlags(cmd.PersistentFlags())

	k8sutil.AddFlags(cmd.Flags())

	// CPU and memory profiling flags
	util.AddProfilingFlags(cmd)

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
