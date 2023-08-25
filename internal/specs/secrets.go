package specs

import (
	"context"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

func LoadFromSecret(ctx context.Context, client kubernetes.Interface, ns string, name string, key string) ([]byte, error) {
	foundSecret, err := client.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get secret")
	}

	spec, ok := foundSecret.Data[key]
	if !ok {
		return nil, errors.Errorf("spec not found in secret %s", name)
	}

	klog.V(1).InfoS("Loaded spec from secret", "name",
		foundSecret.Name, "namespace", foundSecret.Namespace, "data key", key,
	)
	return spec, nil
}

func LoadFromSecretMatchingLabel(ctx context.Context, client kubernetes.Interface, label string, ns string, key string) ([]string, error) {
	var secretsMatchingKey []string

	secrets, err := client.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{LabelSelector: label})
	if err != nil {
		return nil, errors.Wrap(err, "failed to search for secrets in the cluster")
	}

	for _, secret := range secrets.Items {
		spec, ok := secret.Data[key]
		if !ok {
			continue
		}

		klog.V(1).InfoS("Loaded spec from secret", "name", secret.Name,
			"namespace", secret.Namespace, "data key", key, "label selector", label,
		)
		secretsMatchingKey = append(secretsMatchingKey, string(spec))
	}

	return secretsMatchingKey, nil
}
