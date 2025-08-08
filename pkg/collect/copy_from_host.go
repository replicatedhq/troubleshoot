package collect

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/segmentio/ksuid"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"
)

type CollectCopyFromHost struct {
	Collector        *troubleshootv1beta2.CopyFromHost
	BundlePath       string
	Namespace        string
	ClientConfig     *rest.Config
	Client           kubernetes.Interface
	Context          context.Context
	RetryFailedMount bool
	RBACErrors
}

func (c *CollectCopyFromHost) Title() string {
	return getCollectorName(c)
}

func (c *CollectCopyFromHost) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectCopyFromHost) SkipRedaction() bool {
	return c.Collector.SkipRedaction
}

// copies a file or directory from a host or hosts to include in the bundle.
func (c *CollectCopyFromHost) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	var namespace string

	labels := map[string]string{
		"app.kubernetes.io/managed-by":    "troubleshoot.sh",
		"troubleshoot.sh/collector":       "copyfromhost",
		"troubleshoot.sh/copyfromhost-id": ksuid.New().String(),
	}

	hostPath := filepath.Clean(c.Collector.HostPath) // strip trailing slash

	hostDir := filepath.Dir(hostPath)
	fileName := filepath.Base(hostPath)
	if hostDir == filepath.Dir(hostDir) { // is the parent directory the root?
		hostDir = hostPath
		fileName = "."
	}

	namespace = c.Namespace
	if namespace == "" {
		kubeconfig := k8sutil.GetKubeconfig()
		namespace, _, _ = kubeconfig.Namespace()
	}

	_, cleanup, err := c.copyFromHostCreateDaemonSet(c.Context, c.Client, c.Collector, hostDir, namespace, "troubleshoot-copyfromhost-", labels)
	defer cleanup()
	if err != nil {
		return nil, errors.Wrap(err, "create daemonset")
	}

	childCtx, cancel := context.WithCancel(c.Context)
	defer cancel()

	timeoutCtx := context.Background()
	if c.Collector.Timeout != "" {
		timeout, err := time.ParseDuration(c.Collector.Timeout)
		if err != nil {
			return nil, errors.Wrap(err, "parse timeout")
		}

		if timeout > 0 {
			childCtx, cancel = context.WithTimeout(childCtx, timeout)
			defer cancel()
		}
	}

	errCh := make(chan error, 1)
	resultCh := make(chan CollectorResult, 1)
	go func() {
		var outputFilename string
		if c.Collector.Name != "" {
			outputFilename = c.Collector.Name
		} else {
			outputFilename = hostPath
		}
		b, err := copyFromHostGetFilesFromPods(childCtx, c.BundlePath, c.Collector, c.ClientConfig, c.Client, fileName, outputFilename, labels, namespace)
		if err != nil {
			errCh <- err
		} else {
			resultCh <- b
		}
	}()

	select {
	case <-timeoutCtx.Done():
		return nil, errors.New("timeout")
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, errors.New("timeout")
		}
		return nil, err
	}
}

func (c *CollectCopyFromHost) copyFromHostCreateDaemonSet(ctx context.Context, client kubernetes.Interface, collector *troubleshootv1beta2.CopyFromHost, hostPath string, namespace string, generateName string, labels map[string]string) (name string, cleanup func(), err error) {
	pullPolicy := corev1.PullIfNotPresent
	volumeType := corev1.HostPathDirectory
	if collector.ImagePullPolicy != "" {
		pullPolicy = corev1.PullPolicy(collector.ImagePullPolicy)
	}

	ds := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: generateName,
			Namespace:    namespace,
			Labels:       labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyAlways,
					Containers: []corev1.Container{
						{
							Image:           collector.Image,
							ImagePullPolicy: pullPolicy,
							Name:            "collector",
							Command:         []string{"sleep"},
							Args:            []string{"1000000"},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "host",
									MountPath: "/host",
								},
							},
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
					Volumes: []corev1.Volume{
						{
							Name: "host",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: hostPath,
									Type: &volumeType,
								},
							},
						},
					},
				},
			},
		},
	}

	cleanupFuncs := []func(){}
	cleanup = func() {
		for _, fn := range cleanupFuncs {
			fn()
		}
	}

	if collector.ImagePullSecret != nil && collector.ImagePullSecret.Data != nil {
		secretName, err := createSecret(ctx, client, namespace, collector.ImagePullSecret)
		if err != nil {
			return "", cleanup, errors.Wrap(err, "create secret")
		}
		ds.Spec.Template.Spec.ImagePullSecrets = append(ds.Spec.Template.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: secretName})

		cleanupFuncs = append(cleanupFuncs, func() {
			err := client.CoreV1().Secrets(namespace).Delete(context.Background(), collector.ImagePullSecret.Name, metav1.DeleteOptions{})
			if err != nil && !kuberneteserrors.IsNotFound(err) {
				klog.Errorf("Failed to delete secret %s: %v", collector.ImagePullSecret.Name, err)
			}
		})
	}

	createdDS, err := client.AppsV1().DaemonSets(namespace).Create(ctx, &ds, metav1.CreateOptions{})

	if err != nil {
		return "", cleanup, errors.Wrap(err, "create daemonset")
	}
	cleanupFuncs = append(cleanupFuncs, func() {
		deleteDaemonSet(client, ctx, createdDS, namespace, labels)
	})

	// This timeout is different from collector timeout.
	// Time it takes to pull images should not count towards collector timeout.
	childCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		select {
		case <-time.After(1 * time.Second):
			err = checkDaemonPodStatus(client, ctx, labels, namespace, c.RetryFailedMount)
			if err != nil {
				return createdDS.Name, cleanup, err
			}

		case <-childCtx.Done():
			klog.V(2).Infof("Timed out waiting for daemonset %s to be ready", createdDS.Name)
			return createdDS.Name, cleanup, errors.Wrap(ctx.Err(), "wait for daemonset")
		}

		ds, err := client.AppsV1().DaemonSets(namespace).Get(ctx, createdDS.Name, metav1.GetOptions{})
		if err != nil {
			if !kuberneteserrors.IsNotFound(err) {
				continue
			}
			return createdDS.Name, cleanup, errors.Wrap(err, "get daemonset")
		}

		if ds.Status.DesiredNumberScheduled == 0 || ds.Status.DesiredNumberScheduled != ds.Status.NumberAvailable {
			continue
		}

		break
	}

	return createdDS.Name, cleanup, nil
}

