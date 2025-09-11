package api

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// InMemoryCredentialStore provides secure in-memory credential storage
type InMemoryCredentialStore struct {
	credentials   map[string]*Credentials
	mu            sync.RWMutex
	encryptionKey []byte
}

// NewInMemoryCredentialStore creates a new in-memory credential store
func NewInMemoryCredentialStore(encryptionKey string) *InMemoryCredentialStore {
	// Create a 32-byte key from the provided string
	hash := sha256.Sum256([]byte(encryptionKey))

	return &InMemoryCredentialStore{
		credentials:   make(map[string]*Credentials),
		encryptionKey: hash[:],
	}
}

// GetCredentials retrieves credentials for the specified auth type
func (s *InMemoryCredentialStore) GetCredentials(authType string) (*Credentials, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	creds, exists := s.credentials[authType]
	if !exists {
		return nil, fmt.Errorf("credentials not found for auth type: %s", authType)
	}

	// Check if credentials are expired
	if !creds.ExpiresAt.IsZero() && time.Now().After(creds.ExpiresAt) {
		return nil, fmt.Errorf("credentials expired for auth type: %s", authType)
	}

	// Return a copy to prevent external modification
	credsCopy := *creds
	return &credsCopy, nil
}

// SetCredentials stores credentials for the specified auth type
func (s *InMemoryCredentialStore) SetCredentials(authType string, creds *Credentials) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Make a copy to prevent external modification
	credsCopy := *creds
	s.credentials[authType] = &credsCopy

	return nil
}

// RefreshCredentials refreshes credentials for the specified auth type
func (s *InMemoryCredentialStore) RefreshCredentials(authType string) (*Credentials, error) {
	// For this implementation, we don't support automatic refresh
	// In a real implementation, this would call the appropriate auth endpoint
	return nil, fmt.Errorf("automatic credential refresh not implemented")
}

// FileCredentialStore provides secure file-based credential storage
type FileCredentialStore struct {
	filePath      string
	encryptionKey []byte
	mu            sync.RWMutex
	cache         map[string]*Credentials
}

// NewFileCredentialStore creates a new file-based credential store
func NewFileCredentialStore(filePath, encryptionKey string) *FileCredentialStore {
	// Create a 32-byte key from the provided string
	hash := sha256.Sum256([]byte(encryptionKey))

	return &FileCredentialStore{
		filePath:      filePath,
		encryptionKey: hash[:],
		cache:         make(map[string]*Credentials),
	}
}

// GetCredentials retrieves credentials from file storage
func (s *FileCredentialStore) GetCredentials(authType string) (*Credentials, error) {
	s.mu.RLock()

	// Check cache first
	if creds, exists := s.cache[authType]; exists {
		if creds.ExpiresAt.IsZero() || time.Now().Before(creds.ExpiresAt) {
			s.mu.RUnlock()
			credsCopy := *creds
			return &credsCopy, nil
		}
		// Remove expired credentials from cache
		delete(s.cache, authType)
	}

	s.mu.RUnlock()

	// Load from file
	if err := s.loadFromFile(); err != nil {
		return nil, fmt.Errorf("failed to load credentials: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	creds, exists := s.cache[authType]
	if !exists {
		return nil, fmt.Errorf("credentials not found for auth type: %s", authType)
	}

	if !creds.ExpiresAt.IsZero() && time.Now().After(creds.ExpiresAt) {
		return nil, fmt.Errorf("credentials expired for auth type: %s", authType)
	}

	credsCopy := *creds
	return &credsCopy, nil
}

// SetCredentials stores credentials to file storage
func (s *FileCredentialStore) SetCredentials(authType string, creds *Credentials) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Load current credentials
	s.loadFromFileUnsafe()

	// Update cache
	credsCopy := *creds
	s.cache[authType] = &credsCopy

	// Save to file
	return s.saveToFileUnsafe()
}

// RefreshCredentials refreshes credentials (not implemented in this basic version)
func (s *FileCredentialStore) RefreshCredentials(authType string) (*Credentials, error) {
	return nil, fmt.Errorf("automatic credential refresh not implemented")
}

// loadFromFile loads credentials from the encrypted file
func (s *FileCredentialStore) loadFromFile() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.loadFromFileUnsafe()
}

// loadFromFileUnsafe loads credentials without locking (assumes caller has lock)
func (s *FileCredentialStore) loadFromFileUnsafe() error {
	// Check if file exists
	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		// File doesn't exist, start with empty cache
		return nil
	}

	// Read encrypted file
	encryptedData, err := os.ReadFile(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to read credentials file: %w", err)
	}

	// Decrypt data
	decryptedData, err := s.decrypt(encryptedData)
	if err != nil {
		return fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	// Parse JSON
	var credentials map[string]*Credentials
	if err := json.Unmarshal(decryptedData, &credentials); err != nil {
		return fmt.Errorf("failed to parse credentials: %w", err)
	}

	s.cache = credentials
	return nil
}

