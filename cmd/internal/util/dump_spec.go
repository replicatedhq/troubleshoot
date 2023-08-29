package util

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/internal/specs"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"
)

func PrintSpecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dump [URI]",
		Args:  cobra.MinimumNArgs(0),
		Short: "Print loaded specs to stdout",
		RunE: func(cmd *cobra.Command, args []string) error {
			return printSpecs(args)
		},
	}

	cmd.Flags().StringSliceP("selector", "l", []string{"troubleshoot.sh/kind=support-bundle"}, "selector to filter on for loading additional support bundle specs found in secrets within the cluster")
	cmd.Flags().Bool("load-cluster-specs", false, "enable/disable loading additional troubleshoot specs found within the cluster. required when no specs are provided on the command line")

	// Initialize klog flags
	logger.InitKlogFlags(cmd)

	k8sutil.AddFlags(cmd.Flags())

	return cmd
}

func printSpecs(args []string) error {
	config, err := k8sutil.GetRESTConfig()
	if err != nil {
		return errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return errors.Wrap(err, "failed to convert create k8s client")
	}

	ctx := context.Background()
	kinds, err := specs.LoadFromCLIArgs(ctx, client, args, viper.GetViper())
	if err != nil {
		return err
	}

	// TODO: Considerations
	// - Apply merge logic to all specs and print the merged spec
	// - Conside command that called this function i.e preflight, support-bundle etc and print selected spec
	//   This will mean adding util functions that remove unwanted specs from the kinds object
	asYaml, err := kinds.ToYaml()
	if err != nil {
		return err
	}

	fmt.Println(asYaml)

	return nil
}
