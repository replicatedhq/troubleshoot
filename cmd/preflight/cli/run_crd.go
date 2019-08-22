package cli

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/spf13/viper"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func runPreflightsCRD(v *viper.Viper) error {
	troubleshootClient, err := createTroubleshootK8sClient(KubernetesConfigFlags)
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
		return errors.New("unable to preflight, try using the --preflight flags")
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

	config, err := KubernetesConfigFlags.ToRESTConfig()
	if err != nil {
		return errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	stopChan, err := k8sutil.PortForward(config, 8000, 8000, found.Status.ServerPodNamespace, found.Status.ServerPodName)
	if err != nil {
		return err
	}

	if err := receivePreflightResults(found.Namespace, found.Name); err != nil {
		return err
	}

	// Write

	close(stopChan)
	return nil
}
