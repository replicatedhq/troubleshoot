package redact

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// TokenPrefix represents different types of secrets for token generation
type TokenPrefix string

const (
	TokenPrefixPassword   TokenPrefix = "PASSWORD"
	TokenPrefixAPIKey     TokenPrefix = "APIKEY"
	TokenPrefixDatabase   TokenPrefix = "DATABASE"
	TokenPrefixEmail      TokenPrefix = "EMAIL"
	TokenPrefixIP         TokenPrefix = "IP"
	TokenPrefixToken      TokenPrefix = "TOKEN"
	TokenPrefixSecret     TokenPrefix = "SECRET"
	TokenPrefixKey        TokenPrefix = "KEY"
	TokenPrefixCredential TokenPrefix = "CREDENTIAL"
	TokenPrefixAuth       TokenPrefix = "AUTH"
	TokenPrefixGeneric    TokenPrefix = "GENERIC"
)

// TokenizerConfig holds configuration for the tokenizer
type TokenizerConfig struct {
	// Enable tokenization (defaults to checking TROUBLESHOOT_TOKENIZATION env var)
	Enabled bool

	// Salt for deterministic token generation per bundle
	Salt []byte

	// Default token prefix when type cannot be determined
	DefaultPrefix TokenPrefix

	// Token format template (must include %s for prefix and %s for hash)
	TokenFormat string

	// Hash length in characters (default 6)
	HashLength int
}

// Tokenizer handles deterministic secret tokenization
type Tokenizer struct {
	config     TokenizerConfig
	tokenMap   map[string]string // secret value -> token
	reverseMap map[string]string // token -> secret value (for debugging/mapping)
	mutex      sync.RWMutex

	// Secret type detection patterns
	typePatterns map[TokenPrefix]*regexp.Regexp

	// Phase 2: Cross-File Correlation fields
	bundleID          string                     // unique bundle identifier
	secretRefs        map[string][]string        // token -> list of file paths
	duplicateGroups   map[string]*DuplicateGroup // secretHash -> DuplicateGroup
	correlations      []CorrelationGroup         // detected correlations
	fileStats         map[string]*FileStats      // filePath -> FileStats
	cacheStats        CacheStats                 // performance statistics
	normalizedSecrets map[string]string          // normalized secret -> original secret
	secretHashes      map[string]string          // secret value -> hash for deduplication
}

// RedactionMap represents the mapping between tokens and original values
type RedactionMap struct {
	Tokens        map[string]string   `json:"tokens"`       // token -> original value
	Stats         RedactionStats      `json:"stats"`        // redaction statistics
	Timestamp     time.Time           `json:"timestamp"`    // when redaction was performed
	Profile       string              `json:"profile"`      // profile used
	BundleID      string              `json:"bundleId"`     // unique bundle identifier
	SecretRefs    map[string][]string `json:"secretRefs"`   // token -> list of file paths where found
	Duplicates    []DuplicateGroup    `json:"duplicates"`   // groups of identical secrets
	Correlations  []CorrelationGroup  `json:"correlations"` // correlated secret patterns
	EncryptionKey []byte              `json:"-"`            // encryption key (not serialized)
	IsEncrypted   bool                `json:"isEncrypted"`  // whether the mapping is encrypted
}

// RedactionStats contains statistics about the redaction process
type RedactionStats struct {
	TotalSecrets      int                  `json:"totalSecrets"`
	UniqueSecrets     int                  `json:"uniqueSecrets"`
	TokensGenerated   int                  `json:"tokensGenerated"`
	SecretsByType     map[string]int       `json:"secretsByType"`
	ProcessingTimeMs  int64                `json:"processingTimeMs"`
	FilesCovered      int                  `json:"filesCovered"`
	DuplicateCount    int                  `json:"duplicateCount"`
	CorrelationCount  int                  `json:"correlationCount"`
	NormalizationHits int                  `json:"normalizationHits"`
	CacheHits         int                  `json:"cacheHits"`
	CacheMisses       int                  `json:"cacheMisses"`
	FileCoverage      map[string]FileStats `json:"fileCoverage"`
}

