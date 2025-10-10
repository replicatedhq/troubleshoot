package v1beta3

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestResolveStringOrValueFrom_LiteralValue(t *testing.T) {
	client := fake.NewSimpleClientset()
	value := "literal-value"

	sov := StringOrValueFrom{
		Value: &value,
	}

	result, err := ResolveStringOrValueFrom(context.Background(), sov, client, "default")
	require.NoError(t, err)
	assert.Equal(t, "literal-value", result)
}

func TestResolveStringOrValueFrom_EmptyValue(t *testing.T) {
	client := fake.NewSimpleClientset()

	sov := StringOrValueFrom{}

	result, err := ResolveStringOrValueFrom(context.Background(), sov, client, "default")
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestResolveStringOrValueFrom_SecretKeyRef(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"password": []byte("super-secret-password"),
		},
	}

	client := fake.NewSimpleClientset(secret)

	sov := StringOrValueFrom{
		ValueFrom: &ValueFromSource{
			SecretKeyRef: &SecretKeyRef{
				Name: "test-secret",
				Key:  "password",
			},
		},
	}

	result, err := ResolveStringOrValueFrom(context.Background(), sov, client, "default")
	require.NoError(t, err)
	assert.Equal(t, "super-secret-password", result)
}

func TestResolveStringOrValueFrom_SecretKeyRef_WithNamespace(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "custom-namespace",
		},
		Data: map[string][]byte{
			"password": []byte("secret-from-custom-ns"),
		},
	}

	client := fake.NewSimpleClientset(secret)

	sov := StringOrValueFrom{
		ValueFrom: &ValueFromSource{
			SecretKeyRef: &SecretKeyRef{
				Name:      "test-secret",
				Key:       "password",
				Namespace: "custom-namespace",
			},
		},
	}

	result, err := ResolveStringOrValueFrom(context.Background(), sov, client, "default")
	require.NoError(t, err)
	assert.Equal(t, "secret-from-custom-ns", result)
}

func TestResolveStringOrValueFrom_SecretKeyRef_NotFound(t *testing.T) {
	client := fake.NewSimpleClientset()

	sov := StringOrValueFrom{
		ValueFrom: &ValueFromSource{
			SecretKeyRef: &SecretKeyRef{
				Name: "nonexistent-secret",
				Key:  "password",
			},
		},
	}

	result, err := ResolveStringOrValueFrom(context.Background(), sov, client, "default")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get secret")
	assert.Equal(t, "", result)
}

func TestResolveStringOrValueFrom_SecretKeyRef_KeyNotFound(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"password": []byte("secret-value"),
		},
	}

	client := fake.NewSimpleClientset(secret)

	sov := StringOrValueFrom{
		ValueFrom: &ValueFromSource{
			SecretKeyRef: &SecretKeyRef{
				Name: "test-secret",
				Key:  "nonexistent-key",
			},
		},
	}

	result, err := ResolveStringOrValueFrom(context.Background(), sov, client, "default")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "key \"nonexistent-key\" not found")
	assert.Equal(t, "", result)
}

func TestResolveStringOrValueFrom_SecretKeyRef_Optional(t *testing.T) {
	client := fake.NewSimpleClientset()
	optional := true

	sov := StringOrValueFrom{
		ValueFrom: &ValueFromSource{
			SecretKeyRef: &SecretKeyRef{
				Name:     "nonexistent-secret",
				Key:      "password",
				Optional: &optional,
			},
		},
	}

	result, err := ResolveStringOrValueFrom(context.Background(), sov, client, "default")
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestResolveStringOrValueFrom_SecretKeyRef_OptionalKeyNotFound(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"password": []byte("secret-value"),
		},
	}

	client := fake.NewSimpleClientset(secret)
	optional := true

	sov := StringOrValueFrom{
		ValueFrom: &ValueFromSource{
			SecretKeyRef: &SecretKeyRef{
				Name:     "test-secret",
				Key:      "nonexistent-key",
				Optional: &optional,
			},
		},
	}

	result, err := ResolveStringOrValueFrom(context.Background(), sov, client, "default")
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestResolveStringOrValueFrom_ConfigMapKeyRef(t *testing.T) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmap",
			Namespace: "default",
		},
		Data: map[string]string{
			"config-key": "config-value",
		},
	}

	client := fake.NewSimpleClientset(configMap)

	sov := StringOrValueFrom{
		ValueFrom: &ValueFromSource{
			ConfigMapKeyRef: &ConfigMapKeyRef{
				Name: "test-configmap",
				Key:  "config-key",
			},
		},
	}

	result, err := ResolveStringOrValueFrom(context.Background(), sov, client, "default")
	require.NoError(t, err)
	assert.Equal(t, "config-value", result)
}

