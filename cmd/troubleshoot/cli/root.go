package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "troubleshoot [url]",
		Args:  cobra.MinimumNArgs(1),
		Short: "Generate and manage support bundles",
		Long: `A support bundle is an archive of files, output, metrics and state
from a server that can be used to assist when troubleshooting a server.`,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			logger.SetQuiet(v.GetBool("quiet"))

			if len(args) == 0 {
				return runTroubleshootCRD(v)
			}

			return runTroubleshootNoCRD(v, args[0])
		},
	}

	cobra.OnInitialize(initConfig)

	cmd.AddCommand(Analyze())

	cmd.Flags().String("collectors", "", "name of the collectors to use")
	cmd.Flags().String("namespace", "default", "namespace the collectors can be found in")

	cmd.Flags().String("kubecontext", filepath.Join(homeDir(), ".kube", "config"), "the kubecontext to use when connecting")

	cmd.Flags().String("image", "", "the full name of the collector image to use")
	cmd.Flags().String("pullpolicy", "", "the pull policy of the collector image")
	cmd.Flags().Bool("redact", true, "enable/disable default redactions")

	cmd.Flags().String("serviceaccount", "", "name of the service account to use. if not provided, one will be created")
	viper.BindPFlags(cmd.Flags())

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	return cmd
}

func InitAndExecute() {
	if err := RootCmd().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func initConfig() {
	viper.SetEnvPrefix("TROUBLESHOOT")
	viper.AutomaticEnv()
}

func ensureCollectorInList(list []*troubleshootv1beta1.Collect, collector troubleshootv1beta1.Collect) []*troubleshootv1beta1.Collect {
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
