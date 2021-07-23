package collect

import (
	"bytes"
	"context"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"github.com/segmentio/ksuid"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

// CopyFromHost is a function that copies a file or directory from a host or hosts to include in the bundle.
func CopyFromHost(ctx context.Context, namespace string, clientConfig *restclient.Config, client kubernetes.Interface, collector *troubleshootv1beta2.CopyFromHost) (map[string][]byte, error) {
	labels := map[string]string{
		"app.kubernetes.io/managed-by":    "troubleshoot.sh",
		"troubleshoot.sh/collector":       "copyfromhost",
		"troubleshoot.sh/copyfromhost-id": ksuid.New().String(),
	}

	hostPath := filepath.Clean(collector.HostPath) // strip trailing slash

	hostDir := filepath.Dir(hostPath)
	fileName := filepath.Base(hostPath)
	if hostDir == filepath.Dir(hostDir) { // is the parent directory the root?
		hostDir = hostPath
		fileName = "."
	}

	_, cleanup, err := copyFromHostCreateDaemonSet(ctx, client, collector, hostDir, namespace, "troubleshoot-copyfromhost-", labels)
	defer cleanup()
	if err != nil {
		return nil, errors.Wrap(err, "create daemonset")
	}

	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	timeoutCtx := context.Background()
	if collector.Timeout != "" {
		timeout, err := time.ParseDuration(collector.Timeout)
		if err != nil {
			return nil, errors.Wrap(err, "parse timeout")
		}

		if timeout > 0 {
			childCtx, cancel = context.WithTimeout(childCtx, timeout)
			defer cancel()
		}
	}

	errCh := make(chan error, 1)
	resultCh := make(chan map[string][]byte, 1)
	go func() {
		var outputFilename string
		if collector.Name != "" {
			outputFilename = collector.Name
		} else {
			outputFilename = hostPath
		}
		b, err := copyFromHostGetFilesFromPods(childCtx, clientConfig, client, collector, fileName, outputFilename, labels, namespace)
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

func copyFromHostCreateDaemonSet(ctx context.Context, client kubernetes.Interface, collector *troubleshootv1beta2.CopyFromHost, hostPath string, namespace string, generateName string, labels map[string]string) (name string, cleanup func(), err error) {
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
			err := client.CoreV1().Secrets(namespace).Delete(ctx, collector.ImagePullSecret.Name, metav1.DeleteOptions{})
			if err != nil && !kuberneteserrors.IsNotFound(err) {
				logger.Printf("Failed to delete secret %s: %v", collector.ImagePullSecret.Name, err)
			}
		})
	}

	createdDS, err := client.AppsV1().DaemonSets(namespace).Create(ctx, &ds, metav1.CreateOptions{})
	if err != nil {
		return "", cleanup, errors.Wrap(err, "create daemonset")
	}
	cleanupFuncs = append(cleanupFuncs, func() {
		if err := client.AppsV1().DaemonSets(namespace).Delete(ctx, createdDS.Name, metav1.DeleteOptions{}); err != nil {
			logger.Printf("Failed to delete daemonset %s: %v", createdDS.Name, err)
		}
	})

	// This timeout is different from collector timeout.
	// Time it takes to pull images should not count towards collector timeout.
	childCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		select {
		case <-time.After(1 * time.Second):
		case <-childCtx.Done():
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

func copyFromHostGetFilesFromPods(ctx context.Context, clientConfig *restclient.Config, client kubernetes.Interface, collector *troubleshootv1beta2.CopyFromHost, fileName string, outputFilename string, labelSelector map[string]string, namespace string) (map[string][]byte, error) {
	opts := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labelSelector).String(),
	}

	pods, err := client.CoreV1().Pods(namespace).List(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err, "list pods")
	}

	runOutput := map[string][]byte{}
	for _, pod := range pods.Items {
		outputNodeFilename := filepath.Join(outputFilename, pod.Spec.NodeName)
		stdout, stderr, err := getFilesFromPod(ctx, clientConfig, client, pod.Name, "collector", namespace, filepath.Join("/host", fileName))
		if err != nil {
			runOutput[filepath.Join(outputNodeFilename, "error.txt")] = []byte(err.Error())
			if len(stdout) > 0 {
				runOutput[filepath.Join(outputNodeFilename, "stdout.txt")] = stdout
			}
			if len(stderr) > 0 {
				runOutput[filepath.Join(outputNodeFilename, "stderr.txt")] = stderr
			}
		} else {
			if collector.ExtractArchive {
				files, err := extractTar(bytes.NewReader(stdout))
				if err != nil {
					runOutput[filepath.Join(outputNodeFilename, "error.txt")] = []byte(errors.Wrap(err, "extract tar").Error())
				}
				for name, data := range files {
					runOutput[filepath.Join(outputNodeFilename, name)] = data
				}
			} else {
				runOutput[filepath.Join(outputNodeFilename, "archive.tar")] = stdout
			}
		}
	}

	return runOutput, nil
}