// FileStats tracks statistics per file
type FileStats struct {
	FilePath     string         `json:"filePath"`
	SecretsFound int            `json:"secretsFound"`
	TokensUsed   int            `json:"tokensUsed"`
	SecretTypes  map[string]int `json:"secretTypes"`
	ProcessedAt  time.Time      `json:"processedAt"`
}

// DuplicateGroup represents a group of identical secrets found in different locations
type DuplicateGroup struct {
	SecretHash string    `json:"secretHash"` // hash of the normalized secret
	Token      string    `json:"token"`      // the token used for this secret
	SecretType string    `json:"secretType"` // classified type of the secret
	Locations  []string  `json:"locations"`  // file paths where this secret was found
	Count      int       `json:"count"`      // total occurrences
	FirstSeen  time.Time `json:"firstSeen"`  // when first detected
	LastSeen   time.Time `json:"lastSeen"`   // when last detected
}

// CorrelationGroup represents correlated secret patterns across files
type CorrelationGroup struct {
	Pattern     string    `json:"pattern"`     // correlation pattern identifier
	Description string    `json:"description"` // human-readable description
	Tokens      []string  `json:"tokens"`      // tokens involved in correlation
	Files       []string  `json:"files"`       // files where correlation was found
	Confidence  float64   `json:"confidence"`  // confidence score (0.0-1.0)
	DetectedAt  time.Time `json:"detectedAt"`  // when correlation was detected
}

// CacheStats tracks tokenizer cache performance
type CacheStats struct {
	Hits   int64 `json:"hits"`   // cache hits
	Misses int64 `json:"misses"` // cache misses
	Total  int64 `json:"total"`  // total lookups
}

var (
	// Global tokenizer instance
	globalTokenizer *Tokenizer
	tokenizerOnce   sync.Once
)

// NewTokenizer creates a new tokenizer with the given configuration
func NewTokenizer(config TokenizerConfig) *Tokenizer {
	if config.TokenFormat == "" {
		config.TokenFormat = "***TOKEN_%s_%s***"
	}
	if config.HashLength == 0 {
		config.HashLength = 6
	}
	if config.DefaultPrefix == "" {
		config.DefaultPrefix = TokenPrefixGeneric
	}

	// Generate salt if not provided
	if len(config.Salt) == 0 {
		config.Salt = make([]byte, 32)
		if _, err := rand.Read(config.Salt); err != nil {
			// Fallback to time-based salt if crypto rand fails
			timeStr := fmt.Sprintf("%d", time.Now().UnixNano())
			config.Salt = []byte(timeStr)
		}
	}

	// Generate bundle ID if not provided
	bundleID := fmt.Sprintf("bundle_%d_%s", time.Now().UnixNano(), hex.EncodeToString(config.Salt[:8]))

	tokenizer := &Tokenizer{
		config:            config,
		tokenMap:          make(map[string]string),
		reverseMap:        make(map[string]string),
		typePatterns:      make(map[TokenPrefix]*regexp.Regexp),
		bundleID:          bundleID,
		secretRefs:        make(map[string][]string),
		duplicateGroups:   make(map[string]*DuplicateGroup),
		correlations:      make([]CorrelationGroup, 0),
		fileStats:         make(map[string]*FileStats),
		cacheStats:        CacheStats{},
		normalizedSecrets: make(map[string]string),
		secretHashes:      make(map[string]string),
	}

	// Initialize secret type detection patterns
	tokenizer.initTypePatterns()

	return tokenizer
}

