package v1beta1

type SupportBundleVersion struct {
	ApiVersion    string `json:"apiVersion" yaml:"apiVersion"`
	Kind          string `json:"kind" yaml:"kind"`
	VersionNumber string `json:"versionNumber" yaml:"versionNumber"`
}
