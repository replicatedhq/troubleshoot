package collect

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type SecretOutput struct {
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
	Key          string `json:"key"`
	SecretExists bool   `json:"secretExists"`
	KeyExists    bool   `json:"keyExists"`
	Value        string `json:"value,omitempty"`
}

func Secret(c *Collector, secretCollector *troubleshootv1beta2.Secret) (map[string][]byte, error) {
	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, err
	}

	output := map[string][]byte{}

	ctx := context.Background()

	secrets := []corev1.Secret{}
	if secretCollector.Name != "" {
		secret, err := client.CoreV1().Secrets(secretCollector.Namespace).Get(ctx, secretCollector.Name, metav1.GetOptions{})
		if err != nil {
			if kuberneteserrors.IsNotFound(err) {
				filePath, encoded, err := secretToOutput(secretCollector, nil, secretCollector.Name)
				if err != nil {
					return output, errors.Wrapf(err, "collect secret %s", secretCollector.Name)
				}
				output[filePath] = encoded
			}
			errorBytes, err := marshalNonNil([]string{err.Error()})
			if err != nil {
				return nil, errors.Wrapf(err, "marshal secret %s error non nil", secretCollector.Name)
			}
			output[getSecretErrorsFileName(secretCollector)] = errorBytes
			return output, nil
		}
		secrets = append(secrets, *secret)
	} else if len(secretCollector.Selector) > 0 {
		ss, err := listSecretsForSelector(ctx, client, secretCollector.Namespace, secretCollector.Selector)
		if err != nil {
			errorBytes, err := marshalNonNil([]string{err.Error()})
			if err != nil {
				return nil, errors.Wrap(err, "marshal selector error non nil")
			}
			output[getSecretErrorsFileName(secretCollector)] = errorBytes
			return output, nil
		}
		secrets = append(secrets, ss...)
	} else {
		return nil, errors.New("either name or selector must be specified")
	}

	for _, secret := range secrets {
		filePath, encoded, err := secretToOutput(secretCollector, &secret, secret.Name)
		if err != nil {
			return output, errors.Wrapf(err, "collect secret %s", secret.Name)
		}
		output[filePath] = encoded
	}

	return output, nil
}

func secretToOutput(secretCollector *troubleshootv1beta2.Secret, secret *corev1.Secret, secretName string) (string, []byte, error) {
	foundSecret := SecretOutput{
		Namespace:    secretCollector.Namespace,
		Name:         secretName,
		SecretExists: true,
		Key:          secretCollector.Key,
	}

	if secret != nil && secretCollector.Key != "" {
		if val, ok := secret.Data[secretCollector.Key]; ok {
			foundSecret.KeyExists = true
			if secretCollector.IncludeValue {
				foundSecret.Value = string(val)
			}
		}
	}

	return marshalSecretOutput(secretCollector, foundSecret)
}

func listSecretsForSelector(ctx context.Context, client *kubernetes.Clientset, namespace string, selector []string) ([]corev1.Secret, error) {
	serializedLabelSelector := strings.Join(selector, ",")

	listOptions := metav1.ListOptions{
		LabelSelector: serializedLabelSelector,
	}

	secrets, err := client.CoreV1().Secrets(namespace).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	return secrets.Items, nil
}

func marshalSecretOutput(secretCollector *troubleshootv1beta2.Secret, secret SecretOutput) (string, []byte, error) {
	path := getSecretFileName(secretCollector, secret.Name)

	b, err := json.MarshalIndent(secret, "", "  ")
	if err != nil {
		return path, nil, err
	}

	return path, b, nil
}

func getSecretFileName(secretCollector *troubleshootv1beta2.Secret, name string) string {
	parts := []string{"secrets", secretCollector.Namespace, name}
	if secretCollector.Key != "" {
		parts = append(parts, secretCollector.Key)
	}
	return fmt.Sprintf("%s.json", filepath.Join(parts...))
}

func getSecretErrorsFileName(secretCollector *troubleshootv1beta2.Secret) string {
	parts := []string{"secrets-errors", secretCollector.Namespace}
	if secretCollector.Name != "" {
		parts = append(parts, secretCollector.Name)
	} else {
		parts = append(parts, selectorToString(secretCollector.Selector))
	}
	if secretCollector.Key != "" {
		parts = append(parts, secretCollector.Key)
	}
	return fmt.Sprintf("%s.json", filepath.Join(parts...))
}