// GetGlobalTokenizer returns the global tokenizer instance
func GetGlobalTokenizer() *Tokenizer {
	tokenizerOnce.Do(func() {
		enabled := os.Getenv("TROUBLESHOOT_TOKENIZATION") == "true" ||
			os.Getenv("TROUBLESHOOT_TOKENIZATION") == "1" ||
			os.Getenv("TROUBLESHOOT_TOKENIZATION") == "enabled"

		globalTokenizer = NewTokenizer(TokenizerConfig{
			Enabled: enabled,
		})
	})
	return globalTokenizer
}

// IsEnabled returns whether tokenization is enabled
func (t *Tokenizer) IsEnabled() bool {
	return t.config.Enabled
}

// initTypePatterns initializes regex patterns for secret type detection
func (t *Tokenizer) initTypePatterns() {
	patterns := map[TokenPrefix]string{
		TokenPrefixPassword:   `(?i)password|passwd|pwd`,
		TokenPrefixAPIKey:     `(?i)api.?key|apikey|access.?key`,
		TokenPrefixDatabase:   `(?i)database|db.?(url|uri|host|pass|connection)`,
		TokenPrefixEmail:      `(?i)[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`,
		TokenPrefixIP:         `(?i)\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`,
		TokenPrefixToken:      `(?i)token|bearer|jwt|oauth`,
		TokenPrefixSecret:     `(?i)secret|private.?key`,
		TokenPrefixCredential: `(?i)credential|cred|auth`,
		TokenPrefixKey:        `(?i)key|cert|certificate`,
	}

	for prefix, pattern := range patterns {
		if compiled, err := regexp.Compile(pattern); err == nil {
			t.typePatterns[prefix] = compiled
		}
	}
}

// classifySecret determines the appropriate token prefix for a secret value
func (t *Tokenizer) classifySecret(context, value string) TokenPrefix {
	contextLower := strings.ToLower(context)
	valueLower := strings.ToLower(value)

	// Check context first, with specific patterns having priority
	// Order matters here - more specific patterns should be checked first
	specificPrefixes := []TokenPrefix{
		TokenPrefixAPIKey,
		TokenPrefixPassword,
		TokenPrefixDatabase,
		TokenPrefixCredential,
		TokenPrefixSecret,
		TokenPrefixToken,
		TokenPrefixKey, // More general, check last
	}

	for _, prefix := range specificPrefixes {
		if pattern, exists := t.typePatterns[prefix]; exists {
			if pattern.MatchString(contextLower) {
				return prefix
			}
		}
	}

	// Check value patterns for specific formats (email, IP, etc.)
	if pattern, exists := t.typePatterns[TokenPrefixEmail]; exists && pattern.MatchString(value) {
		return TokenPrefixEmail
	}
	if pattern, exists := t.typePatterns[TokenPrefixIP]; exists && pattern.MatchString(value) {
		return TokenPrefixIP
	}

	// Check value content for common secret indicators (same priority order)
	for _, prefix := range specificPrefixes {
		prefixLower := strings.ToLower(string(prefix))
		if strings.Contains(valueLower, prefixLower) {
			return prefix
		}
	}

	return t.config.DefaultPrefix
}

// generateToken creates a deterministic token for a given secret value
func (t *Tokenizer) generateToken(value, context string) string {
	// Classify the secret type
	prefix := t.classifySecret(context, value)

	// Generate deterministic hash using HMAC-SHA256
	h := hmac.New(sha256.New, t.config.Salt)
	h.Write([]byte(value))
	h.Write([]byte(context)) // Include context for better uniqueness
	hash := h.Sum(nil)

	// Convert to hex and truncate to desired length
	hashStr := hex.EncodeToString(hash)
	if len(hashStr) > t.config.HashLength {
		hashStr = hashStr[:t.config.HashLength]
	}

	// Generate token with collision detection
	baseToken := fmt.Sprintf(t.config.TokenFormat, string(prefix), strings.ToUpper(hashStr))

	// Check for collisions and resolve them
	token := t.resolveCollision(baseToken, value)

	return token
}

