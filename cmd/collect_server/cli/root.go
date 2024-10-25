package cli

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/replicatedhq/troubleshoot/cmd/internal/util"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

var chroot string

func checkAndSetupChroot(path string) error {
	if path == "" {
		return nil
	}
	fmt.Println("chrooting to: " + path)
	return syscall.Chroot(path)
}

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "collect_server",
		Short:        "Run a collector",
		Long:         `Run a collector and output the results.`,
		SilenceUsage: true,
		PreRun: func(cmd *cobra.Command, args []string) {
			v := viper.GetViper()
			v.BindPFlags(cmd.Flags())

			logger.SetupLogger(v)

			chroot := viper.GetString("chroot")
			if err := checkAndSetupChroot(chroot); err != nil {
				panic(err)
			}

			if err := util.StartProfiling(); err != nil {
				klog.Errorf("Failed to start profiling: %v", err)
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			return collectServer(v)
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			if err := util.StopProfiling(); err != nil {
				klog.Errorf("Failed to stop profiling: %v", err)
			}
		},
	}

	cobra.OnInitialize(initConfig)

	cmd.AddCommand(util.VersionCmd())

	cmd.Flags().StringSlice("redactors", []string{}, "names of the additional redactors to use")
	cmd.Flags().Bool("redact", true, "enable/disable default redactions")
	cmd.Flags().String("format", "json", "output format, one of json or raw.")
	cmd.Flags().String("collector-image", "", "the full name of the collector image to use")
	cmd.Flags().String("collector-pull-policy", "", "the pull policy of the collector image")
	cmd.Flags().String("selector", "", "selector (label query) to filter remote collection nodes on.")
	cmd.Flags().Bool("collect-without-permissions", false, "always generate a support bundle, even if it some require additional permissions")
	cmd.Flags().Bool("debug", false, "enable debug logging")
	cmd.Flags().String("chroot", "", "chroot to path")

	// hidden in favor of the `insecure-skip-tls-verify` flag
	cmd.Flags().Bool("allow-insecure-connections", false, "when set, do not verify TLS certs when retrieving spec and reporting results")
	cmd.Flags().MarkHidden("allow-insecure-connections")

	viper.BindPFlags(cmd.Flags())
	viper.SetEnvPrefix("collect_server")
	viper.BindEnv("chroot")

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	k8sutil.AddFlags(cmd.Flags())

	// Initialize klog flags
	logger.InitKlogFlags(cmd)

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
	viper.SetEnvPrefix("TROUBLESHOOT")
	viper.AutomaticEnv()
}
