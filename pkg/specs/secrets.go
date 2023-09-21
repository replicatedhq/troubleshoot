package specs

import (
	"context"

	"github.com/pkg/errors"
	specs "github.com/replicatedhq/troubleshoot/internal/specs"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"k8s.io/client-go/kubernetes"
)

// LoadFromSecret reads data from a secret in the cluster
// Deprecated: Remove in a future version (v1.0). Future loader functions
// will be created
func LoadFromSecret(namespace string, secretName string, key string) ([]byte, error) {
	config, err := k8sutil.GetRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert create k8s client")
	}

	data, err := specs.LoadFromSecret(context.TODO(), client, namespace, secretName)
	if err != nil {
		return nil, err
	}

	spec, ok := data[key]
	if !ok {
		return nil, errors.Errorf("spec not found in secret %q: key=%q", secretName, key)
	}

	return spec, nil
}

// LoadFromSecretMatchingLabel reads data from a secret in the cluster using a label selector
// Deprecated: Remove in a future version (v1.0). Future loader functions will be created
func LoadFromSecretMatchingLabel(client kubernetes.Interface, labelSelector string, namespace string, key string) ([]string, error) {
	return specs.LoadFromSecretMatchingLabel(context.TODO(), client, labelSelector, namespace, key)
}