// resolveCollision handles token collisions by appending a counter
func (t *Tokenizer) resolveCollision(baseToken, value string) string {
	// Check for collision without lock first
	existingValue, exists := t.reverseMap[baseToken]

	// No collision
	if !exists || existingValue == value {
		return baseToken
	}

	// Collision detected, try up to 100 variations
	for counter := 1; counter <= 100; counter++ {
		newToken := fmt.Sprintf("%s_%d", baseToken, counter)

		existingValue, exists = t.reverseMap[newToken]
		if !exists || existingValue == value {
			return newToken
		}
	}

	// If we still have collisions after 100 tries, use timestamp
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s_%d", baseToken, timestamp%10000)
}

// TokenizeValue generates or retrieves a token for a secret value
func (t *Tokenizer) TokenizeValue(value, context string) string {
	return t.TokenizeValueWithPath(value, context, "")
}

// TokenizeValueWithPath generates or retrieves a token for a secret value with file path tracking
func (t *Tokenizer) TokenizeValueWithPath(value, context, filePath string) string {
	if !t.config.Enabled || value == "" {
		return MASK_TEXT // Fallback to original behavior
	}

	t.mutex.Lock()
	defer t.mutex.Unlock()

	// Normalize the secret value for better correlation
	normalizedValue := t.normalizeSecret(value)

	// Update cache statistics
	t.cacheStats.Total++

	// Check if we already have a token for this normalized value
	if existing, exists := t.tokenMap[normalizedValue]; exists {
		t.cacheStats.Hits++

		// Track this usage even if token already exists
		if filePath != "" {
			t.addSecretReference(existing, filePath)

			// Get secret type for tracking
			secretType := string(t.classifySecret(context, value))
			t.updateFileStats(filePath, secretType)

			// Update duplicate tracking
			secretHash := t.generateSecretHash(normalizedValue)
			t.trackDuplicateSecret(secretHash, existing, secretType, filePath, normalizedValue)
		}

		return existing
	}

	t.cacheStats.Misses++

	// Generate new token
	token := t.generateToken(normalizedValue, context)

	// Store in both directions (use normalized value as key)
	t.tokenMap[normalizedValue] = token
	t.reverseMap[token] = value // Store original value for mapping

	// Track secret hash for deduplication
	secretHash := t.generateSecretHash(normalizedValue)
	t.secretHashes[normalizedValue] = secretHash

	// Track file reference and stats if path provided
	if filePath != "" {
		t.addSecretReference(token, filePath)

		// Get secret type for tracking
		secretType := string(t.classifySecret(context, value))
		t.updateFileStats(filePath, secretType)

		// Track as duplicate (even first occurrence)
		t.trackDuplicateSecret(secretHash, token, secretType, filePath, normalizedValue)
	}

	return token
}

// GetRedactionMap returns the current redaction map
func (t *Tokenizer) GetRedactionMap(profile string) RedactionMap {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	// Analyze correlations before generating the map
	t.analyzeCorrelations()

	// Create stats
	secretsByType := make(map[string]int)
	for token := range t.reverseMap {
		// Extract type from token format
		if parts := strings.Split(token, "_"); len(parts) >= 2 {
			// Expected format: ***TOKEN_TYPE_HASH***
			if len(parts) >= 3 && strings.HasPrefix(token, "***TOKEN_") {
				tokenType := parts[2] // Extract TYPE part
				secretsByType[tokenType]++
			}
		}
	}

	// Count duplicates and correlations
	duplicateCount := 0
	for _, group := range t.duplicateGroups {
		if group.Count > 1 {
			duplicateCount++
		}
	}

	// Copy file coverage
	fileCoverage := make(map[string]FileStats)
	for path, stats := range t.fileStats {
		if stats != nil {
			fileCoverage[path] = *stats
		}
	}

	// Convert duplicate groups to slice
	duplicates := make([]DuplicateGroup, 0, len(t.duplicateGroups))
	for _, group := range t.duplicateGroups {
		if group != nil {
			duplicates = append(duplicates, *group)
		}
	}

	stats := RedactionStats{
		TotalSecrets:      len(t.tokenMap),
		UniqueSecrets:     len(t.tokenMap),
		TokensGenerated:   len(t.reverseMap),
		SecretsByType:     secretsByType,
		ProcessingTimeMs:  0, // Would be populated by caller
		FilesCovered:      len(t.fileStats),
		DuplicateCount:    duplicateCount,
		CorrelationCount:  len(t.correlations),
		NormalizationHits: len(t.normalizedSecrets),
		CacheHits:         int(t.cacheStats.Hits),
		CacheMisses:       int(t.cacheStats.Misses),
		FileCoverage:      fileCoverage,
	}

	return RedactionMap{
		Tokens:       t.reverseMap,
		Stats:        stats,
		Timestamp:    time.Now(),
		Profile:      profile,
		BundleID:     t.bundleID,
		SecretRefs:   t.secretRefs,
		Duplicates:   duplicates,
		Correlations: t.correlations,
		IsEncrypted:  false, // Will be set when encryption is applied
	}
}

