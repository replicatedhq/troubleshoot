package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/replicatedhq/troubleshoot/cmd/util"
	"github.com/replicatedhq/troubleshoot/internal/traces"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"github.com/replicatedhq/troubleshoot/pkg/preflight"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

func InitAndExecute() {
	exitCode := 0

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

			logger.SetupLogger(v)

			if err := util.StartProfiling(); err != nil {
				klog.Errorf("Failed to start profiling: %v", err)
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()
			closer, err := traces.ConfigureTracing("preflight")
			if err != nil {
				// Do not fail running preflights if tracing fails
				klog.Errorf("Failed to initialize open tracing provider: %v", err)
			} else {
				defer closer()
			}

			// NOTE: exitCode is a variable defined outside of this function
			// RunE can only propagate an error type back up the stack, we need the exit code also...
			exitCode, err = preflight.RunPreflights(v.GetBool("interactive"), v.GetString("output"), v.GetString("format"), args)
			if v.GetBool("debug") || v.IsSet("v") {
				fmt.Printf("\n%s", traces.GetExporterInstance().GetSummary())
			}

			return err
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			if err := util.StopProfiling(); err != nil {
				klog.Errorf("Failed to stop profiling: %v", err)
			}
		},
	}

	cobra.OnInitialize(initConfig)

	cmd.AddCommand(VersionCmd())
	preflight.AddFlags(cmd.PersistentFlags())

	k8sutil.AddFlags(cmd.Flags())

	// Initialize klog flags
	logger.InitKlogFlags(cmd)

	// CPU and memory profiling flags
	util.AddProfilingFlags(cmd)

	var err error
	err = cmd.Execute()

	// We can't just exit inside the command RunE, as we need to ensure the PostRun's are handled
	if err != nil {
		os.Exit(1)
	} else {
		os.Exit(exitCode)
	}
}

func initConfig() {
	viper.SetEnvPrefix("PREFLIGHT")
	viper.AutomaticEnv()
}
