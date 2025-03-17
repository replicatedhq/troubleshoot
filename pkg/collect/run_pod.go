package collect

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

type CollectRunPod struct {
	Collector    *troubleshootv1beta2.RunPod
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectRunPod) Title() string {
	return getCollectorName(c)
}

func (c *CollectRunPod) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectRunPod) Collect(progressChan chan<- interface{}) (result CollectorResult, err error) {
	ctx := context.Background()
	result = NewResult()

	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create client from config")
	}

	pod, err := runPodWithSpec(ctx, client, c.Collector)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run pod")
	}
	defer func() {
		deletePod(ctx, client, pod)
	}()

	if c.Collector.ImagePullSecret != nil && c.Collector.ImagePullSecret.Data != nil {
		defer c.deleteImagePullSecret(context.Background(), client, pod)
	}

	defer func() {
		if err != nil {
			return
		}
		result, err = savePodDetails(ctx, client, result, c.BundlePath, c.ClientConfig, pod, c.Collector)
		if err != nil {
			klog.Errorf("failed to save pod details: %v", err)
		}
	}()

	if c.Collector.Timeout == "" {
		return runWithoutTimeout(ctx, c.BundlePath, c.ClientConfig, pod, c.Collector)
	}

	timeout, err := time.ParseDuration(c.Collector.Timeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse timeout")
	}

	errCh := make(chan error, 1)
	resultCh := make(chan CollectorResult, 1)

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// TODO: Use context with timeout instead of go routine
	go func() {
		b, err := runWithoutTimeout(timeoutCtx, c.BundlePath, c.ClientConfig, pod, c.Collector)
		if err != nil {
			errCh <- err
		} else {
			resultCh <- b
		}
	}()

	select {
	case <-time.After(timeout):
		return result, errors.New("timeout")
	case output := <-resultCh:
		result.AddResult(output)
		return result, nil
	case err := <-errCh:
		return result, err
	}
}

func (c *CollectRunPod) deleteImagePullSecret(ctx context.Context, client kubernetes.Interface, pod *corev1.Pod) {
	for _, k := range pod.Spec.ImagePullSecrets {
		secret, err := client.CoreV1().Secrets(pod.Namespace).Get(ctx, k.Name, metav1.GetOptions{})
		if err != nil {
			if kuberneteserrors.IsNotFound(err) {
				klog.V(2).Infof("Secret %s in namespace %s not found", k.Name, pod.Namespace)
			} else {
				klog.Errorf("Failed to get secret %s in namespace %s: %v", k.Name, pod.Namespace, err)
			}
			continue
		}
		if secret.Labels["app.kubernetes.io/managed-by"] == "troubleshoot.sh" {
			if err := client.CoreV1().Secrets(pod.Namespace).Delete(context.Background(), k.Name, metav1.DeleteOptions{}); err != nil {
				klog.Errorf("Failed to delete secret %s in namespace %s: %v", k.Name, pod.Namespace, err)
			} else {
				klog.V(2).Infof("Deleted secret %s in namespace %s", k.Name, pod.Namespace)
			}
		}
	}
}

func runPodWithSpec(ctx context.Context, client *kubernetes.Clientset, runPodCollector *troubleshootv1beta2.RunPod) (*corev1.Pod, error) {
	pod := createPodStruct(runPodCollector)

	if runPodCollector.ImagePullSecret != nil && runPodCollector.ImagePullSecret.Data != nil {
		secretName, err := createSecret(ctx, client, pod.Namespace, runPodCollector.ImagePullSecret)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create secret")
		}
		pod.Spec.ImagePullSecrets = append(pod.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: secretName})
	}

	created, err := client.CoreV1().Pods(pod.Namespace).Create(ctx, &pod, metav1.CreateOptions{})
	klog.V(2).Infof("Pod %s has been created", pod.Name)

	if err != nil {
		return nil, errors.Wrap(err, "failed to create pod")
	}

	return created, nil
}

