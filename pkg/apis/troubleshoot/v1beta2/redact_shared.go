package v1beta2

type Regex struct {
	Selector string `json:"selector,omitempty" yaml:"selector,omitempty"`
	Redactor string `json:"redactor,omitempty" yaml:"redactor,omitempty"`
}

type FileSelector struct {
	File  string   `json:"file,omitempty" yaml:"file,omitempty"`
	Files []string `json:"files,omitempty" yaml:"files,omitempty"`
}

type Removals struct {
	Values   []string `json:"values,omitempty" yaml:"values,omitempty"`
	Regex    []Regex  `json:"regex,omitempty" yaml:"regex,omitempty"`
	YamlPath []string `json:"yamlPath,omitempty" yaml:"yamlPath,omitempty"`
}

type Redact struct {
	Name         string       `json:"name,omitempty" yaml:"name,omitempty"`
	FileSelector FileSelector `json:"fileSelector,omitempty" yaml:"fileSelector,omitempty"`
	Removals     Removals     `json:"removals,omitempty" yaml:"removals,omitempty"`
}
