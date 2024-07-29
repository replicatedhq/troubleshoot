package collect

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

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
	context   context.Context
	client    kubernetes.Interface
	pod       *corev1.Pod // etcd pod to exec into
	ephemeral bool        // if true, the pod will be deleted after the collector is done
	commands  []string    // list of commands to run in the etcd pod
	args      []string    // list of args to pass to each command
	hostPath  string      // path to the host's etcd certs
}

func (c *CollectEtcd) Title() string {
	return getCollectorName(c)
}

func (c *CollectEtcd) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectEtcd) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	debugInstance := etcdDebug{
		context: c.Context,
		client:  c.Client,
		commands: []string{
			"etcdctl endpoint health",
			"etcdctl endpoint status",
			"etcdctl member list",
			"etcdctl alarm list",
		},
	}

	distribution, err := debugInstance.getSupportedDistro()
	if err != nil {
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

	// finally exec etcdctl troubleshoot commands

	return nil, nil
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
			hostPath: "/var/lib/k0s/pki",
			ca:       "ca.crt",
			cert:     "apiserver-etcd-client.crt",
			key:      "apiserver-etcd-client.key",
		},
		"embedded-cluster": {
			hostPath: "/var/lib/k0s/pki",
			ca:       "ca.crt",
			cert:     "apiserver-etcd-client.crt",
			key:      "apiserver-etcd-client.key",
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
	pods, err := c.client.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
		LabelSelector: label,
	})
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to list etcd pods with label %s", label))
	}
	if len(pods.Items) == 0 {
		return errors.New("no static etcd pod found")
	}
	c.pod = &pods.Items[0]
	return nil
}

func (c *etcdDebug) cleanup() {
	if !c.ephemeral || c.pod == nil {
		return
	}

	// delete the pod
	err := c.client.CoreV1().Pods(c.pod.Namespace).Delete(context.Background(), c.pod.Name, metav1.DeleteOptions{
		GracePeriodSeconds: new(int64), // delete immediately
	})
	if err != nil {
		klog.Errorf("failed to delete pod %s: %v", c.pod.Name, err)
	}
}

func (c *etcdDebug) createEtcdPod() error {
	namespace := "default"
	labels := map[string]string{
		"troubleshoot-role": "etcd-collector",
	}
	spec := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "etcd-collector",
			Namespace:    namespace,
			Labels:       labels,
		},
		Spec: corev1.PodSpec{
			HostNetwork: true,
			Containers: []corev1.Container{
				{
					Name:    "etcd-client",
					Image:   "quay.io/coreos/etcd:latest",
					Command: []string{"sleep"},
					Args:    []string{"1d"},
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

	pod, err := c.client.CoreV1().Pods(namespace).Create(c.context, spec, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to create etcd troubleshoot pod")
	}
	c.pod = pod
	return nil
}
