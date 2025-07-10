package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"
)

const etcdOutputDir = "etcd"

type CollectEtcd struct {
	Collector    *troubleshootv1beta2.Etcd
	BundlePath   string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

// etcdDebug is a helper struct to exec into an etcd pod
type etcdDebug struct {
	context      context.Context
	clientConfig *rest.Config
	client       kubernetes.Interface
	pod          *corev1.Pod // etcd pod to exec into
	ephemeral    bool        // if true, the pod will be deleted after the collector is done
	commands     []string    // list of commands to run in the etcd pod
	args         []string    // list of args to pass to each command
	hostPath     string      // path to the host's etcd certs
	image        string      // image to use for the etcd client pod
}

func (c *CollectEtcd) Title() string {
	return getCollectorName(c)
}

func (c *CollectEtcd) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectEtcd) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	debugInstance := etcdDebug{
		context:      c.Context,
		clientConfig: c.ClientConfig,
		client:       c.Client,
		commands: []string{
			"etcdctl endpoint health",
			"etcdctl endpoint status",
			"etcdctl member list",
			"etcdctl alarm list",
			"etcdctl version",
		},
		image: c.Collector.Image,
	}

	distribution, err := debugInstance.getSupportedDistro()
	if err != nil {
		klog.V(2).Infof("etcd collector is not supported on this distribution: %v", err)
		return nil, err
	}

	// etcd on these distros are not running as pod but as a process managed by k0scontroller
	// we have to spin up an etcd pod to exec into and run the commands
	// after the collector is done, the pod will be deleted
	if distribution == "k0s" || distribution == "embedded-cluster" {
		debugInstance.ephemeral = true
	}
	defer debugInstance.cleanup()

	etcdArgs, hostPath, err := getEtcdArgsByDistribution(distribution)
	if err != nil {
		return nil, err
	}
	debugInstance.args = etcdArgs
	debugInstance.hostPath = hostPath

	err = debugInstance.getOrCreateEtcdPod()
	if err != nil {
		return nil, err
	}

	// wait until the pod is running
	err = debugInstance.waitForPodReady()
	if err != nil {
		return nil, err
	}

	// finally exec etcdctl troubleshoot commands
	output := NewResult()

	for _, command := range debugInstance.commands {
		fileName := generateFilenameFromCommand(command)
		stdout, stderr, err := debugInstance.executeCommand(command)
		if err != nil {
			klog.V(2).Infof("failed to exec command %s: %v", command, err)
			continue
		}
		if len(stdout) > 0 {
			output.SaveResult(c.BundlePath, getFullPath(fileName), bytes.NewBuffer(stdout))
		}
		if len(stderr) > 0 {
			fileName := fmt.Sprintf("%s-stderr", fileName)
			output.SaveResult(c.BundlePath, getFullPath(fileName), bytes.NewBuffer(stderrToJson(stderr)))
		}
	}

	return output, nil
}

func getEtcdArgsByDistribution(distribution string) ([]string, string, error) {
	type certs struct {
		hostPath string
		ca       string
		cert     string
		key      string
	}

	lookup := map[string]certs{
		"k0s": {
			hostPath: "/var/lib/k0s/pki/etcd",
			ca:       "ca.crt",
			cert:     "peer.crt",
			key:      "peer.key",
		},
		"embedded-cluster": {
			hostPath: "/var/lib/k0s/pki/etcd",
			ca:       "ca.crt",
			cert:     "peer.crt",
			key:      "peer.key",
		},
		"kurl": {
			hostPath: "/etc/kubernetes/pki/etcd",
			ca:       "ca.crt",
			cert:     "healthcheck-client.crt",
			key:      "healthcheck-client.key",
		},
	}

	c, ok := lookup[distribution]
	if !ok {
		return nil, "", errors.Errorf("distribution %s not supported", distribution)
	}

	return []string{
		"--cacert", c.hostPath + "/" + c.ca,
		"--cert", c.hostPath + "/" + c.cert,
		"--key", c.hostPath + "/" + c.key,
		"--write-out", "json",
		"--endpoints", "https://127.0.0.1:2379", // always use localhost
	}, c.hostPath, nil
}

// getSupportedDistro returns the distro that etcd collector can run on
// either due to the distro has static etcd pod (kurl by kubeadm) or
// the distro has etcd running as a process (k0s, embedded-cluster)
func (c *etcdDebug) getSupportedDistro() (string, error) {
	// extract distro logic from analyzer.ParseNodesForProviders
	// pkg/analyze/distribution.go
	// we can't import analyzer because of circular dependency
	// TODO: may refactor this to a common package

	nodes, err := c.client.CoreV1().Nodes().List(c.context, metav1.ListOptions{})
	if err != nil {
		return "", errors.Wrap(err, "failed to list nodes")
	}

	for _, node := range nodes.Items {
		for k, v := range node.ObjectMeta.Labels {
			if k == "kurl.sh/cluster" && v == "true" {
				return "kurl", nil
			}
			if k == "node.k0sproject.io/role" {
				return "k0s", nil
			}
			if k == "kots.io/embedded-cluster-role" {
				return "embedded-cluster", nil
			}
		}
	}

	return "", errors.New("current k8s distribution does not support etcd collector")
}

