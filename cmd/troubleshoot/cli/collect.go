package cli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func Collect() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collect",
		Short: "collect a support bundle from a cluster",
		Long:  `...`,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlag("collectors", cmd.Flags().Lookup("collectors"))
			viper.BindPFlag("namespace", cmd.Flags().Lookup("namespace"))
			viper.BindPFlag("kubecontext", cmd.Flags().Lookup("kubecontext"))
		},
		RunE: func(cmd *cobra.Command, args []string) error {

			return nil
		},
	}

	cmd.Flags().String("collectors", "", "name of the collectors to use")
	cmd.Flags().String("namespace", "", "namespace the collectors can be found in")

	cmd.Flags().String("kubecontext", "", "the kubecontext to use when connecting")

	viper.BindPFlags(cmd.Flags())

	return cmd
}
