package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/redact"
)

// Test program to demonstrate tokenization on the comprehensive sample_secrets.yaml file
//
// Usage:
//   cd pkg/redact/testdata
//   TROUBLESHOOT_TOKENIZATION=1 go run test_sample_secrets.go
//
// This will:
// 1. Load the comprehensive sample_secrets.yaml file
// 2. Apply tokenization redaction
// 3. Show statistics and sample tokenized lines
// 4. Save the redacted output for inspection

func main() {
	// Read the comprehensive sample secrets file
	data, err := os.ReadFile("sample_secrets.yaml")
	if err != nil {
		fmt.Printf("Error reading sample_secrets.yaml: %v\n", err)
		fmt.Println("Make sure you're running this from the pkg/redact/testdata directory")
		return
	}

	fmt.Println("=== COMPREHENSIVE SAMPLE SECRETS TOKENIZATION TEST ===")
	fmt.Printf("Original file size: %d bytes\n", len(data))
	fmt.Printf("Total lines in file: %d\n", len(strings.Split(string(data), "\n")))
	fmt.Println()

	// Apply redaction with tokenization
	input := strings.NewReader(string(data))
	redacted, err := redact.Redact(input, "sample_secrets.yaml", []*troubleshootv1beta2.Redact{})
	if err != nil {
		fmt.Printf("Error during redaction: %v\n", err)
		return
	}

	// Copy and analyze result
	var buf strings.Builder
	io.Copy(&buf, redacted)
	result := buf.String()

	// Statistics
	tokenCount := strings.Count(result, "***TOKEN_")
	hiddenCount := strings.Count(result, "***HIDDEN***")

	fmt.Printf("Redacted file size: %d bytes\n", len(result))
	fmt.Printf("Total tokens generated: %d\n", tokenCount)
	fmt.Printf("Total hidden values: %d\n", hiddenCount)

	if tokenCount > 0 {
		fmt.Println("✅ Tokenization is ENABLED")
	} else if hiddenCount > 0 {
		fmt.Println("ℹ️  Tokenization is DISABLED (using ***HIDDEN***)")
	} else {
		fmt.Println("⚠️  No redaction occurred")
	}
	fmt.Println()

	// Analyze token types (only if tokenization is enabled)
	if tokenCount > 0 {
		tokenTypes := make(map[string]int)
		lines := strings.Split(result, "\n")

		for _, line := range lines {
			if strings.Contains(line, "***TOKEN_") {
				// Extract token type
				start := strings.Index(line, "***TOKEN_")
				if start != -1 {
					end := strings.Index(line[start+9:], "_")
					if end != -1 {
						tokenType := line[start+9 : start+9+end]
						tokenTypes[tokenType]++
					}
				}
			}
		}

		fmt.Println("Token types generated:")
		totalTokensVerified := 0
		for tokenType, count := range tokenTypes {
			fmt.Printf("  %-12s: %d occurrences\n", tokenType, count)
			totalTokensVerified += count
		}
		fmt.Printf("  %-12s: %d total\n", "TOTAL", totalTokensVerified)
		fmt.Println()
	}

	// Show sample redacted lines
	lines := strings.Split(result, "\n")
	fmt.Println("Sample redacted lines (first 25):")
	count := 0
	for _, line := range lines {
		if (strings.Contains(line, "***TOKEN_") || strings.Contains(line, "***HIDDEN***")) && count < 25 {
			fmt.Printf("  %s\n", strings.TrimSpace(line))
			count++
		}
	}

	if count == 0 {
		fmt.Println("  (No redacted lines found)")
	}
	fmt.Println()

	// Verify some critical secrets are redacted
	criticalSecrets := []string{
		"super_secret_db_password_123",
		"sk-1234567890abcdefghijklmnopqrstuvwxyz",
		"MyComplexPassword!@#$%",
		"jwt_signing_key_very_long_and_secure_ghi789",
		"basic_yaml_password_123",
		"nested_db_password_123",
		"k8s_db_password_123",
		"json_db_password_123456789abcdef",
	}

	fmt.Println("Critical secrets redaction check:")
	allRedacted := true
	for _, secret := range criticalSecrets {
		if strings.Contains(result, secret) {
			fmt.Printf("  ❌ EXPOSED: '%s'\n", secret)
			allRedacted = false
		} else {
			fmt.Printf("  ✅ REDACTED: '%s'\n", secret)
		}
	}

	if allRedacted {
		fmt.Println("✅ All critical secrets successfully redacted!")
	} else {
		fmt.Println("❌ Some critical secrets were not redacted!")
	}
	fmt.Println()

	// Save redacted output for inspection
	outputFile := "sample_secrets_redacted.yaml"
	err = os.WriteFile(outputFile, []byte(result), 0644)
	if err != nil {
		fmt.Printf("Error saving redacted output: %v\n", err)
	} else {
		fmt.Printf("Redacted output saved to: %s\n", outputFile)
		fmt.Println("You can inspect this file to see the tokenization results.")
	}

	fmt.Println()
	fmt.Println("=== TEST COMPLETE ===")
	fmt.Println()
	fmt.Println("To test different modes:")
	fmt.Println("  Without tokenization: go run test_sample_secrets.go")
	fmt.Println("  With tokenization:    TROUBLESHOOT_TOKENIZATION=1 go run test_sample_secrets.go")
}
