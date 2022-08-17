package cli

import (
	"io/ioutil"
	"os"
	"strings"

	"github.com/go-logr/logr"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "support-bundle [url]",
		Args:  cobra.MinimumNArgs(1),
		Short: "Generate a support bundle",
		Long: `A support bundle is an archive of files, output, metrics and state
from a server that can be used to assist when troubleshooting a Kubernetes cluster.`,
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
			return runTroubleshoot(v, args[0])
		},
	}

	cobra.OnInitialize(initConfig)

	cmd.AddCommand(Analyze())
	cmd.AddCommand(VersionCmd())

	cmd.Flags().StringSlice("redactors", []string{}, "names of the additional redactors to use")
	cmd.Flags().Bool("redact", true, "enable/disable default redactions")
	cmd.Flags().Bool("interactive", true, "enable/disable interactive mode")
	cmd.Flags().Bool("collect-without-permissions", true, "always generate a support bundle, even if it some require additional permissions")
	cmd.Flags().String("since-time", "", "force pod logs collectors to return logs after a specific date (RFC3339)")
	cmd.Flags().String("since", "", "force pod logs collectors to return logs newer than a relative duration like 5s, 2m, or 3h.")
	cmd.Flags().StringP("output", "o", "", "specify the output file path for the support bundle")
	cmd.Flags().Bool("debug", false, "enable debug logging")
	cmd.Flags().StringP("other-spec", "", "specify the output file path for a spec to merge")

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

func ensureCollectorInList(list []*troubleshootv1beta2.Collect, collector troubleshootv1beta2.Collect) []*troubleshootv1beta2.Collect {
	for _, inList := range list {
		if collector.ClusterResources != nil && inList.ClusterResources != nil {
			return list
		}
		if collector.ClusterInfo != nil && inList.ClusterInfo != nil {
			return list
		}
	}

	return append(list, &collector)
}

func writeFile(filename string, contents []byte) error {
	if err := ioutil.WriteFile(filename, contents, 0644); err != nil {
		return err
	}

	return nil
}
