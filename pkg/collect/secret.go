package collect

import (
	"encoding/json"
	"fmt"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
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

func Secret(secretCollector *troubleshootv1beta1.Secret, redact bool) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}

	secretOutput := &SecretOutput{
		FoundSecret: make(map[string][]byte),
		Errors:      make(map[string][]byte),
	}

	secret, encoded, err := secret(client, secretCollector)
	if err != nil {
		errorBytes, err := marshalNonNil([]string{err.Error()})
		if err != nil {
			return err
		}
		secretOutput.Errors[fmt.Sprintf("%s/%s.json", secret.Namespace, secret.Name)] = errorBytes
	}
	if encoded != nil {
		secretOutput.FoundSecret[fmt.Sprintf("%s/%s.json", secret.Namespace, secret.Name)] = encoded
		if redact {
			secretOutput, err = secretOutput.Redact()
			if err != nil {
				return err
			}
		}
	}

	b, err := json.MarshalIndent(secretOutput, "", "  ")
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", b)

	return nil
}

func secret(client *kubernetes.Clientset, secretCollector *troubleshootv1beta1.Secret) (*FoundSecret, []byte, error) {
	found, err := client.CoreV1().Secrets(secretCollector.Namespace).Get(secretCollector.Name, metav1.GetOptions{})
	if err != nil {
		missingSecret := FoundSecret{
			Namespace:    secretCollector.Namespace,
			Name:         secretCollector.Name,
			SecretExists: false,
		}

		b, marshalErr := json.MarshalIndent(missingSecret, "", "  ")
		if marshalErr != nil {
			return nil, nil, marshalErr
		}

		return &missingSecret, b, err
	}

	keyExists := false
	if secretCollector.Key != "" {
		if _, ok := found.Data[secretCollector.Key]; ok {
			keyExists = true
		}
	}

	secret := FoundSecret{
		Namespace:    found.Namespace,
		Name:         found.Name,
		SecretExists: true,
		KeyExists:    keyExists,
	}

	b, err := json.MarshalIndent(secret, "", "  ")
	if err != nil {
		return nil, nil, err
	}

	return &secret, b, nil
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
