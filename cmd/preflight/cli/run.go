package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

			troubleshootClient, err := createTroubleshootK8sClient()
			if err != nil {
				return err
			}

			preflightName := v.GetString("preflight")
			if preflightName == "" {
				preflights, err := troubleshootClient.Preflights(v.GetString("namespace")).List(metav1.ListOptions{})
				if err != nil {
					return err
				}

				if len(preflights.Items) == 1 {
					preflightName = preflights.Items[0].Name
				}
			}

			if preflightName == "" {
				return errors.New("unable to fly, try using the --preflight flags")
			}

			// generate a unique name
			now := time.Now()
			suffix := fmt.Sprintf("%d", now.Unix())

			preflightJobName := fmt.Sprintf("%s-job-%s", preflightName, suffix[len(suffix)-4:])
			preflightJob := troubleshootv1beta1.PreflightJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      preflightJobName,
					Namespace: v.GetString("namespace"),
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "preflightjob.troubleshoot.replicated.com",
				},
				Spec: troubleshootv1beta1.PreflightJobSpec{
					Preflight: troubleshootv1beta1.PreflightRef{
						Name:      preflightName,
						Namespace: v.GetString("namespace"),
					},
					Image:                    v.GetString("image"),
					ImagePullPolicy:          v.GetString("pullpolicy"),
					CollectorImage:           v.GetString("collector-image"),
					CollectorImagePullPolicy: v.GetString("collector-pullpolicy"),
				},
			}
			if _, err := troubleshootClient.PreflightJobs(v.GetString("namespace")).Create(&preflightJob); err != nil {
				return err
			}

			// Poll the status of the Custom Resource for it to include a callback
			var found *troubleshootv1beta1.PreflightJob
			start := time.Now()
			for {
				current, err := troubleshootClient.PreflightJobs(v.GetString("namespace")).Get(preflightJobName, metav1.GetOptions{})
				if err != nil && kuberneteserrors.IsNotFound(err) {
					continue
				} else if err != nil {
					return err
				}

				if current.Status.IsServerReady {
					found = current
					break
				}

				if time.Now().Sub(start) > time.Duration(time.Second*10) {
					return errors.New("preflightjob failed to start")
				}

				time.Sleep(time.Millisecond * 200)
			}

			// Connect to the callback
			stopChan, err := k8sutil.PortForward(v.GetString("kubecontext"), 8000, 8000, found.Status.ServerPodNamespace, found.Status.ServerPodName)
			if err != nil {
				return err
			}

			// if err := receiveSupportBundle(found.Namespace, found.Name); err != nil {
			// 	return err
			// }

			// Write

			close(stopChan)
			return nil
		},
	}

	cmd.Flags().String("preflight", "", "name of the preflight to use")
	cmd.Flags().String("namespace", "default", "namespace the preflight can be found in")

	cmd.Flags().String("kubecontext", filepath.Join(homeDir(), ".kube", "config"), "the kubecontext to use when connecting")

	cmd.Flags().String("image", "", "the full name of the preflight image to use")
	cmd.Flags().String("pullpolicy", "", "the pull policy of the preflight image")
	cmd.Flags().String("collector-image", "", "the full name of the collector image to use")
	cmd.Flags().String("collector-pullpolicy", "", "the pull policy of the collector image")
	cmd.Flags().String("analyzer-image", "", "the full name of the analyzer image to use")
	cmd.Flags().String("analyzer-pullpolicy", "", "the pull policy of the analyzer image")

	viper.BindPFlags(cmd.Flags())

	return cmd
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
