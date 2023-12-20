package specs

import (
	"context"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

func LoadFromConfigMap(ctx context.Context, client kubernetes.Interface, ns string, name string) (map[string]string, error) {
	foundConfigMap, err := client.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get configmap")
	}

	klog.V(1).InfoS("Loaded data from config map", "name",
		foundConfigMap.Name, "namespace", foundConfigMap.Namespace,
	)

	return foundConfigMap.Data, nil
}

func LoadFromConfigMapMatchingLabel(ctx context.Context, client kubernetes.Interface, label string, ns string, key string) ([]string, error) {
	var configMapMatchingKey []string

	configMaps, err := client.CoreV1().ConfigMaps(ns).List(ctx, metav1.ListOptions{LabelSelector: label})
	if err != nil {
		return nil, errors.Wrap(err, "failed to search for configmaps in the cluster")
	}

	for _, configMap := range configMaps.Items {
		spec, ok := configMap.Data[key]
		if !ok {
			continue
		}

		klog.V(1).InfoS("Loaded spec from config map", "name", configMap.Name,
			"namespace", configMap.Namespace, "data key", key, "label selector", label,
		)
		configMapMatchingKey = append(configMapMatchingKey, string(spec))
	}

	return configMapMatchingKey, nil
}
