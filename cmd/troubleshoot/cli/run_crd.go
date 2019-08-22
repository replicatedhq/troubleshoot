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

func runTroubleshootCRD(v *viper.Viper) error {
	troubleshootClient, err := createTroubleshootK8sClient(KubernetesConfigFlags)
	if err != nil {
		return err
	}

	collectorName := v.GetString("collectors")
	if collectorName == "" {
		collectors, err := troubleshootClient.Collectors(v.GetString("namespace")).List(metav1.ListOptions{})
		if err != nil {
			return err
		}

		if len(collectors.Items) == 1 {
			collectorName = collectors.Items[0].Name
		}
	}

	if collectorName == "" {
		return errors.New("unknown collectors, try using the --collectors flags")
	}

	// generate a unique name
	now := time.Now()
	suffix := fmt.Sprintf("%d", now.Unix())

	collectorJobName := fmt.Sprintf("%s-job-%s", collectorName, suffix[len(suffix)-4:])
	collectorJob := troubleshootv1beta1.CollectorJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      collectorJobName,
			Namespace: v.GetString("namespace"),
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "collectorjob.troubleshoot.replicated.com",
		},
		Spec: troubleshootv1beta1.CollectorJobSpec{
			Collector: troubleshootv1beta1.CollectorRef{
				Name:      collectorName,
				Namespace: v.GetString("namespace"),
			},
			Image:           v.GetString("image"),
			ImagePullPolicy: v.GetString("pullpolicy"),
			Redact:          v.GetBool("redact"),
		},
	}
	if _, err := troubleshootClient.CollectorJobs(v.GetString("namespace")).Create(&collectorJob); err != nil {
		return err
	}

	// Poll the status of the Custom Resource for it to include a callback
	var found *troubleshootv1beta1.CollectorJob
	start := time.Now()
	for {
		current, err := troubleshootClient.CollectorJobs(v.GetString("namespace")).Get(collectorJobName, metav1.GetOptions{})
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
			return errors.New("collectorjob failed to start")
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

	if err := receiveSupportBundle(found.Namespace, found.Name); err != nil {
		return err
	}

	// Write

	close(stopChan)
	return nil
}
