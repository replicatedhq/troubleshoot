package specs

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
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
