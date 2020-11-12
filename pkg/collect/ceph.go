package collect

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	DefaultCephNamespace = "rook-ceph"
)

func CephStatus(c *Collector, cephStatusCollector *troubleshootv1beta2.CephStatus) (map[string][]byte, error) {
	ctx := context.TODO()

	if cephStatusCollector.Namespace == "" {
		cephStatusCollector.Namespace = DefaultCephNamespace
	}

	pod, err := findRookCephToolsPod(ctx, c, cephStatusCollector.Namespace)
	if err != nil {
		return nil, err
	}

	execCollector := &troubleshootv1beta2.Exec{
		CollectorMeta: cephStatusCollector.CollectorMeta,
		Name:          cephStatusCollector.Name,
		Selector:      labelsToSelector(pod.Labels),
		Namespace:     cephStatusCollector.Namespace,
		Command:       []string{"ceph", "status"},
		Timeout:       cephStatusCollector.Timeout,
	}
	results, err := Exec(c, execCollector)
	if err != nil {
		return nil, errors.Wrap(err, "failed to exec")
	}

	final := map[string][]byte{}
	for filename, result := range results {
		pathPrefix := GetCephCollectorFilepath(cephStatusCollector.Name, cephStatusCollector.Namespace)
		switch {
		case strings.HasSuffix(filename, "-stdout.txt"):
			final[path.Join(pathPrefix, "status.txt")] = result
		case strings.HasSuffix(filename, "-stderr.txt"):
			final[path.Join(pathPrefix, "status-stderr.txt")] = result
		case strings.HasSuffix(filename, "-errors.json"):
			final[path.Join(pathPrefix, "status-errors.json")] = result
		}
	}
	return final, nil
}

func findRookCephToolsPod(ctx context.Context, c *Collector, namespace string) (*corev1.Pod, error) {
	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create kubernetes client")
	}

	pods, _ := listPodsInSelectors(ctx, client, namespace, []string{"app=rook-ceph-tools"})
	if len(pods) > 0 {
		return &pods[0], nil
	}

	pods, _ = listPodsInSelectors(ctx, client, namespace, []string{"app=rook-ceph-operator"})
	if len(pods) > 0 {
		return &pods[0], nil
	}

	return nil, errors.New("rook ceph tools pod not found")
}

func labelsToSelector(labels map[string]string) []string {
	selector := []string{}
	for key, value := range labels {
		selector = append(selector, fmt.Sprintf("%s=%s", key, value))
	}
	return selector
}

func GetCephCollectorFilepath(name, namespace string) string {
	parts := []string{}
	if name != "" {
		parts = append(parts, name)
	}
	if namespace != "" && namespace != DefaultCephNamespace {
		parts = append(parts, namespace)
	}
	return path.Join(append(parts, "ceph")...)
}
