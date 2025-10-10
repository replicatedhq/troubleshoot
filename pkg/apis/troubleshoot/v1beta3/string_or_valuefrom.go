package v1beta3

// StringOrValueFrom represents a string value that can either be specified
// directly or sourced from a Kubernetes Secret or ConfigMap
type StringOrValueFrom struct {
	// Value is a literal string value
	// +optional
	Value *string `json:"value,omitempty" yaml:"value,omitempty"`

	// ValueFrom is a reference to a value in a Secret or ConfigMap
	// +optional
	ValueFrom *ValueFromSource `json:"valueFrom,omitempty" yaml:"valueFrom,omitempty"`
}

// ValueFromSource represents the source of a value from a Secret or ConfigMap
type ValueFromSource struct {
	// SecretKeyRef references a key in a Secret
	// +optional
	SecretKeyRef *SecretKeyRef `json:"secretKeyRef,omitempty" yaml:"secretKeyRef,omitempty"`

	// ConfigMapKeyRef references a key in a ConfigMap
	// +optional
	ConfigMapKeyRef *ConfigMapKeyRef `json:"configMapKeyRef,omitempty" yaml:"configMapKeyRef,omitempty"`
}

// SecretKeyRef references a specific key in a Kubernetes Secret
type SecretKeyRef struct {
	// Name is the name of the Secret
	Name string `json:"name" yaml:"name"`

	// Key is the key within the Secret to read
	Key string `json:"key" yaml:"key"`

	// Namespace is the namespace of the Secret
	// If not specified, defaults to the namespace where the SupportBundle is running
	// +optional
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

	// Optional specifies whether the Secret must exist
	// If true and the Secret or key doesn't exist, resolves to empty string
	// If false (default) and the Secret or key doesn't exist, resolution fails
	// +optional
	Optional *bool `json:"optional,omitempty" yaml:"optional,omitempty"`
}

// ConfigMapKeyRef references a specific key in a Kubernetes ConfigMap
type ConfigMapKeyRef struct {
	// Name is the name of the ConfigMap
	Name string `json:"name" yaml:"name"`

	// Key is the key within the ConfigMap to read
	Key string `json:"key" yaml:"key"`

	// Namespace is the namespace of the ConfigMap
	// If not specified, defaults to the namespace where the SupportBundle is running
	// +optional
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

	// Optional specifies whether the ConfigMap must exist
	// If true and the ConfigMap or key doesn't exist, resolves to empty string
	// If false (default) and the ConfigMap or key doesn't exist, resolution fails
	// +optional
	Optional *bool `json:"optional,omitempty" yaml:"optional,omitempty"`
}
