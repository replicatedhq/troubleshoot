package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/replicatedhq/troubleshoot/internal/util"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

// Collect goldpinger results from goldpinger service running in a cluster
// The results are stored in goldpinger/check_all.json since we use
// the /check_all endpoint
type CollectGoldpinger struct {
	Collector    *troubleshootv1beta2.Goldpinger
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectGoldpinger) Title() string {
	return getCollectorName(c)
}

func (c *CollectGoldpinger) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectGoldpinger) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	output := NewResult()
	var results []byte
	var err error

	if util.IsInCluster() {
		klog.V(2).Infof("Collector running in cluster, querying goldpinger endpoint straight away")
		results, err = c.fetchCheckAllOutput()
		if err != nil {
			klog.V(2).Infof("Failed to query goldpinger endpoint: %v", err)
			err = output.SaveResult(c.BundlePath, "goldpinger/error.txt", bytes.NewBuffer([]byte(err.Error())))
			return output, err
		}
	} else {
		klog.V(2).Infof("Launch pod to query goldpinger endpoint then collect results from pod logs")
		results, err = c.runPodAndCollectCheckOutput(progressChan)
		if err != nil {
			klog.V(2).Infof("Failed to run pod and collect goldpinger results: %v", err)
			err = output.SaveResult(c.BundlePath, "goldpinger/error.txt", bytes.NewBuffer([]byte(err.Error())))
			return output, err
		}
	}

	err = output.SaveResult(c.BundlePath, constants.GP_CHECK_ALL_RESULTS_PATH, bytes.NewBuffer(results))
	return output, err
}

func (c *CollectGoldpinger) fetchCheckAllOutput() ([]byte, error) {
	client := &http.Client{
		Timeout: 60 * time.Second, // Long enough timeout
	}

	req, err := http.NewRequestWithContext(c.Context, "GET", c.endpoint(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func (c *CollectGoldpinger) runPodAndCollectCheckOutput(progressChan chan<- interface{}) ([]byte, error) {
	namespace := "default"
	if c.Collector.PodLaunchSpec.Namespace != "" {
		namespace = c.Collector.PodLaunchSpec.Namespace
	}

	serviceAccountName := "default"
	if c.Collector.PodLaunchSpec.ServiceAccountName != "" {
		serviceAccountName = c.Collector.PodLaunchSpec.ServiceAccountName
	}

	if err := checkForExistingServiceAccount(c.Context, c.Client, namespace, serviceAccountName); err != nil {
		return nil, err
	}

	image := "alpine:3" // TODO: Will this image always be in airgaps? Perhaps netshoot?
	if c.Collector.PodLaunchSpec.Image != "" {
		image = c.Collector.PodLaunchSpec.Image
	}

	runPodCollectorName := "ts-goldpinger-collector"
	wgetContainerName := "wget-collector"
	runPodSpec := &troubleshootv1beta2.RunPod{
		CollectorMeta: troubleshootv1beta2.CollectorMeta{
			CollectorName: runPodCollectorName,
		},
		Name:            runPodCollectorName,
		Namespace:       namespace,
		Timeout:         c.Collector.PodLaunchSpec.Timeout,
		ImagePullSecret: c.Collector.PodLaunchSpec.ImagePullSecret,
		// TODO: Lets pass the pod spec in the collector spec
		PodSpec: corev1.PodSpec{
			RestartPolicy:      corev1.RestartPolicyNever,
			ServiceAccountName: serviceAccountName,
			Containers: []corev1.Container{
				{
					Image:           image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Name:            wgetContainerName,
					Command:         []string{"wget"},
					Args:            []string{"-q", "-O-", c.endpoint()},
				},
			},
		},
	}

	rbacErrors := c.GetRBACErrors()
	// Pass an empty bundle path since we don't need to save the results
	runPodCollector := &CollectRunPod{runPodSpec, "", c.Namespace, c.ClientConfig, c.Client, c.Context, rbacErrors}

	output, err := runPodCollector.Collect(progressChan)
	if err != nil {
		return nil, err
	}

	// Check if the wget container exited with an error
	var pod corev1.Pod
	err = json.Unmarshal(output[fmt.Sprintf("%s/%s.json", runPodCollectorName, runPodCollectorName)], &pod)
	if err != nil {
		return nil, err
	}

	exitedWithError := false
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == wgetContainerName {
			if status.State.Terminated.ExitCode != 0 {
				exitedWithError = true
			}
		}
	}

	podLogs := output[fmt.Sprintf("%s/%s.log", runPodCollectorName, runPodCollectorName)]
	if exitedWithError {
		return nil, fmt.Errorf("wget container exited with an error: %q", string(podLogs))
	}
	return podLogs, nil
}

func (c *CollectGoldpinger) endpoint() string {
	namespace := c.Collector.Namespace
	if namespace == "" {
		namespace = "kurl"
	}

	return fmt.Sprintf("http://goldpinger.%s.svc.cluster.local:80/check_all", namespace)
}
