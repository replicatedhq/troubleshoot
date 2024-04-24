package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/replicatedhq/troubleshoot/cmd/internal/util"
	"github.com/replicatedhq/troubleshoot/internal/traces"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"github.com/replicatedhq/troubleshoot/pkg/preflight"
	"github.com/replicatedhq/troubleshoot/pkg/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "lint",
		Args:          cobra.MinimumNArgs(1),
		Short:         "Linting troubleshoot specs",
		Long:          "Lint specs against troubleshoot schemas that run a valid check result",
		SilenceUsage:  true,
		SilenceErrors: true,
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
			closer, err := traces.ConfigureTracing("lint")
			if err != nil {
				// Do not fail running lints if tracing fails
				klog.Errorf("Failed to initialize open tracing provider: %v", err)
			} else {
				defer closer()
			}
			fmt.Println(v)
			return err
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			if err := util.StopProfiling(); err != nil {
				klog.Errorf("Failed to stop profiling: %v", err)
			}
		},
	}

	cobra.OnInitialize(initConfig)

	cmd.AddCommand(util.VersionCmd())
	preflight.AddFlags(cmd.PersistentFlags())

	// Dry run flag should be in cmd.PersistentFlags() flags made available to all subcommands
	// Adding here to avoid that
	cmd.Flags().Bool("dry-run", false, "print the preflight spec without running preflight checks")

	k8sutil.AddFlags(cmd.Flags())

	// Initialize klog flags
	logger.InitKlogFlags(cmd)

	// CPU and memory profiling flags
	util.AddProfilingFlags(cmd)

	return cmd
}

func InitAndExecute() {
	cmd := RootCmd()
	err := cmd.Execute()

	if err != nil {
		var exitErr types.ExitError
		if errors.As(err, &exitErr) {
			// We need to do this, there's situations where we need the non-zero exit code (which comes as part of the custom error struct)
			// but there's no actual error, just an exit code.
			// If there's also an error to output (eg. invalid format etc) then print it as well
			if exitErr.ExitStatus() != constants.EXIT_CODE_FAIL && exitErr.ExitStatus() != constants.EXIT_CODE_WARN {
				cmd.PrintErrln("Error:", err.Error())
			}

			os.Exit(exitErr.ExitStatus())
		}

		// Fallback, should almost never be used (the above Exit() should handle almost all situations
		cmd.PrintErrln("Error:", err.Error())
		os.Exit(1)
	}
}

func initConfig() {
	viper.SetEnvPrefix("LINT")
	viper.AutomaticEnv()
}
