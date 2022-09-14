package specs

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func LoadFromSecret(namespace string, secretName string, key string) ([]byte, error) {
	config, err := k8sutil.GetRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert create k8s client")
	}

	foundSecret, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get secret")
	}

	spec, ok := foundSecret.Data[key]
	if !ok {
		return nil, errors.Errorf("spec not found in secret %s", secretName)
	}

	return spec, nil
}

func LoadFromSecretMatchingLabel(client kubernetes.Interface, labelSelector string, namespace string, key string) ([]string, error) {
	var secretsMatchingKey []string

	secrets, err := client.CoreV1().Secrets(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, errors.Wrap(err, "failed to search for secrets in the cluster")
	}

	for _, secret := range secrets.Items {
		spec, ok := secret.Data[key]
		if !ok {
			logger.Printf("expected key of %s not found in secret %s, skipping\n", key, secret.Name)
			continue
		}
		secretsMatchingKey = append(secretsMatchingKey, string(spec))
	}

	return secretsMatchingKey, nil
}