func (c *etcdDebug) getOrCreateEtcdPod() error {
	// if ephemeral, create a etcd client pod to exec into
	// the pod will use hostNetwork: true to access the etcd server
	if c.ephemeral {
		err := c.createEtcdPod()
		if err != nil {
			return errors.Wrap(err, "failed to create etcd pod")
		}
		return nil
	}
	// if not ephemeral, find the static etcd pod to exec into
	// get the first etcd pod in the cluster with label "component=etcd" in all namespaces
	label := "component=etcd"
	pods, err := c.client.CoreV1().Pods("").List(c.context, metav1.ListOptions{
		LabelSelector: label,
	})
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to list etcd pods with label %s", label))
	}
	if len(pods.Items) == 0 {
		return errors.New("no static etcd pod found")
	}

	klog.V(2).Infof("found etcd pod %s in namespace %s", pods.Items[0].Name, pods.Items[0].Namespace)
	c.pod = &pods.Items[0]
	return nil
}

// createEtcdPod creates a etcd client pod to exec into
func (c *etcdDebug) createEtcdPod() error {
	if c.image == "" {
		c.image = "quay.io/coreos/etcd:latest"
	}
	namespace := "default"
	labels := map[string]string{
		"troubleshoot-role": "etcd-collector",
	}
	spec := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "etcd-collector-",
			Namespace:    namespace,
			Labels:       labels,
		},
		Spec: corev1.PodSpec{
			HostNetwork: true,
			Containers: []corev1.Container{
				{
					Name:    "etcd-client",
					Image:   c.image,
					Command: []string{"sleep"},
					Args:    []string{"5m"},
					Env: []corev1.EnvVar{
						{
							Name:  "ETCDCTL_API",
							Value: "3",
						}, {
							Name:  "ETCDCTL_INSECURE_SKIP_TLS_VERIFY",
							Value: "true",
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "etcd-certs",
							MountPath: c.hostPath,
							ReadOnly:  true,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "etcd-certs",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: c.hostPath,
						},
					},
				},
			},
		},
	}

	klog.V(2).Infof("creating etcd troubleshoot pod in namespace %s", namespace)
	pod, err := c.client.CoreV1().Pods(namespace).Create(c.context, spec, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to create etcd troubleshoot pod")
	}
	c.pod = pod
	return nil
}

// cleanup deletes the etcd troubleshoot pod if it's ephemeral
func (c *etcdDebug) cleanup() {
	if !c.ephemeral || c.pod == nil {
		return
	}

	// delete the pod
	klog.V(2).Infof("deleting etcd troubleshoot pod %s in namespace %s", c.pod.Name, c.pod.Namespace)
	err := c.client.CoreV1().Pods(c.pod.Namespace).Delete(context.Background(), c.pod.Name, metav1.DeleteOptions{
		GracePeriodSeconds: new(int64), // delete immediately
	})
	if err != nil {
		klog.Errorf("failed to delete pod %s: %v", c.pod.Name, err)
	}
}

// executeCommand exec into the pod and run the command
// it returns the stdout, stderr and error if any of the command
func (c *etcdDebug) executeCommand(command string) ([]byte, []byte, error) {

	// split command into a slice of strings
	// e.g. "etcdctl endpoint health" -> ["etcdctl", "endpoint", "health"]
	cdmArgs := strings.Fields(command)
	cdmArgs = append(cdmArgs, c.args...)
	klog.V(2).Infof("executing command: %q in pod %q (namespace %q)", strings.Join(cdmArgs, " "), c.pod.Name, c.pod.Namespace)

	req := c.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(c.pod.Name).
		Namespace(c.pod.Namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Command: cdmArgs,
		Stdin:   false,
		Stdout:  true,
		Stderr:  true,
		TTY:     false,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.clientConfig, "POST", req.URL())
	if err != nil {
		return nil, nil, err
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(c.context, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	return stdout.Bytes(), stderr.Bytes(), err
}

// waitForPodReady waits until the etcd troubleshoot pod is running
func (c *etcdDebug) waitForPodReady() error {
	timeout := 60 * time.Second
	ticker := time.NewTicker(1 * time.Second)

	ctx, cancel := context.WithTimeout(c.context, timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return errors.New("timeout waiting for etcd troubleshooting pod to be running")
		case <-ticker.C:
			pod, err := c.client.CoreV1().Pods(c.pod.Namespace).Get(c.context, c.pod.Name, metav1.GetOptions{})
			if err != nil {
				return errors.Wrap(err, "failed to get etcd troubleshoot pod")
			}
			if pod.Status.Phase == corev1.PodRunning {
				// ok, pod is running
				return nil
			}
			klog.V(2).Infof("waiting for etcd troubleshoot pod %q to be running, current status: %q", c.pod.Name, pod.Status.Phase)
		}
	}
}

// generateFilenameFromCommand generates a filename from the command
// e.g. "etcdctl endpoint health" -> "endpoint-health"
func generateFilenameFromCommand(command string) string {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts[1:], "-")
}

// getFullPath returns the full path to the file
// e.g. "endpoint-health" -> "etcd/endpoint-health.json"
func getFullPath(fileName string) string {
	return fmt.Sprintf("%s/%s.json", etcdOutputDir, fileName)
}

// stderrToJson converts stderr output to json bytes
func stderrToJson(stderr []byte) []byte {
	jsonObj := map[string]string{
		"stderr": string(stderr),
	}
	jsonBytes, err := json.Marshal(jsonObj)
	if err != nil {
		klog.Errorf("failed to marshal stderr to json: %v", err)
		return []byte{}
	}
	return jsonBytes
}
