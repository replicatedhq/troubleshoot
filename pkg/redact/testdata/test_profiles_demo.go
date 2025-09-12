package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/redact"
)

func main() {
	// Enable tokenization for better visibility
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "1")

	fmt.Println("🔒 Redaction Profile System Demo")
	fmt.Println("=================================")
	fmt.Printf("Timestamp: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("Tokenization: %s\n\n", os.Getenv("TROUBLESHOOT_TOKENIZATION"))

	// Load sample data
	sampleFile := filepath.Join("sample_profiles.yaml")
	data, err := os.ReadFile(sampleFile)
	if err != nil {
		fmt.Printf("❌ Error reading sample file: %v\n", err)
		fmt.Println("Make sure you're running this from the pkg/redact/testdata/ directory")
		return
	}

	fmt.Printf("📄 Loaded sample data: %d bytes\n", len(data))
	fmt.Printf("📄 Sample file: %s\n\n", sampleFile)

	// Test all profiles
	profiles := []string{"minimal", "standard", "comprehensive", "paranoid"}
	results := make(map[string]ProfileResult)

	for _, profileName := range profiles {
		fmt.Printf("🔍 Testing Profile: %s\n", strings.ToUpper(profileName))
		fmt.Println(strings.Repeat("-", 40))

		result, err := testProfile(profileName, string(data))
		if err != nil {
			fmt.Printf("❌ Error testing profile %s: %v\n\n", profileName, err)
			continue
		}

		results[profileName] = result
		printProfileResult(profileName, result)
		fmt.Println()
	}

	// Summary comparison
	printSummaryComparison(results)

	// Save detailed results
	saveDetailedResults(results, string(data))

	fmt.Println("\n🎉 Profile demo complete!")
	fmt.Println("\nFor reviewers:")
	fmt.Println("- Check the generated *_redacted.yaml files to see the redaction results")
	fmt.Println("- Notice how higher profiles catch progressively more sensitive data")
	fmt.Println("- Verify that tokens are consistent (same value = same token)")
	fmt.Println("- Verify that tokens are unique (different values = different tokens)")
}

type ProfileResult struct {
	ProfileName    string
	TokenCount     int
	HiddenCount    int
	ProcessingTime time.Duration
	RedactedData   string
	TokenTypes     map[string]int
	SampleTokens   []string
}

func testProfile(profileName string, testData string) (ProfileResult, error) {
	start := time.Now()

	// Set the active profile
	err := redact.SetRedactionProfile(profileName)
	if err != nil {
		return ProfileResult{}, fmt.Errorf("failed to set profile: %w", err)
	}

	// Apply redaction
	input := strings.NewReader(testData)
	redacted, err := redact.Redact(input, "sample_profiles.yaml", []*troubleshootv1beta2.Redact{})
	if err != nil {
		return ProfileResult{}, fmt.Errorf("redaction failed: %w", err)
	}

	// Read the result
	var output strings.Builder
	_, err = io.Copy(&output, redacted)
	if err != nil {
		return ProfileResult{}, fmt.Errorf("failed to read redacted output: %w", err)
	}

	result := output.String()
	processingTime := time.Since(start)

	// Analyze results
	tokenCount := strings.Count(result, "***TOKEN_")
	hiddenCount := strings.Count(result, "***HIDDEN***")
	tokenTypes := analyzeTokenTypes(result)
	sampleTokens := extractSampleTokens(result, 5)

	return ProfileResult{
		ProfileName:    profileName,
		TokenCount:     tokenCount,
		HiddenCount:    hiddenCount,
		ProcessingTime: processingTime,
		RedactedData:   result,
		TokenTypes:     tokenTypes,
		SampleTokens:   sampleTokens,
	}, nil
}

func analyzeTokenTypes(redactedData string) map[string]int {
	tokenTypes := make(map[string]int)
	lines := strings.Split(redactedData, "\n")

	for _, line := range lines {
		if strings.Contains(line, "***TOKEN_") {
			// Extract token type
			start := strings.Index(line, "***TOKEN_")
			if start != -1 {
				tokenStart := start + 9 // len("***TOKEN_")
				remaining := line[tokenStart:]
				end := strings.Index(remaining, "_")
				if end != -1 {
					tokenType := remaining[:end]
					tokenTypes[tokenType]++
				}
			}
		}
	}

	return tokenTypes
}