func runWithoutTimeout(ctx context.Context, bundlePath string, clientConfig *rest.Config, pod *corev1.Pod, runPodCollector *troubleshootv1beta2.RunPod) (CollectorResult, error) {
	client, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed create client from config")
	}

	for {
		status, err := client.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			return nil, errors.Wrap(err, "failed to get pod")
		}
		if status.Status.Phase == corev1.PodRunning ||
			status.Status.Phase == corev1.PodFailed ||
			status.Status.Phase == corev1.PodSucceeded {
			break
		}
		if status.Status.Phase == corev1.PodPending {
			for _, v := range status.Status.ContainerStatuses {
				if v.State.Waiting != nil && v.State.Waiting.Reason == "ImagePullBackOff" {
					return nil, errors.Errorf("run pod aborted after getting pod status 'ImagePullBackOff'")
				}

				if v.State.Waiting != nil && v.State.Waiting.Reason == "ContainerCreating" {
					podEvents, err := client.CoreV1().Events(pod.Namespace).List(ctx, metav1.ListOptions{FieldSelector: (fmt.Sprintf("involvedObject.name=%s", pod.Name)), TypeMeta: metav1.TypeMeta{Kind: "Pod"}})
					if err != nil {
						return nil, errors.Wrap(err, "failed to get pod events")
					}

					for _, podEvent := range podEvents.Items {
						if podEvent.Reason == "FailedCreatePodSandBox" {
							klog.V(2).Infof("Pod %s failed to setup network for sandbox", pod.Name)
							return nil, errors.Errorf("run pod aborted after getting pod status 'FailedCreatePodSandBox'")
						}
					}
				}
			}
		}

		time.Sleep(time.Second * 1)
	}

	output := NewResult()

	collectorName := runPodCollector.Name

	limits := troubleshootv1beta2.LogLimits{
		MaxLines: 10000,
		MaxBytes: 5000000,
	}
	podLogs, err := savePodLogs(ctx, bundlePath, client, pod, collectorName, "", &limits, true, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pod logs")
	}

	for k, v := range podLogs {
		output[k] = v
	}

	return output, nil
}

func createSecret(ctx context.Context, client kubernetes.Interface, namespace string, imagePullSecret *troubleshootv1beta2.ImagePullSecrets) (string, error) {
	if imagePullSecret.Data == nil {
		return "", nil
	}

	var out bytes.Buffer
	data := make(map[string][]byte)
	if imagePullSecret.SecretType != "kubernetes.io/dockerconfigjson" {
		return "", errors.Errorf("ImagePullSecret must be of type: kubernetes.io/dockerconfigjson")
	}

	// Check if required field in data exists
	v, found := imagePullSecret.Data[".dockerconfigjson"]
	if !found {
		return "", errors.Errorf("Secret type kubernetes.io/dockerconfigjson requires argument \".dockerconfigjson\"")
	}
	if len(imagePullSecret.Data) > 1 {
		return "", errors.Errorf("Secret type kubernetes.io/dockerconfigjson accepts only one argument \".dockerconfigjson\"")
	}
	// K8s client accepts only Json formated files as data, provided data must be decoded and indented
	parsedConfig, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return "", errors.Wrap(err, "Unable to decode data.")
	}
	err = json.Indent(&out, parsedConfig, "", "\t")
	if err != nil {
		return "", errors.Wrap(err, "Unable to parse encoded data.")
	}
	data[".dockerconfigjson"] = out.Bytes()

	secret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:         imagePullSecret.Name,
			GenerateName: "troubleshoot",
			Namespace:    namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "troubleshoot.sh",
			},
		},
		Data: data,
		Type: corev1.SecretType(imagePullSecret.SecretType),
	}

	created, err := client.CoreV1().Secrets(namespace).Create(ctx, &secret, metav1.CreateOptions{})
	if err != nil {
		return "", errors.Wrap(err, "failed to create secret")
	}

	return created.Name, nil
}

