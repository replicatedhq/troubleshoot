package collect

import (
	"context"
	"path"
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
)

func Collectd(c *Collector, collectdCollector *troubleshootv1beta2.Collectd) (map[string][]byte, error) {
	ctx := context.Background()
	label := ksuid.New().String()
	namespace := collectdCollector.Namespace

	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create client from config")
	}

	dsName, err := createDaemonSet(ctx, client, collectdCollector, namespace, label)
	if dsName != "" {
		defer func() {
			if err := client.AppsV1().DaemonSets(namespace).Delete(ctx, dsName, metav1.DeleteOptions{}); err != nil {
				logger.Printf("Failed to delete daemonset %s: %v\n", dsName, err)
			}
		}()

		if collectdCollector.ImagePullSecret != nil && collectdCollector.ImagePullSecret.Data != nil {
			defer func() {
				err := client.CoreV1().Secrets(namespace).Delete(ctx, collectdCollector.ImagePullSecret.Name, metav1.DeleteOptions{})
				if err != nil && !kuberneteserrors.IsNotFound(err) {
					logger.Printf("Failed to delete secret %s: %v\n", collectdCollector.ImagePullSecret.Name, err)
				}
			}()
		}
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to create daemonset")
	}

	if collectdCollector.Timeout == "" {
		return collectRRDFiles(ctx, client, c, collectdCollector, label, namespace)
	}

	timeout, err := time.ParseDuration(collectdCollector.Timeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse timeout")
	}

	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 1)
	resultCh := make(chan map[string][]byte, 1)
	go func() {
		b, err := collectRRDFiles(childCtx, client, c, collectdCollector, label, namespace)
		if err != nil {
			errCh <- err
		} else {
			resultCh <- b
		}
	}()

	select {
	case <-time.After(timeout):
		return nil, errors.New("timeout")
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return nil, err
	}
}

func createDaemonSet(ctx context.Context, client *kubernetes.Clientset, rrdCollector *troubleshootv1beta2.Collectd, namespace string, label string) (string, error) {
	pullPolicy := corev1.PullIfNotPresent
	volumeType := corev1.HostPathDirectory
	if rrdCollector.ImagePullPolicy != "" {
		pullPolicy = corev1.PullPolicy(rrdCollector.ImagePullPolicy)
	}
	dsLabels := map[string]string{
		"rrd-collector": label,
	}

	ds := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "troubleshoot",
			Namespace:    namespace,
			Labels:       dsLabels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: dsLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: dsLabels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyAlways,
					Containers: []corev1.Container{
						{
							Image:           rrdCollector.Image,
							ImagePullPolicy: pullPolicy,
							Name:            "collector",
							Command:         []string{"sleep"},
							Args:            []string{"1000000"},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "rrd",
									MountPath: "/rrd",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "rrd",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: rrdCollector.HostPath,
									Type: &volumeType,
								},
							},
						},
					},
				},
			},
		},
	}

	if rrdCollector.ImagePullSecret != nil && rrdCollector.ImagePullSecret.Name != "" {
		err := createSecret(ctx, client, namespace, rrdCollector.ImagePullSecret)
		if err != nil {
			return "", errors.Wrap(err, "failed to create secret")
		}
		ds.Spec.Template.Spec.ImagePullSecrets = append(ds.Spec.Template.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: rrdCollector.ImagePullSecret.Name})
	}

	createdDS, err := client.AppsV1().DaemonSets(namespace).Create(ctx, &ds, metav1.CreateOptions{})
	if err != nil {
		return "", errors.Wrap(err, "failed to create daemonset")
	}

	// This timeout is different from collector timeout.
	// Time it takes to pull images should not count towards collector timeout.
	childCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		select {
		case <-time.After(1 * time.Second):
		case <-childCtx.Done():
			return createdDS.Name, errors.Wrap(ctx.Err(), "failed to wait for daemonset")
		}

		ds, err := client.AppsV1().DaemonSets(namespace).Get(ctx, createdDS.Name, metav1.GetOptions{})
		if err != nil {
			if !kuberneteserrors.IsNotFound(err) {
				continue
			}
			return createdDS.Name, errors.Wrap(err, "failed to get daemonset")
		}

		if ds.Status.DesiredNumberScheduled != ds.Status.NumberReady {
			continue
		}

		break
	}

	return createdDS.Name, nil
}

func collectRRDFiles(ctx context.Context, client *kubernetes.Clientset, c *Collector, rrdCollector *troubleshootv1beta2.Collectd, label string, namespace string) (map[string][]byte, error) {
	labelSelector := map[string]string{
		"rrd-collector": label,
	}
	opts := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labelSelector).String(),
	}

	pods, err := client.CoreV1().Pods(namespace).List(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err, "list rrd collector pods")
	}

	pathPrefix := path.Join("collectd", "rrd")
	runOutput := map[string][]byte{}
	for _, pod := range pods.Items {
		stdout, stderr, err := getFilesFromPod(ctx, client, c, pod.Name, "", namespace, "/rrd")
		if err != nil {
			runOutput[path.Join(pathPrefix, pod.Spec.NodeName)+".error"] = []byte(err.Error())
			if len(stdout) > 0 {
				runOutput[filepath.Join(pathPrefix, pod.Spec.NodeName)+".stdout"] = stdout
			}
			if len(stderr) > 0 {
				runOutput[filepath.Join(pathPrefix, pod.Spec.NodeName)+".stderr"] = stderr
			}
			continue
		}

		runOutput[path.Join(pathPrefix, pod.Spec.NodeName)+".tar"] = stdout
	}

	return runOutput, nil
}
