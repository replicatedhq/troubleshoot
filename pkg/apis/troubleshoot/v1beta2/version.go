package v1beta2

type SupportBundleVersionSpec struct {
	VersionNumber string `json:"versionNumber" yaml:"versionNumber"`
}

type SupportBundleVersion struct {
	ApiVersion string                   `json:"apiVersion" yaml:"apiVersion"`
	Kind       string                   `json:"kind" yaml:"kind"`
	Spec       SupportBundleVersionSpec `json:"spec" yaml:"spec"`
}
