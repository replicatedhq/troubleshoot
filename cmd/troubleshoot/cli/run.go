package cli

import (
	"path/filepath"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func Run() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "run a support bundle in a cluster",
		Long: `create a new support bundle in a cluster

For example:

troubleshoot run --collectors application --wait
		`,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			if len(args) == 0 {
				return runTroubleshootCRD(v)
			}

			return runTroubleshootNoCRD(v, args[0])
		},
	}

	cmd.Flags().String("collectors", "", "name of the collectors to use")
	cmd.Flags().String("namespace", "default", "namespace the collectors can be found in")

	cmd.Flags().String("kubecontext", filepath.Join(homeDir(), ".kube", "config"), "the kubecontext to use when connecting")

	cmd.Flags().String("image", "", "the full name of the collector image to use")
	cmd.Flags().String("pullpolicy", "", "the pull policy of the collector image")
	cmd.Flags().Bool("redact", true, "enable/disable default redactions")

	cmd.Flags().String("serviceaccount", "", "name of the service account to use. if not provided, one will be created")
	viper.BindPFlags(cmd.Flags())

	return cmd
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
