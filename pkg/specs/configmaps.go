package specs

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func LoadFromConfigMap(namespace string, configMapName string, key string) ([]byte, error) {
	config, err := k8sutil.GetRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert create k8s client")
	}

	foundConfigMap, err := client.CoreV1().ConfigMaps(namespace).Get(context.TODO(), configMapName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get configmap")
	}

	spec, ok := foundConfigMap.Data[key]
	if !ok {
		return nil, errors.Errorf("spec not found in configmap %s", configMapName)
	}

	return []byte(spec), nil
}

func LoadFromConfigMapMatchingLabel(client kubernetes.Interface, labelSelector string, namespace string, key string) ([]string, error) {
	var configMapMatchingKey []string

	configMaps, err := client.CoreV1().ConfigMaps(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, errors.Wrap(err, "failed to search for configmaps in the cluster")
	}

	for _, configMap := range configMaps.Items {
		spec, ok := configMap.Data[key]
		if !ok {
			logger.Printf("expected key of %s not found in secret %s, skipping\n", key, configMap.Name)
			continue
		}
		configMapMatchingKey = append(configMapMatchingKey, string(spec))
	}

	return configMapMatchingKey, nil
}
