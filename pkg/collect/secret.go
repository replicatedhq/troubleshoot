package collect

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type FoundSecret struct {
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
	Key          string `json:"key"`
	SecretExists bool   `json:"secretExists"`
	KeyExists    bool   `json:"keyExists"`
	Value        string `json:"value,omitempty"`
}

type SecretOutput struct {
	FoundSecret map[string][]byte `json:"secrets/,omitempty"`
	Errors      map[string][]byte `json:"secrets-errors/,omitempty"`
}

func Secret(ctx *Context, secretCollector *troubleshootv1beta1.Secret) ([]byte, error) {
	client, err := kubernetes.NewForConfig(ctx.ClientConfig)
	if err != nil {
		return nil, err
	}

	secretOutput := &SecretOutput{
		FoundSecret: make(map[string][]byte),
		Errors:      make(map[string][]byte),
	}

	path, encoded, err := secret(client, secretCollector)
	if err != nil {
		errorBytes, err := marshalNonNil([]string{err.Error()})
		if err != nil {
			return nil, err
		}
		secretOutput.Errors[path] = errorBytes
	}
	if encoded != nil {
		secretOutput.FoundSecret[path] = encoded
		if ctx.Redact {
			secretOutput, err = secretOutput.Redact()
			if err != nil {
				return nil, err
			}
		}
	}

	b, err := json.MarshalIndent(secretOutput, "", "  ")
	if err != nil {
		return nil, err
	}

	return b, nil
}

func secret(client *kubernetes.Clientset, secretCollector *troubleshootv1beta1.Secret) (string, []byte, error) {
	ns := secretCollector.Namespace
	path := fmt.Sprintf("%s.json", filepath.Join(ns, secretCollector.SecretName))

	found, err := client.CoreV1().Secrets(secretCollector.Namespace).Get(secretCollector.SecretName, metav1.GetOptions{})
	if err != nil {
		missingSecret := FoundSecret{
			Namespace:    secretCollector.Namespace,
			Name:         secretCollector.SecretName,
			SecretExists: false,
		}

		b, marshalErr := json.MarshalIndent(missingSecret, "", "  ")
		if marshalErr != nil {
			return path, nil, marshalErr
		}

		return path, b, err
	}

	ns = found.Namespace
	path = fmt.Sprintf("%s.json", filepath.Join(ns, secretCollector.SecretName, secretCollector.Key))

	keyExists := false
	keyData := ""
	if secretCollector.Key != "" {
		if val, ok := found.Data[secretCollector.Key]; ok {
			keyExists = true
			if secretCollector.IncludeValue {
				keyData = string(val)
			}
		}
	}

	secret := FoundSecret{
		Namespace:    found.Namespace,
		Name:         found.Name,
		SecretExists: true,
		KeyExists:    keyExists,
		Value:        keyData,
	}

	b, err := json.MarshalIndent(secret, "", "  ")
	if err != nil {
		return path, nil, err
	}

	return path, b, nil
}

func (s *SecretOutput) Redact() (*SecretOutput, error) {
	foundSecret, err := redactMap(s.FoundSecret)
	if err != nil {
		return nil, err
	}

	return &SecretOutput{
		FoundSecret: foundSecret,
		Errors:      s.Errors,
	}, nil
}