func copyFromHostGetFilesFromPods(ctx context.Context, bundlePath string, collector *troubleshootv1beta2.CopyFromHost, clientConfig *restclient.Config, client kubernetes.Interface, fileName string, outputFilename string, labelSelector map[string]string, namespace string) (CollectorResult, error) {
	opts := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labelSelector).String(),
	}

	pods, err := client.CoreV1().Pods(namespace).List(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err, "list pods")
	}

	output := NewResult()
	for _, pod := range pods.Items {
		outputNodeFilename := filepath.Join(outputFilename, pod.Spec.NodeName)
		files, stderr, err := copyFilesFromHost(ctx, filepath.Join(bundlePath, outputNodeFilename), clientConfig, client, pod.Name, "collector", namespace, filepath.Join("/host", fileName), collector.ExtractArchive)
		if err != nil {
			output.SaveResult(bundlePath, filepath.Join(outputNodeFilename, "error.txt"), bytes.NewBuffer([]byte(err.Error())))
			if len(stderr) > 0 {
				output.SaveResult(bundlePath, filepath.Join(outputNodeFilename, "stderr.txt"), bytes.NewBuffer(stderr))
			}
		}

		for k, v := range files {
			relPath, err := filepath.Rel(bundlePath, filepath.Join(bundlePath, filepath.Join(outputNodeFilename, k)))
			if err != nil {
				return nil, errors.Wrap(err, "relative path")
			}
			output[relPath] = v
		}
	}

	return output, nil
}

func copyFilesFromHost(ctx context.Context, dstPath string, clientConfig *restclient.Config, client kubernetes.Interface, podName string, containerName string, namespace string, containerPath string, extract bool) (CollectorResult, []byte, error) {
	command := []string{"tar", "-C", filepath.Dir(containerPath), "-cf", "-", filepath.Base(containerPath)}
	req := client.CoreV1().RESTClient().Post().Resource("pods").Name(podName).Namespace(namespace).SubResource("exec")
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, nil, errors.Wrap(err, "failed to add runtime scheme")
	}

	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&corev1.PodExecOptions{
		Command:   command,
		Container: containerName,
		Stdin:     true,
		Stdout:    false,
		Stderr:    true,
		TTY:       false,
	}, parameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(clientConfig, "POST", req.URL())
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create SPDY executor")
	}

	result := NewResult()

	var stdoutWriter io.Writer
	var copyError error
	if extract {
		pipeReader, pipeWriter := io.Pipe()
		tarReader := tar.NewReader(pipeReader)
		stdoutWriter = pipeWriter

		go func() {
			// this can cause "read/write on closed pipe" error, but without this exec.Stream blocks
			defer pipeWriter.Close()

			for {
				header, err := tarReader.Next()
				if err == io.EOF {
					return
				}
				if err != nil {
					pipeWriter.CloseWithError(errors.Wrap(err, "failed to read header from tar"))
					return
				}

				switch header.Typeflag {
				case tar.TypeDir:
					name := filepath.Join(dstPath, header.Name)
					if err := os.MkdirAll(name, os.FileMode(header.Mode)); err != nil {
						pipeWriter.CloseWithError(errors.Wrap(err, "failed to mkdir"))
						return
					}
				case tar.TypeReg:
					err := result.SaveResult(dstPath, header.Name, tarReader)
					if err != nil {
						pipeWriter.CloseWithError(errors.Wrapf(err, "failed to save result for file %s", header.Name))
						return
					}
				}
			}
		}()
	} else {
		w, err := result.GetWriter(dstPath, "archive.tar")
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to craete dest file")
		}
		defer result.CloseWriter(dstPath, "archive.tar", w)

		stdoutWriter = w
	}

	var stderr bytes.Buffer
	copyError = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: stdoutWriter,
		Stderr: &stderr,
		Tty:    false,
	})
	if copyError != nil {
		return result, stderr.Bytes(), errors.Wrap(copyError, "failed to stream command output")
	}

	return result, stderr.Bytes(), nil
}

