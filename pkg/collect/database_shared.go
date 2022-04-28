package collect

type DatabaseConnection struct {
	IsConnected bool              `json:"isConnected"`
	Error       string            `json:"error,omitempty"`
	Version     string            `json:"version,omitempty"`
	Variables   map[string]string `json:"variables,omitempty"`
}