// RunPodOptions and RunPodReadyNodes currently only used for the Sysctl collector
// TODO: refactor sysctl collector and runPod collector to share the same code
type RunPodOptions struct {
	Image               string
	ImagePullPolicy     string
	Namespace           string
	Command             []string
	ImagePullSecretName string
	HostNetwork         bool
}

func RunPodsReadyNodes(ctx context.Context, client v1.CoreV1Interface, opts RunPodOptions) (map[string][]byte, error) {
	wg := sync.WaitGroup{}
	mtx := sync.Mutex{}
	nodeLogs := map[string][]byte{}

	nodes, err := client.Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "list nodes")
	}

	for _, node := range nodes.Items {
		if !k8sutil.NodeIsReady(node) {
			continue
		}

		wg.Add(1)

		go func(node string) {
			defer wg.Done()

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "run-pod-",
					Namespace:    opts.Namespace,
				},
				Spec: corev1.PodSpec{
					NodeSelector: map[string]string{
						"kubernetes.io/hostname": node,
					},
					RestartPolicy: corev1.RestartPolicyNever,
					HostNetwork:   opts.HostNetwork,
					Containers: []corev1.Container{
						{
							Name:            "run",
							Image:           opts.Image,
							ImagePullPolicy: corev1.PullPolicy(opts.ImagePullPolicy),
							Command:         opts.Command,
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      "node-role.kubernetes.io/master",
							Operator: "Exists",
							Effect:   "NoSchedule",
						},
						{
							Key:      "node-role.kubernetes.io/control-plane",
							Operator: "Exists",
							Effect:   "NoSchedule",
						},
					},
				},
			}
			if opts.ImagePullSecretName != "" {
				pod.Spec.ImagePullSecrets = append(pod.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: opts.ImagePullSecretName})
			}
			logs, err := RunPodLogs(ctx, client, pod)
			if err != nil {
				klog.Errorf("Failed to run pod on node %s: %v", node, err)
				return
			}

			mtx.Lock()
			defer mtx.Unlock()
			nodeLogs[node] = logs
		}(node.Name)
	}

	wg.Wait()

	return nodeLogs, nil
}

// RunPodLogs runs a pod to completion on a node and returns its logs
func RunPodLogs(ctx context.Context, client v1.CoreV1Interface, podSpec *corev1.Pod) ([]byte, error) {
	// 1. Create
	pod, err := client.Pods(podSpec.Namespace).Create(ctx, podSpec, metav1.CreateOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create pod")
	}
	defer func() {
		err := client.Pods(pod.Namespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{})
		if err != nil && !kuberneteserrors.IsNotFound(err) {
			klog.Errorf("Failed to delete pod %s: %v\n", pod.Name, err)
		}
	}()

	// 2. Wait
	for {
		pod, err := client.Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			return nil, errors.Wrap(err, "failed to get pod")
		}

		if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
			break
		}

		if pod.Status.Phase == corev1.PodPending {
			for _, v := range pod.Status.ContainerStatuses {
				if v.State.Waiting != nil && v.State.Waiting.Reason == "ImagePullBackOff" {
					return nil, errors.New("wait for pod aborted after getting pod status 'ImagePullBackOff'")
				}
			}
		}
	}

	// 3. Logs
	podLogOpts := corev1.PodLogOptions{
		Container: pod.Spec.Containers[0].Name,
	}
	req := client.Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	logs, err := req.Stream(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get log stream")
	}
	defer logs.Close()

	return io.ReadAll(logs)
}

