package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Retrieve() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "retrieve",
		Short: "retrieve a support bundle from a job in the cluster",
		Long:  `...`,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlag("collectors", cmd.Flags().Lookup("collectors"))
			viper.BindPFlag("namespace", cmd.Flags().Lookup("namespace"))
			viper.BindPFlag("kubecontext", cmd.Flags().Lookup("kubecontext"))
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			troubleshootClient, err := createTroubleshootK8sClient()
			if err != nil {
				return err
			}

			collectorJobs, err := troubleshootClient.CollectorJobs(v.GetString("namespace")).List(metav1.ListOptions{})
			if err != nil {
				return err
			}
			var collectorJob *troubleshootv1beta1.CollectorJob
			if v.GetString("collectors") == "" && len(collectorJobs.Items) == 1 {
				collectorJob = &collectorJobs.Items[0]
			} else {
				for _, foundCollectorJob := range collectorJobs.Items {
					if foundCollectorJob.Spec.Collector.Name == v.GetString("collectors") {
						collectorJob = &foundCollectorJob
						break
					}
				}
			}

			if collectorJob == nil {
				fmt.Printf("unable to find collector job\n")
				return errors.New("no collectors")
			}

			fmt.Printf("connecting to collector job %s\n", collectorJob.Name)

			stopChan, err := k8sutil.PortForward(v.GetString("kubecontext"), 8000, 8000, collectorJob.Status.ServerPodNamespace, collectorJob.Status.ServerPodName)
			if err != nil {
				return err
			}

			if err := receiveSupportBundle(collectorJob.Namespace, collectorJob.Name); err != nil {
				return err
			}

			// write a zip

			close(stopChan)
			return nil
		},
	}

	cmd.Flags().String("collectors", "", "name of the collectors to use")
	cmd.Flags().String("namespace", "", "namespace the collectors can be found in")

	cmd.Flags().String("kubecontext", filepath.Join(homeDir(), ".kube", "config"), "the kubecontext to use when connecting")

	viper.BindPFlags(cmd.Flags())

	return cmd
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
