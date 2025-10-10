package v1beta3

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ResolveStringOrValueFrom resolves a StringOrValueFrom to its actual string value
// by fetching from Secrets or ConfigMaps as needed.
//
// Parameters:
//   - ctx: Context for the resolution operation
//   - sov: The StringOrValueFrom to resolve
//   - client: Kubernetes client for fetching Secrets/ConfigMaps
//   - defaultNamespace: Namespace to use when not specified in the reference
//
// Returns:
//   - The resolved string value
//   - An error if resolution fails (unless Optional is true)
func ResolveStringOrValueFrom(
	ctx context.Context,
	sov StringOrValueFrom,
	client kubernetes.Interface,
	defaultNamespace string,
) (string, error) {
	// If Value is directly specified, use it
	if sov.Value != nil {
		return *sov.Value, nil
	}

	// If ValueFrom is not specified, return empty string
	if sov.ValueFrom == nil {
		return "", nil
	}

	// Resolve from SecretKeyRef
	if sov.ValueFrom.SecretKeyRef != nil {
		return resolveSecretKeyRef(ctx, sov.ValueFrom.SecretKeyRef, client, defaultNamespace)
	}

	// Resolve from ConfigMapKeyRef
	if sov.ValueFrom.ConfigMapKeyRef != nil {
		return resolveConfigMapKeyRef(ctx, sov.ValueFrom.ConfigMapKeyRef, client, defaultNamespace)
	}

	return "", nil
}

// resolveSecretKeyRef fetches a value from a Kubernetes Secret
func resolveSecretKeyRef(
	ctx context.Context,
	ref *SecretKeyRef,
	client kubernetes.Interface,
	defaultNamespace string,
) (string, error) {
	namespace := ref.Namespace
	if namespace == "" {
		namespace = defaultNamespace
	}

	secret, err := client.CoreV1().Secrets(namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		if isOptional(ref.Optional) {
			return "", nil
		}
		return "", fmt.Errorf("failed to get secret %s/%s: %w", namespace, ref.Name, err)
	}

	value, ok := secret.Data[ref.Key]
	if !ok {
		if isOptional(ref.Optional) {
			return "", nil
		}
		return "", fmt.Errorf("key %q not found in secret %s/%s", ref.Key, namespace, ref.Name)
	}

	return string(value), nil
}

// resolveConfigMapKeyRef fetches a value from a Kubernetes ConfigMap
func resolveConfigMapKeyRef(
	ctx context.Context,
	ref *ConfigMapKeyRef,
	client kubernetes.Interface,
	defaultNamespace string,
) (string, error) {
	namespace := ref.Namespace
	if namespace == "" {
		namespace = defaultNamespace
	}

	configMap, err := client.CoreV1().ConfigMaps(namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		if isOptional(ref.Optional) {
			return "", nil
		}
		return "", fmt.Errorf("failed to get configmap %s/%s: %w", namespace, ref.Name, err)
	}

	value, ok := configMap.Data[ref.Key]
	if !ok {
		if isOptional(ref.Optional) {
			return "", nil
		}
		return "", fmt.Errorf("key %q not found in configmap %s/%s", ref.Key, namespace, ref.Name)
	}

	return value, nil
}

// isOptional checks if the optional flag is set to true
func isOptional(optional *bool) bool {
	return optional != nil && *optional
}
