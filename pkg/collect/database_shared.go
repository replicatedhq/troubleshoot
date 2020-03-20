package collect

type DatabaseConnection struct {
	IsConnected bool   `json:"isConnected"`
	Error       string `json:"error"`
	Version     string `json:"version"`
}
