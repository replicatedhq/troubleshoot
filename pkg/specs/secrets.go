package specs

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
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
			continue
		}
		secretsMatchingKey = append(secretsMatchingKey, string(spec))
	}

	return secretsMatchingKey, nil
}

func GetSecretMatchingLabel(client kubernetes.Interface, labelSelector string, namespace string, key string) (*corev1.Secret, error) {

	secrets, err := client.CoreV1().Secrets(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, err
	}

	for _, secret := range secrets.Items {
		_, ok := secret.Data[key]
		if !ok {
			continue
		}
		return &secret, nil
	}

	return nil, kuberneteserrors.NewNotFound(corev1.Resource("secret"), "support bundle spec")
}
