package collect

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

const (
	dnsUtilsImage       = "registry.k8s.io/e2e-test-images/agnhost:2.39"
	nonResolvableDomain = "*"
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

// DNSTroubleshootResult represents the structure of the DNS troubleshooting JSON data
type DNSTroubleshootResult struct {
	KubernetesClusterIP string `json:"kubernetesClusterIP"`
	PodResolvConf       string `json:"podResolvConf"`
	Query               struct {
		Kubernetes struct {
			Name    string `json:"name"`
			Address string `json:"address"`
		} `json:"kubernetes"`
		NonResolvableDomain struct {
			Name    string `json:"name"`
			Address string `json:"address"`
		} `json:"nonResolvableDomain"`
	} `json:"query"`
	KubeDNSPods      []string `json:"kubeDNSPods"`
	KubeDNSService   string   `json:"kubeDNSService"`
	KubeDNSEndpoints string   `json:"kubeDNSEndpoints"`
}

func (c *CollectDNS) Title() string {
	return getCollectorName(c)
}

func (c *CollectDNS) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectDNS) SkipRedaction() bool {
	return c.Collector.SkipRedaction
}

func (c *CollectDNS) Collect(progressChan chan<- interface{}) (CollectorResult, error) {

	ctx, cancel := context.WithTimeout(c.Context, time.Duration(60*time.Second))
	defer cancel()

	sb := strings.Builder{}
	dnsDebug := DNSTroubleshootResult{}

	// get kubernetes Cluster IP
	clusterIP, err := getKubernetesClusterIP(c.Client, ctx)
	if err == nil {
		sb.WriteString(fmt.Sprintf("=== Kubernetes Cluster IP from API Server: %s\n", clusterIP))
		dnsDebug.KubernetesClusterIP = clusterIP
	} else {
		sb.WriteString(fmt.Sprintf("=== Failed to detect Kubernetes Cluster IP: %v\n", err))
	}

	// run a pod and perform DNS lookup
	testDomain := c.Collector.NonResolvable
	if testDomain == "" {
		testDomain = nonResolvableDomain
	}
	dnsDebug.Query.NonResolvableDomain.Name = testDomain

	image := c.Collector.Image
	if image == "" {
		image = dnsUtilsImage
	}

	podLog, err := troubleshootDNSFromPod(c.Client, ctx, testDomain, image)
	if err == nil {
		sb.WriteString(fmt.Sprintf("=== Test DNS resolution in pod %s: \n", image))
		sb.WriteString(podLog)
	} else {
		sb.WriteString(fmt.Sprintf("=== Failed to run commands from pod: %v\n", err))
	}

	// extract DNS queries from pod log
	err = extractDNSQueriesFromPodLog(podLog, &dnsDebug)
	if err != nil {
		sb.WriteString(fmt.Sprintf("=== Failed to extract DNS queries from pod log: %v\n", err))
	}

	// is DNS pods running?
	kubeDNSPods := getRunningKubeDNSPodNames(c.Client, ctx)
	sb.WriteString(fmt.Sprintf("=== Running kube-dns pods: %s\n", kubeDNSPods))
	dnsDebug.KubeDNSPods = strings.Split(kubeDNSPods, ", ")

	// is DNS service up?
	kubeDNSService := getKubeDNSServiceClusterIP(c.Client, ctx)
	sb.WriteString(fmt.Sprintf("=== Running kube-dns service: %s\n", kubeDNSService))
	dnsDebug.KubeDNSService = kubeDNSService

	// are DNS endpoints exposed?
	kubeDNSEndpoints := getKubeDNSEndpoints(c.Client, ctx)
	sb.WriteString(fmt.Sprintf("=== kube-dns endpoints: %s\n", kubeDNSEndpoints))
	dnsDebug.KubeDNSEndpoints = kubeDNSEndpoints

	// get DNS server config
	coreDNSConfig, err := getCoreDNSConfig(c.Client, ctx)
	if err == nil {
		sb.WriteString("=== CoreDNS config: \n")
		sb.WriteString(coreDNSConfig)
	}
	kubeDNSConfig, err := getKubeDNSConfig(c.Client, ctx)
	if err == nil {
		sb.WriteString("=== KubeDNS config: \n")
		sb.WriteString(kubeDNSConfig)
	}

	data := sb.String()
	output := NewResult()

	// save raw debug output
	output.SaveResult(c.BundlePath, "dns/debug.txt", bytes.NewBuffer([]byte(data)))

	// save structured debug output as JSON file
	jsonData, err := json.Marshal(dnsDebug)
	if err != nil {
		return output, errors.Wrap(err, "failed to marshal DNS troubleshooting data")
	}
	output.SaveResult(c.BundlePath, "dns/debug.json", bytes.NewBuffer(jsonData))

	return output, nil
}

