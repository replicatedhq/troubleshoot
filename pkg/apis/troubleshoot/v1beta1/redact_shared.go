package v1beta1

type Redact struct {
	Name   string   `json:"name,omitempty" yaml:"name,omitempty"`
	File   string   `json:"file,omitempty" yaml:"file,omitempty"`
	Files  []string `json:"files,omitempty" yaml:"files,omitempty"`
	Values []string `json:"values,omitempty" yaml:"values,omitempty"`
	Regex  []string `json:"regex,omitempty" yaml:"regex,omitempty"`
}
