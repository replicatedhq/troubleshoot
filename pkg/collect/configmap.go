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
	Namespace       string            `json:"namespace"`
	Name            string            `json:"name"`
	Key             string            `json:"key"`
	ConfigMapExists bool              `json:"configMapExists"`
	KeyExists       bool              `json:"keyExists"`
	Value           string            `json:"value,omitempty"`
	Data            map[string]string `json:"data,omitonempty"`
}

func ConfigMap(ctx context.Context, client kubernetes.Interface, configMapCollector *troubleshootv1beta2.ConfigMap) (map[string][]byte, error) {
	output := map[string][]byte{}

	configMaps := []corev1.ConfigMap{}
	if configMapCollector.Name != "" {
		configMap, err := client.CoreV1().ConfigMaps(configMapCollector.Namespace).Get(ctx, configMapCollector.Name, metav1.GetOptions{})
		if err != nil {
			if kuberneteserrors.IsNotFound(err) {
				filePath, encoded, err := configMapToOutput(configMapCollector, nil, configMapCollector.Name)
				if err != nil {
					return output, errors.Wrapf(err, "collect secret %s", configMapCollector.Name)
				}
				output[filePath] = encoded
			}
			errorBytes, err := marshalNonNil([]string{err.Error()})
			if err != nil {
				return nil, errors.Wrapf(err, "marshal configmap %s error non nil", configMapCollector.Name)
			}
			output[GetConfigMapErrorsFileName(configMapCollector)] = errorBytes
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
			output[GetConfigMapErrorsFileName(configMapCollector)] = errorBytes
			return output, nil
		}
		configMaps = append(configMaps, cms...)
	} else {
		return nil, errors.New("either name or selector must be specified")
	}

	for _, configMap := range configMaps {
		filePath, encoded, err := configMapToOutput(configMapCollector, &configMap, configMap.Name)
		if err != nil {
			return output, errors.Wrapf(err, "collect configMap %s", configMap.Name)
		}
		output[filePath] = encoded
	}

	return output, nil
}

func configMapToOutput(configMapCollector *troubleshootv1beta2.ConfigMap, configMap *corev1.ConfigMap, configMapName string) (string, []byte, error) {
	foundConfigMap := ConfigMapOutput{
		Namespace: configMapCollector.Namespace,
		Name:      configMapName,
		Key:       configMapCollector.Key,
	}

	if configMap != nil {
		foundConfigMap.ConfigMapExists = true
		if configMapCollector.IncludeAllData {
			foundConfigMap.Data = configMap.Data
		}
		if configMapCollector.Key != "" {
			if val, ok := configMap.Data[configMapCollector.Key]; ok {
				foundConfigMap.KeyExists = true
				if configMapCollector.IncludeValue {
					foundConfigMap.Value = string(val)
				}
			}
		}
	}

	return marshalConfigMapOutput(configMapCollector, foundConfigMap)
}

func listConfigMapsForSelector(ctx context.Context, client kubernetes.Interface, namespace string, selector []string) ([]corev1.ConfigMap, error) {
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
	path := GetConfigMapFileName(configMapCollector, configMap.Name)

	b, err := json.MarshalIndent(configMap, "", "  ")
	if err != nil {
		return path, nil, err
	}

	return path, b, nil
}

func GetConfigMapFileName(configMapCollector *troubleshootv1beta2.ConfigMap, name string) string {
	parts := []string{"configmaps", configMapCollector.Namespace, name}
	if configMapCollector.Key != "" {
		parts = append(parts, configMapCollector.Key)
	}
	return fmt.Sprintf("%s.json", filepath.Join(parts...))
}

func GetConfigMapErrorsFileName(configMapCollector *troubleshootv1beta2.ConfigMap) string {
	parts := []string{"configmaps-errors", configMapCollector.Namespace}
	if configMapCollector.Name != "" {
		parts = append(parts, configMapCollector.Name)
	} else {
		parts = append(parts, selectorToString(configMapCollector.Selector))
	}
	if configMapCollector.Key != "" {
		parts = append(parts, configMapCollector.Key)
	}
	return fmt.Sprintf("%s.json", filepath.Join(parts...))
}
