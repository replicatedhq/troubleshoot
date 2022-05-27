package collect

import (
	"context"
	"io/ioutil"
	"sync"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"

	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
				logger.Printf("Failed to run pod on node %s: %v", node, err)
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
func RunPodLogs(ctx context.Context, client v1.CoreV1Interface, pod *corev1.Pod) ([]byte, error) {
	// 1. Create
	pod, err := client.Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create pod")
	}
	defer func() {
		go func() {
			// use context.background for the after-completion cleanup, as the parent context might already be over
			err := client.Pods(pod.Namespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{})
			if err != nil && !kuberneteserrors.IsNotFound(err) {
				logger.Printf("Failed to delete pod %s: %v\n", pod.Name, err)
			}
		}()
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

	return ioutil.ReadAll(logs)
}
