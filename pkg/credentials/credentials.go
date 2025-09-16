package credentials

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrCredentialsNotFound indicates no token could be discovered from any source.
var ErrCredentialsNotFound = errors.New("credentials not found")

// Credentials holds the API token used for authenticated vendor API requests.
//
// The token origin priority is:
// 1) TROUBLESHOOT_TOKEN environment variable
// 2) Token stored on disk via support-bundle login
// If neither are available, callers should prompt the user to log in.
type Credentials struct {
	APIToken string `json:"token"`
}

// GetCurrentCredentials retrieves the current credentials following the priority
// order: environment variable first, then the persisted config file.
func GetCurrentCredentials() (*Credentials, error) {
	if env := strings.TrimSpace(os.Getenv("TROUBLESHOOT_TOKEN")); env != "" {
		return &Credentials{APIToken: env}, nil
	}

	token, err := readTokenFromDisk()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrCredentialsNotFound
		}
		return nil, err
	}
	if strings.TrimSpace(token) == "" {
		return nil, ErrCredentialsNotFound
	}
	return &Credentials{APIToken: token}, nil
}

// SetCurrentCredentials persists the provided token to disk with 0600 permissions.
// The file format is JSON to avoid an external YAML dependency. If a legacy
// ~/.troubleshoot/config.yaml file exists, it will be overwritten with JSON content
// for simplicity.
func SetCurrentCredentials(token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("empty token provided")
	}
	path, err := configFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { // ~/.troubleshoot
		return err
	}
	data, err := json.MarshalIndent(Credentials{APIToken: token}, "", "  ")
	if err != nil {
		return err
	}
	// Write atomically: write to temp then rename
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// RemoveCurrentCredentials deletes the persisted credential file, if present.
func RemoveCurrentCredentials() error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// readTokenFromDisk returns the token from the persisted file, if present.
func readTokenFromDisk() (string, error) {
	path, err := configFilePath()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	// JSON format: {"token":"..."}
	var c Credentials
	if err := json.Unmarshal(b, &c); err == nil {
		return c.APIToken, nil
	}
	// Fallback: extremely simple YAML (token: value)
	// We avoid adding yaml.v3; parse a single key manually if present.
	content := string(b)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "token:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "token:")), nil
		}
	}
	return "", nil
}

// configFilePath returns the full path to the credentials file.
// We intentionally use a YAML-named path for compatibility, even though the
// content we write is JSON for simplicity.
func configFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".troubleshoot", "config.yaml"), nil
}
