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
		Short: "run a set of preflight checks in a cluster",
		Long:  `run preflight checks and return the results`,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			if len(args) == 0 {
				return runPreflightsCRD(v)
			}

			return runPreflightsNoCRD(v, args[0])
		},
	}

	cmd.Flags().Bool("interactive", true, "interactive preflights")
	cmd.Flags().String("format", "human", "output format, one of human, json, yaml. only used when interactive is set to false")

	cmd.Flags().String("preflight", "", "name of the preflight to use")
	cmd.Flags().String("namespace", "default", "namespace the preflight can be found in")

	cmd.Flags().String("kubecontext", filepath.Join(homeDir(), ".kube", "config"), "the kubecontext to use when connecting")

	cmd.Flags().String("image", "", "the full name of the preflight image to use")
	cmd.Flags().String("pullpolicy", "", "the pull policy of the preflight image")
	cmd.Flags().String("collector-image", "", "the full name of the collector image to use")
	cmd.Flags().String("collector-pullpolicy", "", "the pull policy of the collector image")

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
