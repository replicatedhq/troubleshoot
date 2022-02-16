package v1beta2

type SingleOutcome struct {
	When    string `json:"when,omitempty" yaml:"when,omitempty"`
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
	URI     string `json:"uri,omitempty" yaml:"uri,omitempty"`
}

type Outcome struct {
	Fatal *SingleOutcome `json:"fatal,omitempty" yaml:"fatal,omitempty"`
	Fail  *SingleOutcome `json:"fail,omitempty" yaml:"fail,omitempty"`
	Warn  *SingleOutcome `json:"warn,omitempty" yaml:"warn,omitempty"`
	Pass  *SingleOutcome `json:"pass,omitempty" yaml:"pass,omitempty"`
}