// ValidateToken checks if a token matches the expected format
func (t *Tokenizer) ValidateToken(token string) bool {
	// Basic format validation - should match ***TOKEN_PREFIX_HASH***
	pattern := `^\*\*\*TOKEN_[A-Z]+_[A-F0-9]+(\*\*\*|_\d+\*\*\*)$`
	matched, err := regexp.MatchString(pattern, token)
	return err == nil && matched
}

// Reset clears all tokens and mappings (useful for testing)
func (t *Tokenizer) Reset() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.tokenMap = make(map[string]string)
	t.reverseMap = make(map[string]string)
	t.secretRefs = make(map[string][]string)
	t.duplicateGroups = make(map[string]*DuplicateGroup)
	t.correlations = make([]CorrelationGroup, 0)
	t.fileStats = make(map[string]*FileStats)
	t.cacheStats = CacheStats{}
	t.normalizedSecrets = make(map[string]string)
	t.secretHashes = make(map[string]string)
}

// GetTokenCount returns the number of tokens generated
func (t *Tokenizer) GetTokenCount() int {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	return len(t.tokenMap)
}

// ResetGlobalTokenizer resets the global tokenizer instance (useful for testing)
func ResetGlobalTokenizer() {
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}
}

// analyzeCorrelations detects patterns and correlations across secrets
func (t *Tokenizer) analyzeCorrelations() {
	// Detect common correlation patterns
	correlations := make([]CorrelationGroup, 0)

	// Pattern 1: Database connection components (host, user, password, database)
	dbTokens := make([]string, 0)
	dbFiles := make([]string, 0)

	for token, files := range t.secretRefs {
		// Check if token looks like database-related
		if strings.Contains(token, "DATABASE") || strings.Contains(token, "PASSWORD") {
			dbTokens = append(dbTokens, token)
			for _, file := range files {
				// Add file if not already present
				found := false
				for _, existing := range dbFiles {
					if existing == file {
						found = true
						break
					}
				}
				if !found {
					dbFiles = append(dbFiles, file)
				}
			}
		}
	}

	if len(dbTokens) >= 2 && len(dbFiles) >= 1 {
		correlations = append(correlations, CorrelationGroup{
			Pattern:     "database_credentials",
			Description: "Database connection credentials found together",
			Tokens:      dbTokens,
			Files:       dbFiles,
			Confidence:  0.8,
			DetectedAt:  time.Now(),
		})
	}

	// Pattern 2: AWS credential pairs (Access Key + Secret)
	awsTokens := make([]string, 0)
	awsFiles := make([]string, 0)

	for token, files := range t.secretRefs {
		// Look for any APIKEY or SECRET tokens - AWS detection can be broader
		if strings.Contains(token, "APIKEY") || strings.Contains(token, "SECRET") {
			awsTokens = append(awsTokens, token)
			for _, file := range files {
				found := false
				for _, existing := range awsFiles {
					if existing == file {
						found = true
						break
					}
				}
				if !found {
					awsFiles = append(awsFiles, file)
				}
			}
		}
	}

	if len(awsTokens) >= 2 && len(awsFiles) >= 1 {
		correlations = append(correlations, CorrelationGroup{
			Pattern:     "aws_credentials",
			Description: "AWS credential pair (access key + secret) found together",
			Tokens:      awsTokens,
			Files:       awsFiles,
			Confidence:  0.9,
			DetectedAt:  time.Now(),
		})
	}

	// Pattern 3: API authentication (API key + token)
	apiTokens := make([]string, 0)
	apiFiles := make([]string, 0)

	for token, files := range t.secretRefs {
		if strings.Contains(token, "APIKEY") || strings.Contains(token, "TOKEN") {
			apiTokens = append(apiTokens, token)
			for _, file := range files {
				found := false
				for _, existing := range apiFiles {
					if existing == file {
						found = true
						break
					}
				}
				if !found {
					apiFiles = append(apiFiles, file)
				}
			}
		}
	}

	if len(apiTokens) >= 2 && len(apiFiles) >= 1 {
		correlations = append(correlations, CorrelationGroup{
			Pattern:     "api_authentication",
			Description: "API authentication tokens found together",
			Tokens:      apiTokens,
			Files:       apiFiles,
			Confidence:  0.7,
			DetectedAt:  time.Now(),
		})
	}

	t.correlations = correlations
}

