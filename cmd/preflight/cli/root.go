package cli

import (
	"fmt"
	"os"
	"strconv"
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

// Hacky way of passing the exit code up the stack
// Ideally I'd pass a custom error struct through, but I can't get cobra.Command to accept one at the moment
// So we can pass any error + the exit code up the stack
func wrapExitCodeInError(theErr error, exitCode int) error {
	useErr := ""
	if theErr != nil {
		useErr = theErr.Error()
	}

	return fmt.Errorf("%d:-:-:-:%s", exitCode, useErr)
}

// Returns error (did unwrap succeed), error (the unwrapped error), int (exit code)
// TODOLATER: consolidate the 2 error responses into 1? any downsides?
func unwrapExitCodeFromError(inputErr error) (error, error, int) {
	splitErr := strings.Split(inputErr.Error(), ":-:-:-:")
	if len(splitErr) != 2 {
		return fmt.Errorf("Invalid error input, cannot unwrap exit code - %s", inputErr), fmt.Errorf("ERROR"), 1
	}

	exitCode, err := strconv.Atoi(splitErr[0])
	if err != nil {
		return err, fmt.Errorf("ERROR"), 1
	}

	var unwrappedErr error
	if len(splitErr[1]) > 0 {
		unwrappedErr = fmt.Errorf(splitErr[1])
	}

	return nil, unwrappedErr, exitCode
}

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preflight [url]",
		Args:  cobra.MinimumNArgs(1),
		Short: "Run and retrieve preflight checks in a cluster",
		Long: `A preflight check is a set of validations that can and should be run to ensure
that a cluster meets the requirements to run an application.`,
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

			return wrapExitCodeInError(err, exitCode)
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
	errAndExitCode := RootCmd().Execute()

	err, unwrappedErr, exitCode := unwrapExitCodeFromError(errAndExitCode)
	if err != nil {
		print(err)
		os.Exit(1)
	}

	if unwrappedErr != nil {
		print(unwrappedErr)
		os.Exit(1)
	}

	os.Exit(exitCode)
}

func initConfig() {
	viper.SetEnvPrefix("PREFLIGHT")
	viper.AutomaticEnv()
}
