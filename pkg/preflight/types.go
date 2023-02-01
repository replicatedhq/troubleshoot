package preflight

type UploadPreflightResult struct {
	Strict bool `json:"strict,omitempty"`
	IsFail bool `json:"isFail,omitempty"`
	IsWarn bool `json:"isWarn,omitempty"`
	IsPass bool `json:"isPass,omitempty"`

	Title   string `json:"title"`
	Message string `json:"message"`
	URI     string `json:"uri,omitempty"`
	Note    string `json:"note,omitempty"`
}

type UploadPreflightError struct {
	Error string `json:"error"`
}

type UploadPreflightResults struct {
	Results []*UploadPreflightResult `json:"results,omitempty"`
	Errors  []*UploadPreflightError  `json:"errors,omitempty"`
}
