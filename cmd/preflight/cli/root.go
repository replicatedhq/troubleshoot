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
	"github.com/replicatedhq/troubleshoot/pkg/updater"
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

			// Auto-update preflight unless disabled by flag or env
			envAuto := os.Getenv("PREFLIGHT_AUTO_UPDATE")
			autoFromEnv := true
			if envAuto != "" {
				if strings.EqualFold(envAuto, "0") || strings.EqualFold(envAuto, "false") {
					autoFromEnv = false
				}
			}
			if v.GetBool("auto-update") && autoFromEnv {
				exe, err := os.Executable()
				if err == nil {
					_ = updater.CheckAndUpdate(cmd.Context(), updater.Options{
						BinaryName:  "preflight",
						CurrentPath: exe,
						Printf:      func(f string, a ...interface{}) { klog.V(1).Infof(f, a...) },
					})
				}
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

			err = preflight.RunPreflights(v.GetBool("interactive"), v.GetString("output"), v.GetString("format"), args)
			if !v.GetBool("dry-run") && (v.GetBool("debug") || v.IsSet("v")) {
				fmt.Fprintf(os.Stderr, "\n%s", traces.GetExporterInstance().GetSummary())
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

	cmd.AddCommand(util.VersionCmd())
	cmd.AddCommand(OciFetchCmd())
	cmd.AddCommand(TemplateCmd())
	cmd.AddCommand(DocsCmd())
	cmd.AddCommand(ConvertCmd())

	preflight.AddFlags(cmd.PersistentFlags())

	// Dry run flag should be in cmd.PersistentFlags() flags made available to all subcommands
	// Adding here to avoid that
	cmd.Flags().Bool("dry-run", false, "print the preflight spec without running preflight checks")
	cmd.Flags().Bool("no-uri", false, "When this flag is used, Preflight does not attempt to retrieve the spec referenced by the uri: field`")
	cmd.Flags().Bool("auto-update", true, "enable automatic binary self-update check and install")

	// Template values for v1beta3 specs
	cmd.Flags().StringSlice("values", []string{}, "Path to YAML files containing template values for v1beta3 specs (can be used multiple times)")
	cmd.Flags().StringSlice("set", []string{}, "Set template values on the command line for v1beta3 specs (can be used multiple times)")

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
	viper.SetEnvPrefix("PREFLIGHT")
	viper.AutomaticEnv()
}
