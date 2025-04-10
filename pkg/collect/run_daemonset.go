package collect

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const (
	defaultTimeout = time.Duration(60 * time.Second)
)

type CollectRunDaemonSet struct {
	Collector    *troubleshootv1beta2.RunDaemonSet
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectRunDaemonSet) Title() string {
	return getCollectorName(c)
}

func (c *CollectRunDaemonSet) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectRunDaemonSet) SkipRedaction() bool {
	return c.Collector.SkipRedaction
}

func (c *CollectRunDaemonSet) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	ctx := context.Background()

	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create client from config")
	}

	// create DaemonSet Spec
	dsSpec, err := createDaemonSetSpec(c.Collector)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create DaemonSet spec")
	}

	// create ImagePullSecret
	var secretName string
	if c.Collector.ImagePullSecret != nil && c.Collector.ImagePullSecret.Data != nil {
		secretName, err = createSecret(ctx, client, dsSpec.ObjectMeta.Namespace, c.Collector.ImagePullSecret)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create ImagePullSecret")
		}
		dsSpec.Spec.Template.Spec.ImagePullSecrets = append(dsSpec.Spec.Template.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: secretName})
	}

	// run DaemonSet
	ds, err := client.AppsV1().DaemonSets(dsSpec.ObjectMeta.Namespace).Create(ctx, dsSpec, metav1.CreateOptions{})
	klog.V(2).Infof("DaemonSet %s has been created", ds.Name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create DaemonSet")
	}

	defer func() {
		// delete DaemonSet
		err := client.AppsV1().DaemonSets(ds.ObjectMeta.Namespace).Delete(ctx, ds.ObjectMeta.Name, metav1.DeleteOptions{})
		if err != nil {
			klog.Errorf("Failed to delete DaemonSet %s: %v", ds.Name, err)
		}
		// delete ImagePullSecret
		if secretName != "" {
			err := client.CoreV1().Secrets(ds.ObjectMeta.Namespace).Delete(ctx, secretName, metav1.DeleteOptions{})
			if err != nil {
				klog.Errorf("Failed to delete Secret %s: %v", secretName, err)
			}
		}
	}()

	// set custom timeout if any
	var (
		timeout            time.Duration
		errInvalidDuration error
	)
	if c.Collector.Timeout != "" {
		timeout, errInvalidDuration = time.ParseDuration(c.Collector.Timeout)
		if errInvalidDuration != nil {
			return nil, errors.Wrapf(errInvalidDuration, "failed to parse timeout %q", c.Collector.Timeout)
		}
	}
	if timeout <= time.Duration(0) {
		timeout = defaultTimeout
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// block till DaemonSet has right number of scheduled Pods
	err = waitForDaemonSetPods(timeoutCtx, client, ds)
	if err != nil {
		return nil, errors.Wrap(err, "failed to wait for DaemonSet pods")
	}
	klog.V(2).Infof("DaemonSet %s has desired number of pods", ds.Name)

	// get all Pods in DaemonSet
	pods, err := client.CoreV1().Pods(ds.ObjectMeta.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: getLabelSelector(ds),
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to list pods")
	}

	results := NewResult()

	// collect logs from all Pods
	// or save error message if failed to get logs
	wg := &sync.WaitGroup{}
	mtx := &sync.Mutex{}
	for _, pod := range pods.Items {
		wg.Add(1)
		go func(pod corev1.Pod) {
			defer wg.Done()

			select {
			case <-timeoutCtx.Done():
				klog.Errorf("Timeout reached while waiting for pod %s", pod.Name)
				return
			default:
			}

			var logs []byte

			nodeName, err := getPodNodeAtCompletion(timeoutCtx, client.CoreV1(), pod)
			if err != nil {
				nodeName = fmt.Sprintf("unknown-node-%s", pod.Name)
				errString := fmt.Sprintf("Failed to get node name/wait for pod %s to complete: %v", pod.Name, err)
				klog.Error(errString)
				logs = []byte(errString)
			} else {
				logs, err = getPodLog(timeoutCtx, client.CoreV1(), pod)
				if err != nil {
					errString := fmt.Sprintf("Failed to get log from pod %s: %v", pod.Name, err)
					klog.Error(errString)
					logs = []byte(errString)
				}
			}

			mtx.Lock()
			defer mtx.Unlock()
			results[nodeName] = logs
			klog.V(2).Infof("Collected logs for pod %s", pod.Name)

		}(pod)
	}
	wg.Wait()

	output := NewResult()
	for k, v := range results {
		filename := k + ".log"
		err := output.SaveResult(c.BundlePath, filepath.Join(c.Collector.Name, filename), bytes.NewBuffer(v))
		if err != nil {
			return nil, err
		}
	}

	return output, nil

}

func createDaemonSetSpec(c *troubleshootv1beta2.RunDaemonSet) (*appsv1.DaemonSet, error) {
	ds := &appsv1.DaemonSet{}

	labels := make(map[string]string)
	labels["troubleshoot-role"] = "run-daemonset-collector"

	namespace := "default"
	if c.Namespace != "" {
		namespace = c.Namespace
	}
	ds.ObjectMeta = metav1.ObjectMeta{
		GenerateName: fmt.Sprintf("run-daemonset-%s-", c.Name),
		Namespace:    namespace,
		Labels:       labels,
	}

	ds.Spec = appsv1.DaemonSetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: labels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Labels:    labels,
			},
			Spec: c.PodSpec,
		},
	}

	return ds, nil
}

func getLabelSelector(ds *appsv1.DaemonSet) string {
	labelSelector := ""
	for k, v := range ds.Spec.Template.ObjectMeta.Labels {
		labelSelector += k + "=" + v + ","
	}
	return strings.TrimSuffix(labelSelector, ",")
}

func getPodLog(ctx context.Context, client v1.CoreV1Interface, pod corev1.Pod) ([]byte, error) {
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

// getPodNodeAtCompletion waits for the Pod to complete and returns the node name
func getPodNodeAtCompletion(ctx context.Context, client v1.CoreV1Interface, pod corev1.Pod) (string, error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			pod, err := client.Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
			if err != nil {
				return "", errors.Wrap(err, "failed to get pod")
			}

			if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
				return pod.Spec.NodeName, nil
			}

			// we assume a container restart means the Pod has completed before
			if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].RestartCount > 0 {
				return pod.Spec.NodeName, nil
			}

			if pod.Status.Phase == corev1.PodPending {
				for _, v := range pod.Status.ContainerStatuses {
					if v.State.Waiting != nil && v.State.Waiting.Reason == "ImagePullBackOff" {
						return "", errors.New("wait for pod aborted after getting pod status 'ImagePullBackOff'")
					}
				}
			}
		}
	}
}

// waitForDaemonSetPods waits for the DaemonSet to have the desired number of pods scheduled
func waitForDaemonSetPods(ctx context.Context, client kubernetes.Interface, ds *appsv1.DaemonSet) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			ds, err := client.AppsV1().DaemonSets(ds.ObjectMeta.Namespace).Get(ctx, ds.ObjectMeta.Name, metav1.GetOptions{})
			if err != nil {
				return errors.Wrap(err, "failed to get DaemonSet")
			}

			// we return as soon as the desired number of pods are scheduled
			if ds.Status.DesiredNumberScheduled == ds.Status.CurrentNumberScheduled {
				return nil
			}
		}
	}
}
