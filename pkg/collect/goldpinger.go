package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/internal/util"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
			errMsg := fmt.Sprintf("Failed to query goldpinger endpoint in cluster: %v", err)
			klog.V(2).Infof(errMsg)
			err = output.SaveResult(c.BundlePath, "goldpinger/error.txt", bytes.NewBuffer([]byte(errMsg)))
			return output, err
		}
	} else {
		klog.V(2).Infof("Launch pod to query goldpinger endpoint then collect results from pod logs")
		results, err = c.runPodAndCollectGPResults(progressChan)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to run pod to collect goldpinger results: %v", err)
			klog.V(2).Infof(errMsg)
			err = output.SaveResult(c.BundlePath, "goldpinger/error.txt", bytes.NewBuffer([]byte(errMsg)))
			return output, err
		}
	}

	err = output.SaveResult(c.BundlePath, constants.GP_CHECK_ALL_RESULTS_PATH, bytes.NewBuffer(results))
	return output, err
}

func (c *CollectGoldpinger) fetchCheckAllOutput() ([]byte, error) {
	client := &http.Client{
		Timeout: time.Minute, // Long enough timeout
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

func (c *CollectGoldpinger) runPodAndCollectGPResults(progressChan chan<- interface{}) ([]byte, error) {
	rest.InClusterConfig()

	namespace := "default"
	serviceAccountName := ""
	image := constants.GP_DEFAULT_IMAGE
	var imagePullSecret *troubleshootv1beta2.ImagePullSecrets

	if c.Collector.PodLaunchOptions != nil {
		if c.Collector.PodLaunchOptions.Namespace != "" {
			namespace = c.Collector.PodLaunchOptions.Namespace
		}

		if c.Collector.PodLaunchOptions.ServiceAccountName != "" {
			serviceAccountName = c.Collector.PodLaunchOptions.ServiceAccountName
			if err := checkForExistingServiceAccount(c.Context, c.Client, namespace, serviceAccountName); err != nil {
				return nil, err
			}
		}

		if c.Collector.PodLaunchOptions.Image != "" {
			image = c.Collector.PodLaunchOptions.Image
		}
		imagePullSecret = c.Collector.PodLaunchOptions.ImagePullSecret
	}

	runPodCollectorName := "ts-goldpinger-collector"
	collectorContainerName := "collector"
	runPodSpec := &troubleshootv1beta2.RunPod{
		CollectorMeta: troubleshootv1beta2.CollectorMeta{
			CollectorName: runPodCollectorName,
		},
		Name:            runPodCollectorName,
		Namespace:       namespace,
		Timeout:         time.Minute.String(),
		ImagePullSecret: imagePullSecret,
		PodSpec: corev1.PodSpec{
			RestartPolicy:      corev1.RestartPolicyNever,
			ServiceAccountName: serviceAccountName,
			Containers: []corev1.Container{
				{
					Image:           image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Name:            collectorContainerName,
					Command:         []string{"wget"},
					Args:            []string{"-q", "-O-", c.endpoint()},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("50m"),
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
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

	// Check if the collector container exited with an error
	var pod corev1.Pod
	err = json.Unmarshal(output[fmt.Sprintf("%s/%s.json", runPodCollectorName, runPodCollectorName)], &pod)
	if err != nil {
		return nil, err
	}

	var terminationError *corev1.ContainerStateTerminated
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == collectorContainerName && status.State.Terminated != nil {
			if status.State.Terminated.ExitCode != 0 {
				terminationError = status.State.Terminated
			}
		}
	}

	podLogs := output[fmt.Sprintf("%s/%s.log", runPodCollectorName, runPodCollectorName)]
	if terminationError != nil {
		m := map[string]string{
			"podName":  pod.Name,
			"exitCode": strconv.Itoa(int(terminationError.ExitCode)),
			"reason":   terminationError.Reason,
			"message":  terminationError.Message,
			"logs":     string(podLogs),
		}

		b, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			return nil, err
		}
		return nil, errors.New(string(b))
	}
	return podLogs, nil
}

func (c *CollectGoldpinger) endpoint() string {
	namespace := c.Collector.Namespace
	if namespace == "" {
		namespace = constants.GP_DEFAULT_NAMESPACE
	}

	return fmt.Sprintf("http://goldpinger.%s.svc.cluster.local:80/check_all", namespace)
}
