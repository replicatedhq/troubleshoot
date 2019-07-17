package v1beta1

type SingleOutcome struct {
	When    string `json:"when,omitempty" yaml:"when,omitempty"`
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
	URI     string `json:"uri,omitempty" yaml:"uri,omitempty"`
}

type Outcome struct {
	Fail *SingleOutcome `json:"fail,omitempty" yaml:"fail,omitempty"`
	Warn *SingleOutcome `json:"warn,omitempty" yaml:"warn,omitempty"`
	Pass *SingleOutcome `json:"pass,omitempty" yaml:"pass,omitempty"`
}

type ClusterVersion struct {
	AnalyzeMeta `json:",inline" yaml:",inline"`
	Outcomes    []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type StorageClass struct {
	AnalyzeMeta `json:",inline" yaml:",inline"`
	Outcome     []*Outcome `json:"outcomes" yaml:"outcomes"`
	Name        string     `json:"name" yaml:"name"`
}

type AnalyzeMeta struct {
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}

type Analyze struct {
	ClusterVersion *ClusterVersion `json:"clusterVersion,omitempty" yaml:"clusterVersion,omitempty"`
	StorageClass   *StorageClass   `json:"storageClass,omitempty" yaml:"supportBundle,omitempty"`
}
