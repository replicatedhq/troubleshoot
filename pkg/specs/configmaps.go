package specs

import (
	"context"

	"github.com/pkg/errors"
	specs "github.com/replicatedhq/troubleshoot/internal/specs"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"k8s.io/client-go/kubernetes"
)

// LoadFromConfigMap reads data from a configmap and returns the list of values extracted from the key
// Deprecated: Remove in a future version (v1.0). Future loader functions
// will be created
func LoadFromConfigMap(namespace string, configMapName string, key string) ([]byte, error) {
	config, err := k8sutil.GetRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert create k8s client")
	}

	data, err := specs.LoadFromConfigMap(context.TODO(), client, namespace, configMapName)
	if err != nil {
		return nil, err
	}

	spec, ok := data[key]
	if !ok {
		return nil, errors.Errorf("spec not found in configmap %q: key=%s", configMapName, key)
	}

	return []byte(spec), nil
}

// LoadFromConfigMapMatchingLabel reads data from a configmap and returns the list of values extracted from the key
// Deprecated: Remove in a future version (v1.0). Future loader functions will be created
func LoadFromConfigMapMatchingLabel(client kubernetes.Interface, labelSelector string, namespace string, key string) ([]string, error) {
	return specs.LoadFromConfigMapMatchingLabel(context.TODO(), client, labelSelector, namespace, key)
}