func TestResolveStringOrValueFrom_ConfigMapKeyRef_WithNamespace(t *testing.T) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmap",
			Namespace: "custom-namespace",
		},
		Data: map[string]string{
			"config-key": "config-from-custom-ns",
		},
	}

	client := fake.NewSimpleClientset(configMap)

	sov := StringOrValueFrom{
		ValueFrom: &ValueFromSource{
			ConfigMapKeyRef: &ConfigMapKeyRef{
				Name:      "test-configmap",
				Key:       "config-key",
				Namespace: "custom-namespace",
			},
		},
	}

	result, err := ResolveStringOrValueFrom(context.Background(), sov, client, "default")
	require.NoError(t, err)
	assert.Equal(t, "config-from-custom-ns", result)
}

func TestResolveStringOrValueFrom_ConfigMapKeyRef_NotFound(t *testing.T) {
	client := fake.NewSimpleClientset()

	sov := StringOrValueFrom{
		ValueFrom: &ValueFromSource{
			ConfigMapKeyRef: &ConfigMapKeyRef{
				Name: "nonexistent-configmap",
				Key:  "config-key",
			},
		},
	}

	result, err := ResolveStringOrValueFrom(context.Background(), sov, client, "default")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get configmap")
	assert.Equal(t, "", result)
}

func TestResolveStringOrValueFrom_ConfigMapKeyRef_KeyNotFound(t *testing.T) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmap",
			Namespace: "default",
		},
		Data: map[string]string{
			"config-key": "config-value",
		},
	}

	client := fake.NewSimpleClientset(configMap)

	sov := StringOrValueFrom{
		ValueFrom: &ValueFromSource{
			ConfigMapKeyRef: &ConfigMapKeyRef{
				Name: "test-configmap",
				Key:  "nonexistent-key",
			},
		},
	}

	result, err := ResolveStringOrValueFrom(context.Background(), sov, client, "default")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "key \"nonexistent-key\" not found")
	assert.Equal(t, "", result)
}

func TestResolveStringOrValueFrom_ConfigMapKeyRef_Optional(t *testing.T) {
	client := fake.NewSimpleClientset()
	optional := true

	sov := StringOrValueFrom{
		ValueFrom: &ValueFromSource{
			ConfigMapKeyRef: &ConfigMapKeyRef{
				Name:     "nonexistent-configmap",
				Key:      "config-key",
				Optional: &optional,
			},
		},
	}

	result, err := ResolveStringOrValueFrom(context.Background(), sov, client, "default")
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestResolveStringOrValueFrom_ConfigMapKeyRef_OptionalKeyNotFound(t *testing.T) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmap",
			Namespace: "default",
		},
		Data: map[string]string{
			"config-key": "config-value",
		},
	}

	client := fake.NewSimpleClientset(configMap)
	optional := true

	sov := StringOrValueFrom{
		ValueFrom: &ValueFromSource{
			ConfigMapKeyRef: &ConfigMapKeyRef{
				Name:     "test-configmap",
				Key:      "nonexistent-key",
				Optional: &optional,
			},
		},
	}

	result, err := ResolveStringOrValueFrom(context.Background(), sov, client, "default")
	require.NoError(t, err)
	assert.Equal(t, "", result)
}
