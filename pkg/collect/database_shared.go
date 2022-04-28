package collect

type MySQLVariable struct {
	Key   string
	Value string
}

type DatabaseConnection struct {
	IsConnected bool              `json:"isConnected"`
	Error       string            `json:"error,omitempty"`
	Version     string            `json:"version,omitempty"`
	Variables   map[string]string `json:"variables,omitempty"`
}