// GetBundleID returns the unique bundle identifier
func (t *Tokenizer) GetBundleID() string {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.bundleID
}

// GetDuplicateGroups returns all duplicate secret groups
func (t *Tokenizer) GetDuplicateGroups() []DuplicateGroup {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	duplicates := make([]DuplicateGroup, 0, len(t.duplicateGroups))
	for _, group := range t.duplicateGroups {
		if group != nil && group.Count > 1 {
			duplicates = append(duplicates, *group)
		}
	}
	return duplicates
}

// GetFileStats returns statistics for a specific file
func (t *Tokenizer) GetFileStats(filePath string) (FileStats, bool) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	if stats, exists := t.fileStats[filePath]; exists && stats != nil {
		return *stats, true
	}
	return FileStats{}, false
}

// GetCacheStats returns cache performance statistics
func (t *Tokenizer) GetCacheStats() CacheStats {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.cacheStats
}

// normalizeSecret performs various normalizations on secret values for better correlation
func (t *Tokenizer) normalizeSecret(value string) string {
	// Track original value for statistics
	originalValue := value

	// 1. Trim whitespace
	value = strings.TrimSpace(value)

	// 2. Handle common case variations (but preserve case for actual secrets)
	// Only normalize if it looks like a common pattern, not actual credentials
	if len(value) < 8 { // Short values might be user names, etc.
		// Check if it's all letters (might be username)
		if matched, _ := regexp.MatchString(`^[a-zA-Z]+$`, value); matched {
			value = strings.ToLower(value)
		}
	}

	// 3. Remove common prefixes/suffixes that don't change secret meaning
	prefixes := []string{"Bearer ", "Basic ", "Token ", "API_KEY=", "PASSWORD=", "SECRET="}
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			value = strings.TrimPrefix(value, prefix)
			break
		}
	}

	// 4. Handle quotes (both single and double)
	if (strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) ||
		(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
		value = value[1 : len(value)-1]
	}

	// 5. Normalize common connection string patterns
	// Example: "user:pass@host" vs "user: pass @ host"
	value = regexp.MustCompile(`\s*:\s*`).ReplaceAllString(value, ":")
	value = regexp.MustCompile(`\s*@\s*`).ReplaceAllString(value, "@")

	// Track normalization statistics
	if value != originalValue {
		t.cacheStats.Total++
		// This is a bit of a hack to track normalization hits
		t.normalizedSecrets[value] = originalValue
	}

	return value
}