func savePodDetails(ctx context.Context, client *kubernetes.Clientset, output CollectorResult, bundlePath string, clientConfig *rest.Config, pod *corev1.Pod, runPodCollector *troubleshootv1beta2.RunPod) (CollectorResult, error) {
	podStatus, err := client.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pod")
	}

	podEvents, err := client.CoreV1().Events(pod.Namespace).List(ctx, metav1.ListOptions{FieldSelector: (fmt.Sprintf("involvedObject.name=%s", pod.Name)), TypeMeta: metav1.TypeMeta{Kind: "Pod"}})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pod events")
	}

	podBytes, err := json.MarshalIndent(podStatus, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal pod status")
	}

	podEventBytes, err := json.MarshalIndent(podEvents, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal pod events")
	}

	err = output.SaveResult(bundlePath, filepath.Join(runPodCollector.Name, fmt.Sprintf("%s.json", runPodCollector.Name)), bytes.NewBuffer(podBytes))
	if err != nil {
		klog.Errorf("failed to save pod status results to %s.json: %v", runPodCollector.Name, err)
	}

	err = output.SaveResult(bundlePath, filepath.Join(runPodCollector.Name, fmt.Sprintf("%s-events.json", runPodCollector.Name)), bytes.NewBuffer(podEventBytes))
	if err != nil {
		klog.Errorf("failed to save pod event results to %s-events.json: %v", runPodCollector.Name, err)
	}
	return output, nil
}

func deletePod(ctx context.Context, client *kubernetes.Clientset, pod *corev1.Pod) {
	if err := client.CoreV1().Pods(pod.Namespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{}); err != nil {
		klog.Errorf("Failed to delete pod %s: %v", pod.Name, err)
		return
	}
	klog.V(2).Infof("Pod %s has been scheduled for deletion", pod.Name)

	// Wait until the pod is deleted
	// Poll every second to check if the Pod has been deleted.
	klog.V(2).Infof("Continuously poll each second for Pod %s deletion for maximum %d seconds", pod.Name, constants.MAX_TIME_TO_WAIT_FOR_POD_DELETION/time.Second)
	err := wait.PollUntilContextTimeout(ctx, time.Second, constants.MAX_TIME_TO_WAIT_FOR_POD_DELETION, true, func(ctx context.Context) (bool, error) {
		_, getErr := client.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		// If the Pod is not found, it has been deleted.
		if kuberneteserrors.IsNotFound(getErr) {
			return true, nil
		}
		// If there is an error from context (e.g., context deadline exceeded), return the error.
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		// Otherwise, the Pod has not yet been deleted. Keep polling.
		return false, nil
	})
	if err != nil {
		zeroGracePeriod := int64(0)
		klog.V(2).Infof("Forcefully deleting pod %s after reaching the maximum wait time of %d seconds due to err=%v",
			pod.Name, constants.MAX_TIME_TO_WAIT_FOR_POD_DELETION/time.Second, err,
		)
		if err := client.CoreV1().Pods(pod.Namespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{
			GracePeriodSeconds: &zeroGracePeriod,
		}); err != nil {
			klog.Errorf("Forced deletion of pod %s failed: %v", pod.Name, err)
			return
		}
		klog.V(2).Infof("Pod %s in %s namespace has been deleted", pod.Name, pod.Namespace)
	} else {
		klog.V(2).Infof("Pod %s in %s namespace has been deleted", pod.Name, pod.Namespace)
	}
}

func createPodStruct(runPodCollector *troubleshootv1beta2.RunPod) corev1.Pod {
	podLabels := make(map[string]string)
	podLabels["troubleshoot-role"] = "run-collector"

	namespace := "default"
	if runPodCollector.Namespace != "" {
		namespace = runPodCollector.Namespace
	}

	podName := "run-pod"
	if runPodCollector.CollectorName != "" {
		podName = runPodCollector.CollectorName
	} else if runPodCollector.Name != "" {
		podName = runPodCollector.Name
	}

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        podName,
			Namespace:   namespace,
			Labels:      podLabels,
			Annotations: runPodCollector.Annotations,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		Spec: runPodCollector.PodSpec,
	}

	return pod
}
