package collect

import (
	"bytes"
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
	"k8s.io/client-go/rest"
)

type SecretOutput struct {
	Namespace    string            `json:"namespace"`
	Name         string            `json:"name"`
	Key          string            `json:"key"`
	SecretExists bool              `json:"secretExists"`
	KeyExists    bool              `json:"keyExists"`
	Value        string            `json:"value,omitempty"`
	Data         map[string]string `json:"data,omitempty"`
}

type CollectSecret struct {
	Collector    *troubleshootv1beta2.Secret
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectSecret) Title() string {
	return getCollectorName(c)
}

func (c *CollectSecret) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectSecret) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	output := NewResult()

	secrets := []corev1.Secret{}
	if c.Collector.Name != "" {
		secret, err := c.Client.CoreV1().Secrets(c.Collector.Namespace).Get(c.Context, c.Collector.Name, metav1.GetOptions{})
		if err != nil {
			if kuberneteserrors.IsNotFound(err) {
				filePath, encoded, err := secretToOutput(c.Collector, nil, c.Collector.Name)
				if err != nil {
					return output, errors.Wrapf(err, "collect secret %s", c.Collector.Name)
				}
				output.SaveResult(c.BundlePath, filePath, bytes.NewBuffer(encoded))
			}
			output.SaveResult(c.BundlePath, GetSecretErrorsFileName(c.Collector), marshalErrors([]string{err.Error()}))
			return output, nil
		}
		secrets = append(secrets, *secret)
	} else if len(c.Collector.Selector) > 0 {
		ss, err := listSecretsForSelector(c.Context, c.Client, c.Collector.Namespace, c.Collector.Selector)
		if err != nil {
			output.SaveResult(c.BundlePath, GetSecretErrorsFileName(c.Collector), marshalErrors([]string{err.Error()}))
			return output, nil
		}
		secrets = append(secrets, ss...)
	} else {
		return nil, errors.New("either name or selector must be specified")
	}

	for _, secret := range secrets {
		filePath, encoded, err := secretToOutput(c.Collector, &secret, secret.Name)
		if err != nil {
			return output, errors.Wrapf(err, "collect secret %s", secret.Name)
		}
		output.SaveResult(c.BundlePath, filePath, bytes.NewBuffer(encoded))
	}

	return output, nil
}

func secretToOutput(secretCollector *troubleshootv1beta2.Secret, secret *corev1.Secret, secretName string) (string, []byte, error) {
	foundSecret := SecretOutput{
		Namespace: secretCollector.Namespace,
		Name:      secretName,
		Key:       secretCollector.Key,
	}

	if secret != nil {
		foundSecret.SecretExists = true

		if secretCollector.IncludeAllData {
			// Just give them all the data - they can find what they need
			foundSecret.Data = make(map[string]string)
			for k, v := range secret.Data {
				foundSecret.Data[k] = string(v)
			}
		} else if secretCollector.Key != "" {
			// Only do key-specific logic if they're NOT asking for all data
			if val, ok := secret.Data[secretCollector.Key]; ok {
				foundSecret.KeyExists = true
				if secretCollector.IncludeValue {
					foundSecret.Value = string(val)
				}
			}
		}
	}

	return marshalSecretOutput(secretCollector, foundSecret)
}

func listSecretsForSelector(ctx context.Context, client kubernetes.Interface, namespace string, selector []string) ([]corev1.Secret, error) {
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
	path := GetSecretFileName(secretCollector, secret.Name)

	b, err := json.MarshalIndent(secret, "", "  ")
	if err != nil {
		return path, nil, err
	}

	return path, b, nil
}

func GetSecretFileName(secretCollector *troubleshootv1beta2.Secret, name string) string {
	parts := []string{"secrets", secretCollector.Namespace, name}
	// Only include key in filename when doing key-specific processing
	if secretCollector.Key != "" && !secretCollector.IncludeAllData {
		parts = append(parts, secretCollector.Key)
	}
	return fmt.Sprintf("%s.json", filepath.Join(parts...))
}

func GetSecretErrorsFileName(secretCollector *troubleshootv1beta2.Secret) string {
	parts := []string{"secrets-errors", secretCollector.Namespace}
	if secretCollector.Name != "" {
		parts = append(parts, secretCollector.Name)
	} else {
		parts = append(parts, selectorToString(secretCollector.Selector))
	}
	// Only include key in filename when doing key-specific processing
	if secretCollector.Key != "" && !secretCollector.IncludeAllData {
		parts = append(parts, secretCollector.Key)
	}
	return fmt.Sprintf("%s.json", filepath.Join(parts...))
}
