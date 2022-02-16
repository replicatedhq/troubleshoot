package preflight

type UploadPreflightResult struct {
	IsFatal   bool `json:"isFatal,omitempty"`
	IsFail    bool `json:"isFail,omitempty"`
	IsWarn    bool `json:"isWarn,omitempty"`
	IsPass    bool `json:"isPass,omitempty"`

	Title   string `json:"title"`
	Message string `json:"message"`
	URI     string `json:"uri,omitempty"`
}

type UploadPreflightError struct {
	Error string `json:"error"`
}

type UploadPreflightResults struct {
	Results []*UploadPreflightResult `json:"results,omitempty"`
	Errors  []*UploadPreflightError  `json:"errors,omitempty"`
}
