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

type ConfigMapOutput struct {
	Namespace       string `json:"namespace"`
	Name            string `json:"name"`
	Key             string `json:"key"`
	ConfigMapExists bool   `json:"configMapExists"`
	KeyExists       bool   `json:"keyExists"`
	Value           string `json:"value,omitempty"`
}

func ConfigMap(c *Collector, configMapCollector *troubleshootv1beta2.ConfigMap) (map[string][]byte, error) {
	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, err
	}

	output := map[string][]byte{}

	ctx := context.Background()

	configMaps := []corev1.ConfigMap{}
	if configMapCollector.Name != "" {
		configMap, err := client.CoreV1().ConfigMaps(configMapCollector.Namespace).Get(ctx, configMapCollector.Name, metav1.GetOptions{})
		if kuberneteserrors.IsNotFound(err) {
			missingConfigMap := ConfigMapOutput{
				Namespace:       configMapCollector.Namespace,
				Name:            configMapCollector.Name,
				ConfigMapExists: false,
			}
			path, b, err := marshalConfigMapOutput(configMapCollector, missingConfigMap)
			if err != nil {
				return output, errors.Wrap(err, "marshal found configMap")
			}
			output[path] = b
			return output, nil
		} else if err != nil {
			errorBytes, err := marshalNonNil([]string{err.Error()})
			if err != nil {
				return nil, errors.Wrapf(err, "marshal configMap %s error non nil", configMapCollector.Name)
			}
			output[getConfigMapErrorsFileName(configMapCollector)] = errorBytes
			return output, nil
		}
		configMaps = append(configMaps, *configMap)
	} else if len(configMapCollector.Selector) > 0 {
		cms, err := listConfigMapsForSelector(ctx, client, configMapCollector.Namespace, configMapCollector.Selector)
		if err != nil {
			errorBytes, err := marshalNonNil([]string{err.Error()})
			if err != nil {
				return nil, errors.Wrap(err, "marshal selector error non nil")
			}
			output[getConfigMapErrorsFileName(configMapCollector)] = errorBytes
			return output, nil
		}
		configMaps = append(configMaps, cms...)
	} else {
		return nil, errors.New("either name or selector must be specified")
	}

	for _, configMap := range configMaps {
		filePath, encoded, err := configMapToOutput(configMapCollector, configMap)
		if err != nil {
			return output, errors.Wrapf(err, "collect configMap %s", configMap.Name)
		}
		output[filePath] = encoded
	}

	return output, nil
}

func configMapToOutput(configMapCollector *troubleshootv1beta2.ConfigMap, configMap corev1.ConfigMap) (string, []byte, error) {
	keyExists := false
	keyData := ""
	configMapKey := ""
	if configMapCollector.Key != "" {
		configMapKey = configMapCollector.Key
		if val, ok := configMap.Data[configMapCollector.Key]; ok {
			keyExists = true
			if configMapCollector.IncludeValue {
				keyData = string(val)
			}
		}
	}

	foundConfigMap := ConfigMapOutput{
		Namespace:       configMap.Namespace,
		Name:            configMap.Name,
		Key:             configMapKey,
		ConfigMapExists: true,
		KeyExists:       keyExists,
		Value:           keyData,
	}
	return marshalConfigMapOutput(configMapCollector, foundConfigMap)
}

func listConfigMapsForSelector(ctx context.Context, client *kubernetes.Clientset, namespace string, selector []string) ([]corev1.ConfigMap, error) {
	serializedLabelSelector := strings.Join(selector, ",")

	listOptions := metav1.ListOptions{
		LabelSelector: serializedLabelSelector,
	}

	configMaps, err := client.CoreV1().ConfigMaps(namespace).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	return configMaps.Items, nil
}

func marshalConfigMapOutput(configMapCollector *troubleshootv1beta2.ConfigMap, configMap ConfigMapOutput) (string, []byte, error) {
	path := getConfigMapFileName(configMapCollector, configMap.Name)

	b, err := json.MarshalIndent(configMap, "", "  ")
	if err != nil {
		return path, nil, err
	}

	return path, b, nil
}

func getConfigMapFileName(configMapCollector *troubleshootv1beta2.ConfigMap, name string) string {
	if configMapCollector.CollectorName != "" {
		return filepath.Join("configMaps", configMapCollector.CollectorName, configMapCollector.Namespace, fmt.Sprintf("%s.json", name))
	}
	return filepath.Join("configMaps", configMapCollector.Namespace, fmt.Sprintf("%s.json", name))
}

func getConfigMapErrorsFileName(configMapCollector *troubleshootv1beta2.ConfigMap) string {
	var filename string
	if configMapCollector.Name != "" {
		filename = configMapCollector.Name
	} else {
		filename = selectorToString(configMapCollector.Selector)
	}
	if configMapCollector.CollectorName != "" {
		return filepath.Join("configmaps-errors", configMapCollector.CollectorName, configMapCollector.Namespace, fmt.Sprintf("%s.json", filename))
	}
	return filepath.Join("configmaps-errors", configMapCollector.Namespace, fmt.Sprintf("%s.json", filename))
}