// saveToFileUnsafe saves credentials to file without locking (assumes caller has lock)
func (s *FileCredentialStore) saveToFileUnsafe() error {
	// Serialize credentials
	jsonData, err := json.Marshal(s.cache)
	if err != nil {
		return fmt.Errorf("failed to serialize credentials: %w", err)
	}

	// Encrypt data
	encryptedData, err := s.encrypt(jsonData)
	if err != nil {
		return fmt.Errorf("failed to encrypt credentials: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create credentials directory: %w", err)
	}

	// Write to file with secure permissions
	if err := os.WriteFile(s.filePath, encryptedData, 0600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	return nil
}

// encrypt encrypts data using AES-GCM
func (s *FileCredentialStore) encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// decrypt decrypts data using AES-GCM
func (s *FileCredentialStore) decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// EnvironmentCredentialStore provides credential storage from environment variables
type EnvironmentCredentialStore struct {
	envMappings map[string]EnvMapping
}

// EnvMapping maps auth types to environment variable names
type EnvMapping struct {
	TokenVar    string `json:"tokenVar"`
	APIKeyVar   string `json:"apiKeyVar"`
	UsernameVar string `json:"usernameVar"`
	PasswordVar string `json:"passwordVar"`
	ExpiryVar   string `json:"expiryVar"`
}

// NewEnvironmentCredentialStore creates a new environment-based credential store
func NewEnvironmentCredentialStore() *EnvironmentCredentialStore {
	return &EnvironmentCredentialStore{
		envMappings: map[string]EnvMapping{
			"bearer": {
				TokenVar:  "AUTH_BEARER_TOKEN",
				ExpiryVar: "AUTH_BEARER_EXPIRY",
			},
			"api-key": {
				APIKeyVar: "API_KEY",
			},
			"basic": {
				UsernameVar: "AUTH_USERNAME",
				PasswordVar: "AUTH_PASSWORD",
			},
			"openrouter": {
				APIKeyVar: "OPENROUTER_API_KEY",
			},
			"hosted": {
				TokenVar:  "HOSTED_AUTH_TOKEN",
				ExpiryVar: "HOSTED_AUTH_EXPIRY",
			},
		},
	}
}

// SetEnvMapping sets the environment variable mapping for an auth type
func (s *EnvironmentCredentialStore) SetEnvMapping(authType string, mapping EnvMapping) {
	s.envMappings[authType] = mapping
}

// GetCredentials retrieves credentials from environment variables
func (s *EnvironmentCredentialStore) GetCredentials(authType string) (*Credentials, error) {
	mapping, exists := s.envMappings[authType]
	if !exists {
		return nil, fmt.Errorf("no environment mapping for auth type: %s", authType)
	}

	creds := &Credentials{
		Metadata: make(map[string]string),
	}

	// Get token
	if mapping.TokenVar != "" {
		if token := os.Getenv(mapping.TokenVar); token != "" {
			creds.Token = token
		}
	}

	// Get API key
	if mapping.APIKeyVar != "" {
		if apiKey := os.Getenv(mapping.APIKeyVar); apiKey != "" {
			creds.APIKey = apiKey
		}
	}

	// Get username/password
	if mapping.UsernameVar != "" {
		creds.Username = os.Getenv(mapping.UsernameVar)
	}
	if mapping.PasswordVar != "" {
		creds.Password = os.Getenv(mapping.PasswordVar)
	}

	// Get expiry
	if mapping.ExpiryVar != "" {
		if expiryStr := os.Getenv(mapping.ExpiryVar); expiryStr != "" {
			if expiry, err := time.Parse(time.RFC3339, expiryStr); err == nil {
				creds.ExpiresAt = expiry
			}
		}
	}

	// Check if any credentials were found
	if creds.Token == "" && creds.APIKey == "" && creds.Username == "" {
		return nil, fmt.Errorf("no credentials found in environment for auth type: %s", authType)
	}

	// Check expiry
	if !creds.ExpiresAt.IsZero() && time.Now().After(creds.ExpiresAt) {
		return nil, fmt.Errorf("credentials expired for auth type: %s", authType)
	}

	return creds, nil
}

// SetCredentials is not supported for environment store
func (s *EnvironmentCredentialStore) SetCredentials(authType string, creds *Credentials) error {
	return fmt.Errorf("setting credentials not supported for environment store")
}

// RefreshCredentials is not supported for environment store
func (s *EnvironmentCredentialStore) RefreshCredentials(authType string) (*Credentials, error) {
	return nil, fmt.Errorf("credential refresh not supported for environment store")
}

// ChainCredentialStore allows chaining multiple credential stores with fallback
type ChainCredentialStore struct {
	stores []CredentialStore
}

// NewChainCredentialStore creates a new chain credential store
func NewChainCredentialStore(stores ...CredentialStore) *ChainCredentialStore {
	return &ChainCredentialStore{
		stores: stores,
	}
}

// GetCredentials attempts to get credentials from each store in order
func (s *ChainCredentialStore) GetCredentials(authType string) (*Credentials, error) {
	var lastErr error

	for i, store := range s.stores {
		creds, err := store.GetCredentials(authType)
		if err == nil {
			return creds, nil
		}
		lastErr = err

		// Log the attempt (in real implementation, would use proper logging)
		fmt.Printf("Store %d failed to get credentials for %s: %v\n", i+1, authType, err)
	}

	return nil, fmt.Errorf("all credential stores failed, last error: %w", lastErr)
}

// SetCredentials sets credentials in the first store that supports it
func (s *ChainCredentialStore) SetCredentials(authType string, creds *Credentials) error {
	var lastErr error

	for _, store := range s.stores {
		err := store.SetCredentials(authType, creds)
		if err == nil {
			return nil
		}
		lastErr = err
	}

	return fmt.Errorf("no credential store could set credentials: %w", lastErr)
}

// RefreshCredentials attempts to refresh credentials using any available store
func (s *ChainCredentialStore) RefreshCredentials(authType string) (*Credentials, error) {
	var lastErr error

	for _, store := range s.stores {
		creds, err := store.RefreshCredentials(authType)
		if err == nil {
			return creds, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("no credential store could refresh credentials: %w", lastErr)
}

// CredentialValidator provides validation for credentials
type CredentialValidator struct{}

// ValidateCredentials validates that credentials are properly formatted and not expired
func (cv *CredentialValidator) ValidateCredentials(authType string, creds *Credentials) error {
	if creds == nil {
		return fmt.Errorf("credentials cannot be nil")
	}

	// Check expiry
	if !creds.ExpiresAt.IsZero() && time.Now().After(creds.ExpiresAt) {
		return fmt.Errorf("credentials are expired")
	}

	// Validate based on auth type
	switch authType {
	case "bearer":
		if creds.Token == "" {
			return fmt.Errorf("bearer token is required")
		}
		if len(creds.Token) < 10 {
			return fmt.Errorf("bearer token appears to be too short")
		}

	case "api-key":
		if creds.APIKey == "" {
			return fmt.Errorf("API key is required")
		}
		if len(creds.APIKey) < 10 {
			return fmt.Errorf("API key appears to be too short")
		}

	case "basic":
		if creds.Username == "" || creds.Password == "" {
			return fmt.Errorf("username and password are required for basic auth")
		}

	default:
		// For unknown auth types, just check that at least one field is set
		if creds.Token == "" && creds.APIKey == "" && creds.Username == "" {
			return fmt.Errorf("no credential fields are set")
		}
	}

	return nil
}

// SecureCredentialMasker provides utilities to safely log credentials
type SecureCredentialMasker struct{}

// MaskCredentials returns a copy of credentials with sensitive fields masked
func (scm *SecureCredentialMasker) MaskCredentials(creds *Credentials) *Credentials {
	if creds == nil {
		return nil
	}

	masked := &Credentials{
		ExpiresAt: creds.ExpiresAt,
		Metadata:  make(map[string]string),
	}

	// Copy non-sensitive metadata
	for key, value := range creds.Metadata {
		masked.Metadata[key] = value
	}

	// Mask sensitive fields
	if creds.Token != "" {
		masked.Token = scm.maskString(creds.Token)
	}
	if creds.APIKey != "" {
		masked.APIKey = scm.maskString(creds.APIKey)
	}
	if creds.Username != "" {
		masked.Username = creds.Username // Username is typically not sensitive
	}
	if creds.Password != "" {
		masked.Password = "[MASKED]"
	}

	return masked
}

// maskString masks most of a string, showing only first and last few characters
func (scm *SecureCredentialMasker) maskString(s string) string {
	if len(s) <= 6 {
		return "[MASKED]"
	}

	if len(s) <= 10 {
		return s[:2] + "***" + s[len(s)-1:]
	}

	return s[:3] + "***" + s[len(s)-3:]
}