func deleteDaemonSet(client kubernetes.Interface, ctx context.Context, createdDS *appsv1.DaemonSet, namespace string, labels map[string]string) {
	klog.V(2).Infof("Daemonset %s has been scheduled for deletion", createdDS.Name)
	zeroGracePeriod := int64(0)
	// Foreground is used to delete the DaemonSet pods before deleting the DaemonSet
	deletePropagationForeground := metav1.DeletePropagationForeground

	if err := client.AppsV1().DaemonSets(namespace).Delete(ctx, createdDS.Name, metav1.DeleteOptions{
		GracePeriodSeconds: &zeroGracePeriod,
		PropagationPolicy:  &deletePropagationForeground,
	}); err != nil {
		klog.Errorf("Failed to delete daemonset %s: %v", createdDS.Name, err)
		return
	}

	var labelSelector []string
	for k, v := range labels {
		labelSelector = append(labelSelector, fmt.Sprintf("%s=%s", k, v))
	}

	dsPods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: strings.Join(labelSelector, ",")})
	if err != nil {
		klog.Errorf("Failed to list pods for DaemonSet %s: %v", createdDS.Name, err)
		return
	}

	for _, pod := range dsPods.Items {
		klog.V(2).Infof("Deleting pod %s", pod.Name)
		if err := client.CoreV1().Pods(namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{
			GracePeriodSeconds: &zeroGracePeriod,
		}); err != nil {
			klog.Errorf("Failed to delete pod %s: %v", pod.Name, err)
		}
	}

	klog.V(2).Infof("Continuously poll each second for Pod deletion of DaemontSet %s for maximum %d seconds", createdDS.Name, constants.MAX_TIME_TO_WAIT_FOR_POD_DELETION/time.Second)

	err = wait.PollUntilContextTimeout(ctx, time.Second, constants.MAX_TIME_TO_WAIT_FOR_POD_DELETION, true, func(ctx context.Context) (bool, error) {
		pods, listErr := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: strings.Join(labelSelector, ","),
		})

		if listErr != nil {
			klog.Errorf("Failed to list pods created by %s daemonset: %v", createdDS.Name, listErr)
		}
		// If there are no pods remaining, return true to stop the polling
		if len(pods.Items) == 0 {
			return true, nil
		}
		// If there is an error from context (e.g., context deadline exceeded), return the error.
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		// If there are still pods remaining and there was no context error, save the list of pods,
		dsPods = pods
		return false, nil
	})

	// If there was an error from the polling (e.g., the context deadline was exceeded before all pods were deleted),
	// delete each remaining pod with a zero-second grace period
	if err != nil {
		for _, pod := range dsPods.Items {
			klog.V(2).Infof("Pod %s forcefully deleted after reaching the maximum wait time of %d seconds", pod.Name, constants.MAX_TIME_TO_WAIT_FOR_POD_DELETION/time.Second)
			err := client.CoreV1().Pods(namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{
				GracePeriodSeconds: &zeroGracePeriod,
			})
			if err != nil {
				klog.Errorf("Failed to wait for pod %s deletion: %v", pod.Name, err)
				return
			}
			klog.V(2).Infof("Daemonset pod %s in %s namespace has been deleted", pod.Name, pod.Namespace)
		}
	}
}

func checkDaemonPodStatus(client kubernetes.Interface, ctx context.Context, labels map[string]string, namespace string, retryFailedMount bool) error {
	var labelSelector []string
	for k, v := range labels {
		labelSelector = append(labelSelector, fmt.Sprintf("%s=%s", k, v))
	}
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: strings.Join(labelSelector, ","),
	})
	if err != nil {
		return errors.Wrap(err, "get daemonset pods")
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			events, _ := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
				FieldSelector: fmt.Sprintf("involvedObject.uid=%s", pod.UID),
			})

			for _, event := range events.Items {
				// If the pod has a FailedMount event, it means that the pod failed to mount the volume and the pod will be stuck in the Pending state.
				// In this case, we return an error to the caller to indicate that path does not exist.
				if event.Reason == "FailedMount" && !retryFailedMount {
					klog.V(2).Infof("pod %s has a FailedMount event: %s", pod.Name, event.Message)
					return errors.Errorf("path does not exist")
				}
			}
		}
	}
	return nil
}