func extractSampleTokens(redactedData string, limit int) []string {
	var tokens []string
	lines := strings.Split(redactedData, "\n")

	for _, line := range lines {
		if strings.Contains(line, "***TOKEN_") && len(tokens) < limit {
			tokens = append(tokens, strings.TrimSpace(line))
		}
	}

	return tokens
}

func printProfileResult(profileName string, result ProfileResult) {
	// Profile description
	descriptions := map[string]string{
		"minimal":       "Basic passwords, API keys, tokens",
		"standard":      "↳ + IP addresses, URLs, emails",
		"comprehensive": "↳ + usernames, hostnames, file paths",
		"paranoid":      "↳ + long strings, UUIDs, phone numbers, SSNs",
	}

	fmt.Printf("📋 Description: %s\n", descriptions[profileName])
	fmt.Printf("⚡ Processing time: %v\n", result.ProcessingTime)
	fmt.Printf("🔢 Tokens found: %d\n", result.TokenCount)
	fmt.Printf("🔢 Hidden values: %d\n", result.HiddenCount)

	// Token types breakdown
	if len(result.TokenTypes) > 0 {
		fmt.Printf("🏷️  Token types:\n")

		// Sort token types for consistent output
		var types []string
		for tokenType := range result.TokenTypes {
			types = append(types, tokenType)
		}
		sort.Strings(types)

		for _, tokenType := range types {
			count := result.TokenTypes[tokenType]
			fmt.Printf("   %s: %d\n", tokenType, count)
		}
	}

	// Sample redacted lines
	if len(result.SampleTokens) > 0 {
		fmt.Printf("📝 Sample redacted lines:\n")
		for i, token := range result.SampleTokens {
			if i >= 3 { // Limit to 3 samples for readability
				break
			}
			fmt.Printf("   %s\n", token)
		}
		if len(result.SampleTokens) > 3 {
			fmt.Printf("   ... and %d more\n", len(result.SampleTokens)-3)
		}
	}

	// Effectiveness rating
	effectiveness := getEffectivenessRating(profileName, result.TokenCount)
	fmt.Printf("⭐ Effectiveness: %s\n", effectiveness)
}

func getEffectivenessRating(profileName string, tokenCount int) string {
	expectedRanges := map[string][2]int{
		"minimal":       {5, 15},
		"standard":      {15, 35},
		"comprehensive": {35, 60},
		"paranoid":      {60, 100},
	}

	if ranges, exists := expectedRanges[profileName]; exists {
		min, max := ranges[0], ranges[1]
		if tokenCount >= min && tokenCount <= max {
			return fmt.Sprintf("✅ Excellent (%d tokens in expected range %d-%d)", tokenCount, min, max)
		} else if tokenCount < min {
			return fmt.Sprintf("⚠️  Low (%d tokens, expected %d-%d)", tokenCount, min, max)
		} else {
			return fmt.Sprintf("🔥 High (%d tokens, expected %d-%d)", tokenCount, min, max)
		}
	}

	return fmt.Sprintf("📊 %d tokens detected", tokenCount)
}

func printSummaryComparison(results map[string]ProfileResult) {
	fmt.Println("📊 PROFILE COMPARISON SUMMARY")
	fmt.Println("=============================")

	profiles := []string{"minimal", "standard", "comprehensive", "paranoid"}

	fmt.Printf("%-15s %-8s %-8s %-12s %s\n", "Profile", "Tokens", "Hidden", "Time", "Token Types")
	fmt.Println(strings.Repeat("-", 65))

	for _, profile := range profiles {
		if result, exists := results[profile]; exists {
			typeCount := len(result.TokenTypes)
			fmt.Printf("%-15s %-8d %-8d %-12v %d types\n",
				strings.Title(profile),
				result.TokenCount,
				result.HiddenCount,
				result.ProcessingTime.Truncate(time.Millisecond),
				typeCount)
		}
	}

	fmt.Println()

	// Escalation analysis
	fmt.Println("📈 ESCALATION ANALYSIS")
	fmt.Println("======================")

	for i, profile := range profiles {
		if result, exists := results[profile]; exists {
			if i == 0 {
				fmt.Printf("%s: %d tokens (baseline)\n", strings.Title(profile), result.TokenCount)
			} else {
				prevProfile := profiles[i-1]
				if prevResult, prevExists := results[prevProfile]; prevExists {
					increase := result.TokenCount - prevResult.TokenCount
					percentage := float64(increase) / float64(prevResult.TokenCount) * 100
					fmt.Printf("%s: %d tokens (+%d, +%.1f%% vs %s)\n",
						strings.Title(profile), result.TokenCount, increase, percentage, prevProfile)
				}
			}
		}
	}
}

