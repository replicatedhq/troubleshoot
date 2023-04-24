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

// So we can pass any error + the exit code up the stack
type exitCodeAndError struct {
	Code int
	Err  error
}

func (e *exitCodeAndError) Error() string {
	return e.Err.Error()
}

func (e *exitCodeAndError) ExitCode() int {
	return int(e.Code)
}

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

			logger.SetupLogger(v)

			if err := util.StartProfiling(); err != nil {
				klog.Errorf("Failed to start profiling: %v", err)
			}
		},
		RunE: func(cmd *cobra.Command, args []string) exitCodeAndError {
			v := viper.GetViper()
			closer, err := traces.ConfigureTracing("preflight")
			if err != nil {
				// Do not fail running preflights if tracing fails
				klog.Errorf("Failed to initialize open tracing provider: %v", err)
			} else {
				defer closer()
			}

			exitCode, err := preflight.RunPreflights(v.GetBool("interactive"), v.GetString("output"), v.GetString("format"), args)
			if v.GetBool("debug") || v.IsSet("v") {
				fmt.Printf("\n%s", traces.GetExporterInstance().GetSummary())
			}

			return &exitCodeAndError{
				Code: exitCode,
				Err:  err,
			}
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			if err := util.StopProfiling(); err != nil {
				klog.Errorf("Failed to stop profiling: %v", err)
			}
		},
	}

	cobra.OnInitialize(initConfig)

	cmd.AddCommand(VersionCmd())
	cmd.AddCommand(OciFetchCmd())
	preflight.AddFlags(cmd.PersistentFlags())

	k8sutil.AddFlags(cmd.Flags())

	// Initialize klog flags
	logger.InitKlogFlags(cmd)

	// CPU and memory profiling flags
	util.AddProfilingFlags(cmd)

	return cmd
}

func InitAndExecute() {
	err := RootCmd().Execute()

	if err != nil {
		os.Exit(1)
	} else {
		os.Exit(err.ExitCode())
	}
}

func initConfig() {
	viper.SetEnvPrefix("PREFLIGHT")
	viper.AutomaticEnv()
}