// generateSecretHash creates a consistent hash for secret deduplication
func (t *Tokenizer) generateSecretHash(normalizedValue string) string {
	h := hmac.New(sha256.New, []byte("secret-hash-salt"))
	h.Write([]byte(normalizedValue))
	hash := h.Sum(nil)
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes for shorter hash
}

// addSecretReference tracks where a token was used
func (t *Tokenizer) addSecretReference(token, filePath string) {
	if t.secretRefs == nil {
		t.secretRefs = make(map[string][]string)
	}

	// Check if file already exists for this token
	for _, existingFile := range t.secretRefs[token] {
		if existingFile == filePath {
			return // Already recorded
		}
	}

	t.secretRefs[token] = append(t.secretRefs[token], filePath)
}

// trackDuplicateSecret manages duplicate secret detection and tracking
func (t *Tokenizer) trackDuplicateSecret(secretHash, token, secretType, filePath string, normalizedValue string) {
	now := time.Now()

	if existing, exists := t.duplicateGroups[secretHash]; exists {
		// Update existing duplicate group
		existing.Count++
		existing.LastSeen = now

		// Add location if not already present
		for _, loc := range existing.Locations {
			if loc == filePath {
				return // Location already tracked
			}
		}
		existing.Locations = append(existing.Locations, filePath)
	} else {
		// Create new duplicate group
		t.duplicateGroups[secretHash] = &DuplicateGroup{
			SecretHash: secretHash,
			Token:      token,
			SecretType: secretType,
			Locations:  []string{filePath},
			Count:      1,
			FirstSeen:  now,
			LastSeen:   now,
		}
	}
}

// updateFileStats tracks statistics per file
func (t *Tokenizer) updateFileStats(filePath, secretType string) {
	if t.fileStats == nil {
		t.fileStats = make(map[string]*FileStats)
	}

	stats, exists := t.fileStats[filePath]
	if !exists {
		stats = &FileStats{
			FilePath:     filePath,
			SecretsFound: 0,
			TokensUsed:   0,
			SecretTypes:  make(map[string]int),
			ProcessedAt:  time.Now(),
		}
		t.fileStats[filePath] = stats
	}

	stats.SecretsFound++
	stats.TokensUsed++
	stats.SecretTypes[secretType]++
	stats.ProcessedAt = time.Now()
}

// Phase 2.2: Redaction Mapping System