func saveDetailedResults(results map[string]ProfileResult, originalData string) {
	fmt.Println("\n💾 SAVING DETAILED RESULTS")
	fmt.Println("==========================")

	// Save original data for comparison
	originalFile := "sample_profiles_original.yaml"
	err := os.WriteFile(originalFile, []byte(originalData), 0644)
	if err != nil {
		fmt.Printf("❌ Error saving original file: %v\n", err)
	} else {
		fmt.Printf("📄 Original data saved: %s\n", originalFile)
	}

	// Save redacted results for each profile
	for profileName, result := range results {
		filename := fmt.Sprintf("sample_profiles_%s_redacted.yaml", profileName)
		err := os.WriteFile(filename, []byte(result.RedactedData), 0644)
		if err != nil {
			fmt.Printf("❌ Error saving %s results: %v\n", profileName, err)
		} else {
			fmt.Printf("📄 %s results saved: %s (%d bytes)\n",
				strings.Title(profileName), filename, len(result.RedactedData))
		}
	}

	// Create a summary report
	summaryFile := "profile_test_summary.txt"
	summary := generateSummaryReport(results)
	err = os.WriteFile(summaryFile, []byte(summary), 0644)
	if err != nil {
		fmt.Printf("❌ Error saving summary: %v\n", err)
	} else {
		fmt.Printf("📊 Summary report saved: %s\n", summaryFile)
	}
}

func generateSummaryReport(results map[string]ProfileResult) string {
	var report strings.Builder

	report.WriteString("REDACTION PROFILE SYSTEM - TEST SUMMARY REPORT\n")
	report.WriteString("===============================================\n")
	report.WriteString(fmt.Sprintf("Generated: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	report.WriteString(fmt.Sprintf("Tokenization: %s\n\n", os.Getenv("TROUBLESHOOT_TOKENIZATION")))

	profiles := []string{"minimal", "standard", "comprehensive", "paranoid"}

	for _, profileName := range profiles {
		if result, exists := results[profileName]; exists {
			report.WriteString(fmt.Sprintf("PROFILE: %s\n", strings.ToUpper(profileName)))
			report.WriteString(strings.Repeat("-", 30) + "\n")
			report.WriteString(fmt.Sprintf("Tokens detected: %d\n", result.TokenCount))
			report.WriteString(fmt.Sprintf("Hidden values: %d\n", result.HiddenCount))
			report.WriteString(fmt.Sprintf("Processing time: %v\n", result.ProcessingTime))
			report.WriteString(fmt.Sprintf("Token types: %d\n", len(result.TokenTypes)))

			if len(result.TokenTypes) > 0 {
				report.WriteString("Token type breakdown:\n")
				var types []string
				for tokenType := range result.TokenTypes {
					types = append(types, tokenType)
				}
				sort.Strings(types)

				for _, tokenType := range types {
					count := result.TokenTypes[tokenType]
					report.WriteString(fmt.Sprintf("  - %s: %d\n", tokenType, count))
				}
			}

			report.WriteString("\n")
		}
	}

	report.WriteString("RECOMMENDATIONS FOR REVIEWERS:\n")
	report.WriteString("==============================\n")
	report.WriteString("1. Compare the *_redacted.yaml files to see profile differences\n")
	report.WriteString("2. Verify token consistency (same values = same tokens)\n")
	report.WriteString("3. Verify token uniqueness (different values = different tokens)\n")
	report.WriteString("4. Check that higher profiles catch more sensitive data\n")
	report.WriteString("5. Ensure no critical secrets appear in plaintext\n")
	report.WriteString("6. Review token type classification accuracy\n")

	return report.String()
}
