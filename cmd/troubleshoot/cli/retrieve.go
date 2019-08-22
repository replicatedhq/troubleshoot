package cli

import (
	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
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
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			troubleshootClient, err := createTroubleshootK8sClient(KubernetesConfigFlags)
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
				logger.Printf("unable to find collector job\n")
				return errors.New("no collectors")
			}

			logger.Printf("connecting to collector job %s\n", collectorJob.Name)

			config, err := KubernetesConfigFlags.ToRESTConfig()
			if err != nil {
				return errors.Wrap(err, "failed to convert kube flags to rest config")
			}

			stopChan, err := k8sutil.PortForward(config, 8000, 8000, collectorJob.Status.ServerPodNamespace, collectorJob.Status.ServerPodName)
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

	viper.BindPFlags(cmd.Flags())

	return cmd
}
