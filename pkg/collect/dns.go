package collect

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type CollectDNS struct {
	Collector    *troubleshootv1beta2.DNS
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectDNS) Title() string {
	return getCollectorName(c)
}

func (c *CollectDNS) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectDNS) Collect(progressChan chan<- interface{}) (CollectorResult, error) {

	ctx, cancel := context.WithTimeout(c.Context, time.Duration(60*time.Second))
	defer cancel()

	sb := strings.Builder{}

	// get kubernetes Cluster IP
	clusterIP, err := getKubernetesClusterIP(c.Client, ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get kubernetes cluster IP")
	}
	sb.WriteString(fmt.Sprintf("=== Kubernetes Cluster IP from API Server: %s\n", clusterIP))

	// run a pod and perform DNS lookup
	podLog, err := troubleshootPodDNS(c.Client, ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to troubleshoot pod DNS")
	}
	sb.WriteString("=== Run commands from pod... \n")
	sb.WriteString(podLog)

	// get CoreDNS config
	coreDNSConfig, err := getCoreDNSConfig(c.Client, ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get CoreDNS config")
	}
	sb.WriteString("=== CoreDNS Config: \n")
	sb.WriteString(coreDNSConfig)

	data := sb.String()
	output := NewResult()
	output.SaveResult(c.BundlePath, filepath.Join("dns", c.Collector.CollectorName), bytes.NewBuffer([]byte(data)))

	return output, nil
}

func getKubernetesClusterIP(client kubernetes.Interface, ctx context.Context) (string, error) {
	service, err := client.CoreV1().Services("default").Get(ctx, "kubernetes", metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return service.Spec.ClusterIP, nil
}

func troubleshootPodDNS(client kubernetes.Interface, ctx context.Context) (string, error) {
	namespace := "default"
	image := "nicolaka/netshoot"
	command := []string{"/bin/bash", "-c", `
		set -x
		dig +short kubernetes.default.svc.cluster.local
		cat /etc/resolv.conf
		exit 0
	`}

	// TODO: image pull secret
	podLabels := map[string]string{
		"troubleshoot-role": "dns-collector",
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "troubleshoot-dns-",
			Namespace:    namespace,
			Labels:       podLabels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "troubleshoot-dns",
					Image:   image,
					Command: command,
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	created, err := client.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", errors.Wrap(err, "failed to run troubleshoot DNS pod")
	}
	klog.V(2).Infof("Pod with prefix %s has been created", created.GenerateName)

	defer func() {
		if created == nil {
			return
		}
		err := client.CoreV1().Pods(namespace).Delete(ctx, created.Name, metav1.DeleteOptions{})
		if err != nil {
			klog.Errorf("Failed to delete troubleshoot DNS pod %s: %v", created.Name, err)
		}
		klog.V(2).Infof("Deleted pod %s", created.Name)
	}()

	// wait for pod to be completed
	watcher, err := client.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: "troubleshoot-role=dns-collector",
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to watch pod")
	}
	defer func() {
		if watcher != nil {
			watcher.Stop()
		}
	}()

	for event := range watcher.ResultChan() {
		pod, ok := event.Object.(*corev1.Pod)
		if !ok {
			continue
		}
		if pod.Status.Phase == corev1.PodSucceeded {
			break
		}
		if pod.Status.Phase == corev1.PodFailed {
			return "", errors.New("troubleshoot DNS pod failed")
		}
	}

	// get pod logs
	podLogOpts := corev1.PodLogOptions{}
	req := client.CoreV1().Pods(namespace).GetLogs(created.Name, &podLogOpts)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get pod logs")
	}
	defer podLogs.Close()

	bytes, err := io.ReadAll(podLogs)
	if err != nil {
		return "", errors.Wrap(err, "failed to read troubleshoot DNS pod logs")
	}

	return string(bytes), nil
}

func getCoreDNSConfig(client kubernetes.Interface, ctx context.Context) (string, error) {
	configMap, err := client.CoreV1().ConfigMaps("kube-system").Get(ctx, "coredns", metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return configMap.Data["Corefile"], nil
}