// GenerateRedactionMapFile creates a redaction mapping file with optional encryption
func (t *Tokenizer) GenerateRedactionMapFile(profile, outputPath string, encrypt bool) error {
	// Analyze correlations before generating map
	t.analyzeCorrelations()

	// Get the redaction map
	redactionMap := t.GetRedactionMap(profile)

	// Encrypt if requested
	if encrypt {
		encryptionKey := make([]byte, 32)
		if _, err := rand.Read(encryptionKey); err != nil {
			return fmt.Errorf("failed to generate encryption key: %w", err)
		}

		encryptedMap, err := t.encryptRedactionMap(redactionMap, encryptionKey)
		if err != nil {
			return fmt.Errorf("failed to encrypt redaction map: %w", err)
		}

		redactionMap = encryptedMap
		redactionMap.IsEncrypted = true
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(redactionMap, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal redaction map: %w", err)
	}

	// Write to file with secure permissions
	if err := ioutil.WriteFile(outputPath, jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write redaction map file: %w", err)
	}

	return nil
}

// encryptRedactionMap encrypts sensitive parts of the redaction map
func (t *Tokenizer) encryptRedactionMap(redactionMap RedactionMap, encryptionKey []byte) (RedactionMap, error) {
	// Create cipher
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return redactionMap, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return redactionMap, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Encrypt the tokens map
	encryptedTokens := make(map[string]string)
	for token, originalValue := range redactionMap.Tokens {
		// Generate nonce
		nonce := make([]byte, gcm.NonceSize())
		if _, err := rand.Read(nonce); err != nil {
			return redactionMap, fmt.Errorf("failed to generate nonce: %w", err)
		}

		// Encrypt the original value
		encryptedValue := gcm.Seal(nonce, nonce, []byte(originalValue), nil)
		encryptedTokens[token] = hex.EncodeToString(encryptedValue)
	}

	// Create encrypted copy
	encryptedMap := redactionMap
	encryptedMap.Tokens = encryptedTokens
	encryptedMap.EncryptionKey = encryptionKey // Store key (won't be serialized due to json:"-")
	encryptedMap.IsEncrypted = true            // Mark as encrypted

	return encryptedMap, nil
}

// decryptRedactionMap decrypts an encrypted redaction map
func (t *Tokenizer) decryptRedactionMap(encryptedMap RedactionMap, encryptionKey []byte) (RedactionMap, error) {
	if !encryptedMap.IsEncrypted {
		return encryptedMap, nil // Not encrypted
	}

	// Create cipher
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return encryptedMap, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return encryptedMap, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt the tokens map
	decryptedTokens := make(map[string]string)
	for token, encryptedValue := range encryptedMap.Tokens {
		// Decode hex
		encryptedBytes, err := hex.DecodeString(encryptedValue)
		if err != nil {
			continue // Skip malformed entries
		}

		if len(encryptedBytes) < gcm.NonceSize() {
			continue // Invalid data
		}

		// Extract nonce and ciphertext
		nonce := encryptedBytes[:gcm.NonceSize()]
		ciphertext := encryptedBytes[gcm.NonceSize():]

		// Decrypt
		decryptedBytes, err := gcm.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			continue // Skip failed decryptions
		}

		decryptedTokens[token] = string(decryptedBytes)
	}

	// Create decrypted copy
	decryptedMap := encryptedMap
	decryptedMap.Tokens = decryptedTokens
	decryptedMap.IsEncrypted = false

	return decryptedMap, nil
}

// LoadRedactionMapFile loads and optionally decrypts a redaction mapping file
func LoadRedactionMapFile(filePath string, encryptionKey []byte) (RedactionMap, error) {
	// Read file
	jsonData, err := ioutil.ReadFile(filePath)
	if err != nil {
		return RedactionMap{}, fmt.Errorf("failed to read redaction map file: %w", err)
	}

	// Parse JSON
	var redactionMap RedactionMap
	if err := json.Unmarshal(jsonData, &redactionMap); err != nil {
		return RedactionMap{}, fmt.Errorf("failed to parse redaction map: %w", err)
	}

	// Decrypt if needed and key provided
	if redactionMap.IsEncrypted && len(encryptionKey) > 0 {
		tokenizer := &Tokenizer{} // Temporary instance for decryption
		decryptedMap, err := tokenizer.decryptRedactionMap(redactionMap, encryptionKey)
		if err != nil {
			return RedactionMap{}, fmt.Errorf("failed to decrypt redaction map: %w", err)
		}
		return decryptedMap, nil
	}

	return redactionMap, nil
}

// ValidateRedactionMapFile validates the structure and integrity of a redaction map file
func ValidateRedactionMapFile(filePath string) error {
	redactionMap, err := LoadRedactionMapFile(filePath, nil)
	if err != nil {
		return err
	}

	// Basic validation checks
	if redactionMap.BundleID == "" {
		return fmt.Errorf("invalid redaction map: missing bundle ID")
	}

	if redactionMap.Stats.TotalSecrets != len(redactionMap.Tokens) {
		return fmt.Errorf("invalid redaction map: stats mismatch (expected %d secrets, found %d)",
			redactionMap.Stats.TotalSecrets, len(redactionMap.Tokens))
	}

	// Validate token format
	tokenizer := &Tokenizer{}
	for token := range redactionMap.Tokens {
		if !tokenizer.ValidateToken(token) {
			return fmt.Errorf("invalid token format: %s", token)
		}
	}

	return nil
}
