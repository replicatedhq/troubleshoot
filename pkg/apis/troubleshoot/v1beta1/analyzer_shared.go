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
	AnalyzeMeta      `json:",inline" yaml:",inline"`
	Outcomes         []*Outcome `json:"outcomes" yaml:"outcomes"`
	StorageClassName string     `json:"storageClassName" yaml:"storageClassName"`
}

type CustomResourceDefinition struct {
	AnalyzeMeta                  `json:",inline" yaml:",inline"`
	Outcomes                     []*Outcome `json:"outcomes" yaml:"outcomes"`
	CustomResourceDefinitionName string     `json:"customResourceDefinitionName" yaml:"customResourceDefinitionName"`
}

type AnalyzeMeta struct {
	CheckName string `json:"checkName,omitempty" yaml:"checkName,omitempty"`
}

type Analyze struct {
	ClusterVersion           *ClusterVersion           `json:"clusterVersion,omitempty" yaml:"clusterVersion,omitempty"`
	StorageClass             *StorageClass             `json:"storageClass,omitempty" yaml:"storageClass,omitempty"`
	CustomResourceDefinition *CustomResourceDefinition `json:"customResourceDefinition,omitempty" yaml:"customResourceDefinition,omitempty"`
}