func getKubernetesClusterIP(client kubernetes.Interface, ctx context.Context) (string, error) {
	service, err := client.CoreV1().Services("default").Get(ctx, "kubernetes", metav1.GetOptions{})
	if err != nil {
		klog.V(2).Infof("Failed to detect Kubernetes Cluster IP: %v", err)
		return "", err
	}

	return service.Spec.ClusterIP, nil
}

func troubleshootDNSFromPod(client kubernetes.Interface, ctx context.Context, nonResolvableDomain string, image string) (string, error) {
	namespace := "default"
	command := []string{"/bin/sh", "-c", fmt.Sprintf(`
		echo "=== /etc/resolv.conf ==="
		cat /etc/resolv.conf
		echo "=== dig kubernetes ==="
		dig +search +short kubernetes
		echo "=== dig non-existent-domain ==="
		dig +search +short %s
		exit 0
	`, nonResolvableDomain)}

	// TODO: image pull secret?
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
		klog.V(2).Infof("Failed to detect CoreDNS config: %v", err)
		return "", err
	}

	return configMap.Data["Corefile"], nil
}

func getKubeDNSConfig(client kubernetes.Interface, ctx context.Context) (string, error) {
	configMap, err := client.CoreV1().ConfigMaps("kube-system").Get(ctx, "kube-dns", metav1.GetOptions{})
	if err != nil {
		klog.V(2).Infof("Failed to detect KubeDNS config: %v", err)
		return "", err
	}

	if configMap.Data == nil {
		return "", nil
	}

	dataBytes, err := json.Marshal(configMap.Data)
	if err != nil {
		return "", err
	}

	return string(dataBytes), nil
}

func getRunningKubeDNSPodNames(client kubernetes.Interface, ctx context.Context) string {
	pods, err := client.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
		LabelSelector: "k8s-app=kube-dns",
	})
	if err != nil {
		klog.V(2).Infof("failed to list kube-dns pods: %v", err)
		return ""
	}

	var podNames []string
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			podNames = append(podNames, pod.Name)
		}
	}

	return strings.Join(podNames, ", ")
}

func getKubeDNSServiceClusterIP(client kubernetes.Interface, ctx context.Context) string {
	service, err := client.CoreV1().Services("kube-system").Get(ctx, "kube-dns", metav1.GetOptions{})
	if err != nil {
		klog.V(2).Infof("failed to get kube-dns service: %v", err)
		return ""
	}

	return service.Spec.ClusterIP
}

func getKubeDNSEndpoints(client kubernetes.Interface, ctx context.Context) string {
	endpoints, err := client.CoreV1().Endpoints("kube-system").Get(ctx, "kube-dns", metav1.GetOptions{})
	if err != nil {
		klog.V(2).Infof("failed to get kube-dns endpoints: %v", err)
		return ""
	}

	var endpointStrings []string
	for _, subset := range endpoints.Subsets {
		for _, address := range subset.Addresses {
			if len(subset.Ports) > 0 {
				endpointStrings = append(endpointStrings, fmt.Sprintf("%s:%d", address.IP, subset.Ports[0].Port))
			}
		}
	}

	return strings.Join(endpointStrings, ", ")
}

func extractDNSQueriesFromPodLog(podLog string, dnsDebug *DNSTroubleshootResult) error {
	scanner := bufio.NewScanner(strings.NewReader(podLog))

	var currentSection string

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case strings.Contains(line, "=== /etc/resolv.conf ==="):
			currentSection = "podResolvConf"
		case strings.Contains(line, "=== dig kubernetes ==="):
			currentSection = "kubernetes"
		case strings.Contains(line, "=== dig non-existent-domain ==="):
			currentSection = "nonResolvableDomain"
		default:
			switch currentSection {
			case "podResolvConf":
				dnsDebug.PodResolvConf += line + "\n"
			case "kubernetes":
				dnsDebug.Query.Kubernetes.Name = "kubernetes"
				dnsDebug.Query.Kubernetes.Address = line
			case "nonResolvableDomain":
				dnsDebug.Query.NonResolvableDomain.Address = line
			}
		}
	}

	return nil

}
