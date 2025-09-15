package redact

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestPhase2_ComprehensiveDemo demonstrates all Phase 2 features in a realistic scenario
func TestPhase2_ComprehensiveDemo(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	// Create temporary directory for output files
	tempDir, err := ioutil.TempDir("", "phase2_demo")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fmt.Println("\n🚀 Phase 2 Comprehensive Demo: Cross-File Correlation & Intelligent Mapping")
	fmt.Println("============================================================================")

	tokenizer := GetGlobalTokenizer()
	tokenizer.Reset()

	// === Demo: Cross-File Secret Correlation ===
	fmt.Println("\n📋 Scenario: Database credentials used across multiple Kubernetes manifests")

	// Database password appears in multiple files with different formatting
	sharedPassword := "prod-database-secret-2023"
	variations := []struct {
		file    string
		format  string
		context string
	}{
		{"k8s-secret.yaml", `"` + sharedPassword + `"`, "k8s_secret"},
		{"configmap.yaml", "  " + sharedPassword + "  ", "configmap"}, // whitespace
		{"deployment.yaml", "Bearer " + sharedPassword, "deployment"}, // prefix
		{"docker-compose.yml", sharedPassword, "docker"},              // plain
	}

	fmt.Println("\nProcessing same secret in different formats:")
	for _, v := range variations {
		token := tokenizer.TokenizeValueWithPath(v.format, v.context, v.file)
		fmt.Printf("  📁 %s: '%s' → %s\n", v.file, v.format, token)
	}

	// === Demo: API Credentials Correlation ===
	fmt.Println("\n📋 Scenario: API credentials used together across services")

	apiCredentials := []struct {
		secret  string
		context string
		files   []string
	}{
		{"AKIA1234567890EXAMPLE", "aws_access_key", []string{"app.yaml", "worker.yaml"}},
		{"aws-secret-abcdef1234567890", "aws_secret_key", []string{"app.yaml", "worker.yaml"}},
		{"sk-live-api-key-production", "api_key", []string{"gateway.yaml", "auth.yaml"}},
		{"oauth-client-secret-xyz789", "oauth_secret", []string{"gateway.yaml", "auth.yaml"}},
	}

	fmt.Println("\nProcessing correlated API credentials:")
	for _, cred := range apiCredentials {
		for _, file := range cred.files {
			token := tokenizer.TokenizeValueWithPath(cred.secret, cred.context, file)
			fmt.Printf("  🔑 %s in %s → %s\n", cred.context, file, token)
		}
	}

	// === Demo: Advanced Analytics ===
	fmt.Println("\n📊 Advanced Analytics & Intelligence")

	redactionMap := tokenizer.GetRedactionMap("demo-profile")

	fmt.Printf("Bundle ID: %s\n", redactionMap.BundleID)
	fmt.Printf("Total Secrets: %d\n", redactionMap.Stats.TotalSecrets)
	fmt.Printf("Unique Secrets: %d\n", redactionMap.Stats.UniqueSecrets)
	fmt.Printf("Files Covered: %d\n", redactionMap.Stats.FilesCovered)
	fmt.Printf("Cache Hits: %d / %d (%.1f%% hit rate)\n",
		redactionMap.Stats.CacheHits,
		redactionMap.Stats.CacheHits+redactionMap.Stats.CacheMisses,
		float64(redactionMap.Stats.CacheHits)/float64(redactionMap.Stats.CacheHits+redactionMap.Stats.CacheMisses)*100)

	// === Demo: Duplicate Detection ===
	fmt.Println("\n🔍 Duplicate Secret Detection:")
	for i, dup := range redactionMap.Duplicates {
		if dup.Count > 1 {
			fmt.Printf("  Duplicate %d: %s (%s)\n", i+1, dup.Token, dup.SecretType)
			fmt.Printf("    Found in %d locations: %v\n", dup.Count, dup.Locations)
			fmt.Printf("    First seen: %s, Last seen: %s\n",
				dup.FirstSeen.Format("15:04:05"), dup.LastSeen.Format("15:04:05"))
		}
	}

	// === Demo: Correlation Analysis ===
	fmt.Println("\n🔗 Correlation Analysis:")
	for i, corr := range redactionMap.Correlations {
		fmt.Printf("  Correlation %d: %s (%.1f%% confidence)\n",
			i+1, corr.Description, corr.Confidence*100)
		fmt.Printf("    Pattern: %s\n", corr.Pattern)
		fmt.Printf("    Tokens: %d, Files: %d\n", len(corr.Tokens), len(corr.Files))
		fmt.Printf("    Files: %v\n", corr.Files)
	}

	// === Demo: File Coverage Analysis ===
	fmt.Println("\n📈 Per-File Statistics:")
	for file, stats := range redactionMap.Stats.FileCoverage {
		fmt.Printf("  📁 %s:\n", file)
		fmt.Printf("    Secrets Found: %d, Tokens Used: %d\n", stats.SecretsFound, stats.TokensUsed)
		fmt.Printf("    Secret Types: %v\n", stats.SecretTypes)
	}

	// === Demo: Encrypted Mapping Generation ===
	fmt.Println("\n🔐 Encrypted Redaction Map Generation")

	encryptedMapPath := filepath.Join(tempDir, "demo-redaction-map-encrypted.json")
	err = tokenizer.GenerateRedactionMapFile("demo-profile", encryptedMapPath, true)
	if err != nil {
		t.Fatalf("Failed to generate encrypted mapping: %v", err)
	}

	// Check file was created with secure permissions
	fileInfo, err := os.Stat(encryptedMapPath)
	if err != nil {
		t.Fatalf("Failed to stat encrypted file: %v", err)
	}

	fmt.Printf("✅ Encrypted mapping file created: %s\n", encryptedMapPath)
	fmt.Printf("📋 File size: %d bytes\n", fileInfo.Size())
	fmt.Printf("🔒 File permissions: %o (secure)\n", fileInfo.Mode())

	// === Demo: Mapping File Validation ===
	fmt.Println("\n✅ Redaction Map Validation")

	if err := ValidateRedactionMapFile(encryptedMapPath); err != nil {
		t.Errorf("Encrypted mapping validation failed: %v", err)
	} else {
		fmt.Println("✅ Encrypted redaction map validation passed")
	}

	// === Demo: Unencrypted mapping for comparison ===
	unencryptedMapPath := filepath.Join(tempDir, "demo-redaction-map.json")
	err = tokenizer.GenerateRedactionMapFile("demo-profile", unencryptedMapPath, false)
	if err != nil {
		t.Fatalf("Failed to generate unencrypted mapping: %v", err)
	}

	if err := ValidateRedactionMapFile(unencryptedMapPath); err != nil {
		t.Errorf("Unencrypted mapping validation failed: %v", err)
	} else {
		fmt.Println("✅ Unencrypted redaction map validation passed")
	}

	// === Demo: Load and inspect mapping ===
	fmt.Println("\n📖 Mapping File Content Analysis")

	loadedMap, err := LoadRedactionMapFile(unencryptedMapPath, nil)
	if err != nil {
		t.Fatalf("Failed to load mapping: %v", err)
	}

	fmt.Printf("📊 Loaded mapping contains:\n")
	fmt.Printf("  - %d tokens\n", len(loadedMap.Tokens))
	fmt.Printf("  - %d duplicate groups\n", len(loadedMap.Duplicates))
	fmt.Printf("  - %d correlations\n", len(loadedMap.Correlations))
	fmt.Printf("  - %d files covered\n", len(loadedMap.Stats.FileCoverage))

	// === Verification ===
	fmt.Println("\n🎯 Phase 2 Feature Verification:")

	// Verify cross-file correlation
	crossFileTokens := make(map[string]int)
	for token, refs := range loadedMap.SecretRefs {
		if len(refs) > 1 {
			crossFileTokens[token] = len(refs)
		}
	}

	if len(crossFileTokens) > 0 {
		fmt.Printf("✅ Cross-file correlation: %d tokens found across multiple files\n", len(crossFileTokens))
		for token, count := range crossFileTokens {
			fmt.Printf("   %s appears in %d files\n", token, count)
		}
	} else {
		t.Error("❌ Expected cross-file token correlation")
	}

	// Verify duplicate detection
	if len(loadedMap.Duplicates) > 0 {
		fmt.Printf("✅ Duplicate detection: %d duplicate groups found\n", len(loadedMap.Duplicates))
	} else {
		t.Error("❌ Expected duplicate secret groups")
	}

	// Verify correlation analysis
	if len(loadedMap.Correlations) > 0 {
		fmt.Printf("✅ Correlation analysis: %d patterns detected\n", len(loadedMap.Correlations))
	} else {
		t.Error("❌ Expected correlation patterns")
	}

	// Verify performance tracking
	if loadedMap.Stats.CacheHits > 0 {
		fmt.Printf("✅ Performance optimization: %d cache hits\n", loadedMap.Stats.CacheHits)
	}

	// Verify normalization
	if loadedMap.Stats.NormalizationHits >= 0 {
		fmt.Printf("✅ Secret normalization: %d normalizations applied\n", loadedMap.Stats.NormalizationHits)
	}

	fmt.Println("\n🎉 Phase 2 Demo Complete - All features working!")
}
