package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

// GlobalProblemDescription is set by the CLI when --problem-description is provided
var GlobalProblemDescription string

// AnalyzeLLM is the analyzer that uses an LLM to analyze support bundle contents
type AnalyzeLLM struct {
	analyzer *troubleshootv1beta2.LLMAnalyze
}

func (a *AnalyzeLLM) Title() string {
	if a.analyzer.CheckName != "" {
		return a.analyzer.CheckName
	}
	return "LLM Problem Analysis"
}

func (a *AnalyzeLLM) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeLLM) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	// Get API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return []*AnalyzeResult{{
			Title:   a.Title(),
			IsFail:  true,
			Message: "OPENAI_API_KEY environment variable is required for LLM analyzer",
		}}, nil
	}

	// Get problem description from global variable (set by CLI) or environment
	problemDescription := GlobalProblemDescription
	if problemDescription == "" {
		problemDescription = os.Getenv("PROBLEM_DESCRIPTION")
	}
	if problemDescription == "" {
		problemDescription = "Please analyze the logs and identify any issues or problems"
	}

	// Collect files to analyze
	files, err := a.collectFiles(getFile, findFiles)
	if err != nil {
		return nil, errors.Wrap(err, "failed to collect files for LLM analysis")
	}

	if len(files) == 0 {
		// Debug: Try to see what files exist
		debugPattern := filepath.Join(a.analyzer.CollectorName, a.analyzer.FileName)
		debugFiles, _ := findFiles(debugPattern, []string{})
		debugMsg := fmt.Sprintf("No files found. Pattern: %s, Found count: %d", debugPattern, len(debugFiles))

		return []*AnalyzeResult{{
			Title:   a.Title(),
			IsWarn:  true,
			Message: debugMsg,
		}}, nil
	}

	// Call LLM API
	analysis, err := a.callLLM(apiKey, problemDescription, files)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call LLM API")
	}

	// Map LLM response to outcomes
	return a.mapToOutcomes(analysis), nil
}

func (a *AnalyzeLLM) collectFiles(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) (map[string]string, error) {
	files := make(map[string]string)
	totalSize := 0
	maxSize := 1024 * 1024 // 1MB default
	if a.analyzer.MaxSize > 0 {
		maxSize = a.analyzer.MaxSize * 1024 // Convert from KB to bytes
	}
	maxFiles := 20 // Increased default for more comprehensive analysis
	if a.analyzer.MaxFiles > 0 {
		maxFiles = a.analyzer.MaxFiles
	}
	
	// Default priority patterns if not specified
	priorityPatterns := a.analyzer.PriorityPatterns
	if len(priorityPatterns) == 0 {
		priorityPatterns = []string{"error", "fatal", "exception", "panic", "crash", "OOM", "kill", "fail"}
	}
	
	// Default skip patterns
	skipPatterns := a.analyzer.SkipPatterns
	if len(skipPatterns) == 0 {
		skipPatterns = []string{"*.png", "*.jpg", "*.jpeg", "*.gif", "*.ico", "*.svg", "*.zip", "*.tar", "*.gz"}
	}

	// If specific file pattern is provided
	if a.analyzer.FileName != "" || a.analyzer.CollectorName != "" {
		var globPattern string
		if a.analyzer.CollectorName != "" && a.analyzer.FileName != "" {
			globPattern = fmt.Sprintf("%s/%s", a.analyzer.CollectorName, a.analyzer.FileName)
		} else if a.analyzer.CollectorName != "" {
			globPattern = fmt.Sprintf("%s/**/*", a.analyzer.CollectorName)
		} else {
			globPattern = a.analyzer.FileName
		}

		matchingFiles, err := findFiles(globPattern, []string{})
		if err != nil {
			return nil, errors.Wrap(err, "failed to find files")
		}

		// Score and sort files if using smart selection
		type scoredFile struct {
			path    string
			content []byte
			score   int
		}
		
		var scoredFiles []scoredFile
		for filePath, content := range matchingFiles {
			// Skip files based on patterns
			if a.shouldSkipFile(filePath, skipPatterns) {
				continue
			}
			
			// Skip binary files
			if isBinaryFile(content) {
				continue
			}
			
			score := a.fileScore(filePath, content, priorityPatterns)
			scoredFiles = append(scoredFiles, scoredFile{
				path:    filePath,
				content: content,
				score:   score,
			})
		}
		
		// Sort by score (highest first)
		sort.Slice(scoredFiles, func(i, j int) bool {
			return scoredFiles[i].score > scoredFiles[j].score
		})
		
		// Add files up to limits
		for _, sf := range scoredFiles {
			if len(files) >= maxFiles {
				break
			}
			if totalSize+len(sf.content) > maxSize {
				break
			}
			files[sf.path] = string(sf.content)
			totalSize += len(sf.content)
		}
	} else {
		// Default: get some common log files
		patterns := []string{
			"cluster-resources/pods/*.json",
			"cluster-resources/events/*.json",
			"*/logs/*.log",
		}

		for _, pattern := range patterns {
			if len(files) >= maxFiles {
				break
			}

			matchingFiles, err := findFiles(pattern, []string{})
			if err != nil {
				continue
			}

			for filePath, content := range matchingFiles {
				if len(files) >= maxFiles {
					break
				}

				if totalSize+len(content) > maxSize {
					break
				}

				files[filePath] = string(content)
				totalSize += len(content)
			}
		}
	}

	return files, nil
}

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
	Timeout  int             `json:"timeout,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

type llmAnalysis struct {
	IssueFound    bool     `json:"issue_found"`
	Summary       string   `json:"summary"`
	Issue         string   `json:"issue"`
	Solution      string   `json:"solution"`
	Severity      string   `json:"severity"` // "critical", "warning", "info"
	Confidence    float64  `json:"confidence"`
	// Enhanced fields for more actionable output
	Commands      []string `json:"commands,omitempty"`       // kubectl commands to run
	Documentation []string `json:"documentation,omitempty"` // relevant doc links
	RootCause     string   `json:"root_cause,omitempty"`     // identified root cause
	AffectedPods  []string `json:"affected_pods,omitempty"`  // list of affected resources
	NextSteps     []string `json:"next_steps,omitempty"`     // ordered action items
	RelatedIssues []string `json:"related_issues,omitempty"` // other potential problems
}

func (a *AnalyzeLLM) callLLM(apiKey, problemDescription string, files map[string]string) (*llmAnalysis, error) {
	// Build prompt
	var promptBuilder strings.Builder
	promptBuilder.WriteString("You are analyzing a Kubernetes support bundle to identify issues.\n\n")
	promptBuilder.WriteString(fmt.Sprintf("Problem Description: %s\n\n", problemDescription))
	promptBuilder.WriteString("Please analyze the following files and respond with a JSON object containing:\n")
	promptBuilder.WriteString("- issue_found (boolean): whether an issue was identified\n")
	promptBuilder.WriteString("- summary (string): brief summary of findings\n")
	promptBuilder.WriteString("- issue (string): detailed description of the issue if found\n")
	promptBuilder.WriteString("- solution (string): recommended solution if an issue was found\n")
	promptBuilder.WriteString("- severity (string): 'critical', 'warning', or 'info'\n")
	promptBuilder.WriteString("- confidence (number): confidence level from 0.0 to 1.0\n")
	promptBuilder.WriteString("- commands (array): kubectl commands that could help resolve the issue\n")
	promptBuilder.WriteString("- documentation (array): relevant Kubernetes documentation URLs\n")
	promptBuilder.WriteString("- root_cause (string): the identified root cause of the problem\n")
	promptBuilder.WriteString("- affected_pods (array): list of affected pod names\n")
	promptBuilder.WriteString("- next_steps (array): ordered list of recommended actions\n")
	promptBuilder.WriteString("- related_issues (array): other potential problems found\n\n")
	promptBuilder.WriteString("Files to analyze:\n\n")

	for path, content := range files {
		promptBuilder.WriteString(fmt.Sprintf("=== %s ===\n", path))
		// Truncate very long files (20K chars for more context)
		if len(content) > 20000 {
			content = content[:20000] + "\n... (truncated)"
		}
		promptBuilder.WriteString(content)
		promptBuilder.WriteString("\n\n")
	}

	// Determine model (default to cost-effective option)
	model := "gpt-4o-mini"
	if a.analyzer.Model != "" {
		model = a.analyzer.Model
	}

	// Create request
	reqBody := openAIRequest{
		Model: model,
		Messages: []openAIMessage{
			{
				Role:    "system",
				Content: "You are a Kubernetes troubleshooting expert. Analyze support bundles and identify issues. Always respond with valid JSON.",
			},
			{
				Role:    "user",
				Content: promptBuilder.String(),
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal request")
	}

	// Create HTTP request with timeout (120s for large analyses)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	// Make request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call OpenAI API")
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response")
	}

	// Parse response
	var openAIResp openAIResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return nil, errors.Wrap(err, "failed to parse OpenAI response")
	}

	// Check for API error
	if openAIResp.Error != nil {
		return nil, fmt.Errorf("OpenAI API error: %s", openAIResp.Error.Message)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, errors.New("no response from OpenAI API")
	}

	// Parse LLM's JSON response
	var analysis llmAnalysis
	content := openAIResp.Choices[0].Message.Content

	// Try to extract JSON from the response (in case LLM added extra text)
	startIdx := strings.Index(content, "{")
	endIdx := strings.LastIndex(content, "}")
	if startIdx >= 0 && endIdx >= 0 && endIdx > startIdx {
		content = content[startIdx : endIdx+1]
	}

	if err := json.Unmarshal([]byte(content), &analysis); err != nil {
		// Log the parsing error for debugging
		fmt.Fprintf(os.Stderr, "Warning: Failed to parse LLM JSON response: %v\nRaw response: %s\n", err, content)
		
		// If JSON parsing fails, create a basic analysis from the text
		analysis = llmAnalysis{
			IssueFound: strings.Contains(strings.ToLower(content), "error") ||
				strings.Contains(strings.ToLower(content), "fail") ||
				strings.Contains(strings.ToLower(content), "issue"),
			Summary:    content,
			Severity:   "warning",
			Confidence: 0.5,
		}
	}

	return &analysis, nil
}

func (a *AnalyzeLLM) mapToOutcomes(analysis *llmAnalysis) []*AnalyzeResult {
	result := &AnalyzeResult{
		Title: a.Title(),
	}

	// Build enhanced message with all available information
	message := analysis.Summary
	if analysis.RootCause != "" {
		message = fmt.Sprintf("%s\n\nRoot Cause: %s", message, analysis.RootCause)
	}
	if analysis.Issue != "" {
		message = fmt.Sprintf("%s\n\nIssue: %s", message, analysis.Issue)
	}
	if analysis.Solution != "" {
		message = fmt.Sprintf("%s\n\nSolution: %s", message, analysis.Solution)
	}
	if len(analysis.NextSteps) > 0 {
		message = fmt.Sprintf("%s\n\nNext Steps:", message)
		for i, step := range analysis.NextSteps {
			message = fmt.Sprintf("%s\n%d. %s", message, i+1, step)
		}
	}
	if len(analysis.Commands) > 0 {
		message = fmt.Sprintf("%s\n\nRecommended Commands:", message)
		for _, cmd := range analysis.Commands {
			message = fmt.Sprintf("%s\n  %s", message, cmd)
		}
	}
	if len(analysis.AffectedPods) > 0 {
		message = fmt.Sprintf("%s\n\nAffected Pods: %s", message, strings.Join(analysis.AffectedPods, ", "))
	}
	if len(analysis.Documentation) > 0 {
		message = fmt.Sprintf("%s\n\nRelevant Documentation:", message)
		for _, doc := range analysis.Documentation {
			message = fmt.Sprintf("%s\n  - %s", message, doc)
		}
	}
	if len(analysis.RelatedIssues) > 0 {
		message = fmt.Sprintf("%s\n\nRelated Issues:", message)
		for _, issue := range analysis.RelatedIssues {
			message = fmt.Sprintf("%s\n  - %s", message, issue)
		}
	}

	// Map to outcomes based on analysis
	if !analysis.IssueFound {
		result.IsPass = true
		result.Message = "No issues detected by LLM analysis"
	} else {
		switch analysis.Severity {
		case "critical":
			result.IsFail = true
			result.Message = message
		case "warning":
			result.IsWarn = true
			result.Message = message
		default:
			result.IsPass = true
			result.Message = message
		}
	}

	// If we have specific outcomes defined, use those instead
	if len(a.analyzer.Outcomes) > 0 {
		for _, outcome := range a.analyzer.Outcomes {
			if outcome.Fail != nil && analysis.IssueFound && analysis.Severity == "critical" {
				if outcome.Fail.When == "" || outcome.Fail.When == "issue_found" {
					result.IsFail = true
					result.IsWarn = false
					result.IsPass = false
					if outcome.Fail.Message != "" {
						result.Message = a.replaceTemplateVars(outcome.Fail.Message, analysis)
					} else {
						result.Message = message
					}
					break
				}
			} else if outcome.Warn != nil && analysis.IssueFound && analysis.Severity == "warning" {
				if outcome.Warn.When == "" || outcome.Warn.When == "potential_issue" {
					result.IsFail = false
					result.IsWarn = true
					result.IsPass = false
					if outcome.Warn.Message != "" {
						result.Message = a.replaceTemplateVars(outcome.Warn.Message, analysis)
					} else {
						result.Message = message
					}
					break
				}
			} else if outcome.Pass != nil && !analysis.IssueFound {
				if outcome.Pass.When == "" {
					result.IsFail = false
					result.IsWarn = false
					result.IsPass = true
					if outcome.Pass.Message != "" {
						result.Message = outcome.Pass.Message
					}
					break
				}
			}
		}
	}

	return []*AnalyzeResult{result}
}

// GenerateMarkdownReport creates a detailed Markdown report from the analysis
func (a *AnalyzeLLM) GenerateMarkdownReport(analysis *llmAnalysis) string {
	var report strings.Builder
	
	report.WriteString("# LLM Analysis Report\n\n")
	
	// Executive Summary
	report.WriteString("## Executive Summary\n")
	report.WriteString(fmt.Sprintf("**Status:** %s\n", func() string {
		if !analysis.IssueFound {
			return "✅ No Issues Found"
		}
		switch analysis.Severity {
		case "critical":
			return "🔴 Critical Issue"
		case "warning":
			return "🟡 Warning"
		default:
			return "ℹ️ Information"
		}
	}()))
	report.WriteString(fmt.Sprintf("**Confidence:** %.0f%%\n\n", analysis.Confidence*100))
	report.WriteString(fmt.Sprintf("%s\n\n", analysis.Summary))
	
	if analysis.IssueFound {
		// Root Cause
		if analysis.RootCause != "" {
			report.WriteString("## Root Cause Analysis\n")
			report.WriteString(fmt.Sprintf("%s\n\n", analysis.RootCause))
		}
		
		// Issue Details
		if analysis.Issue != "" {
			report.WriteString("## Issue Details\n")
			report.WriteString(fmt.Sprintf("%s\n\n", analysis.Issue))
		}
		
		// Solution
		if analysis.Solution != "" {
			report.WriteString("## Recommended Solution\n")
			report.WriteString(fmt.Sprintf("%s\n\n", analysis.Solution))
		}
		
		// Next Steps
		if len(analysis.NextSteps) > 0 {
			report.WriteString("## Action Plan\n")
			for i, step := range analysis.NextSteps {
				report.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
			}
			report.WriteString("\n")
		}
		
		// Commands
		if len(analysis.Commands) > 0 {
			report.WriteString("## Recommended Commands\n")
			report.WriteString("```bash\n")
			for _, cmd := range analysis.Commands {
				report.WriteString(fmt.Sprintf("%s\n", cmd))
			}
			report.WriteString("```\n\n")
		}
		
		// Affected Resources
		if len(analysis.AffectedPods) > 0 {
			report.WriteString("## Affected Resources\n")
			report.WriteString("### Pods\n")
			for _, pod := range analysis.AffectedPods {
				report.WriteString(fmt.Sprintf("- %s\n", pod))
			}
			report.WriteString("\n")
		}
		
		// Related Issues
		if len(analysis.RelatedIssues) > 0 {
			report.WriteString("## Related Issues\n")
			for _, issue := range analysis.RelatedIssues {
				report.WriteString(fmt.Sprintf("- %s\n", issue))
			}
			report.WriteString("\n")
		}
		
		// Documentation
		if len(analysis.Documentation) > 0 {
			report.WriteString("## References\n")
			for _, doc := range analysis.Documentation {
				report.WriteString(fmt.Sprintf("- [%s](%s)\n", doc, doc))
			}
			report.WriteString("\n")
		}
	}
	
	report.WriteString("---\n")
	report.WriteString("*Generated by Troubleshoot LLM Analyzer*\n")
	
	return report.String()
}

// fileScore calculates a relevance score for a file based on its path and content
func (a *AnalyzeLLM) fileScore(filePath string, content []byte, priorityPatterns []string) int {
	score := 0
	contentStr := strings.ToLower(string(content))
	filePathLower := strings.ToLower(filePath)
	
	// Check for priority patterns in content and filename
	for _, pattern := range priorityPatterns {
		patternLower := strings.ToLower(pattern)
		// Higher score for content matches
		score += strings.Count(contentStr, patternLower) * 2
		// Lower score for filename matches
		if strings.Contains(filePathLower, patternLower) {
			score += 5
		}
	}
	
	// Boost score for log files
	if strings.HasSuffix(filePathLower, ".log") {
		score += 10
	}
	
	// Slightly lower score for JSON (structured but less readable)
	if strings.HasSuffix(filePathLower, ".json") {
		score += 5
	}
	
	// Check for recent timestamps in content (simple heuristic)
	if strings.Contains(contentStr, "2024") || strings.Contains(contentStr, "2025") {
		score += 3
	}
	
	return score
}

// shouldSkipFile checks if a file should be skipped based on patterns
func (a *AnalyzeLLM) shouldSkipFile(filePath string, skipPatterns []string) bool {
	for _, pattern := range skipPatterns {
		matched, err := filepath.Match(pattern, filepath.Base(filePath))
		if err == nil && matched {
			return true
		}
	}
	
	// Check if file appears to be binary (simple heuristic)
	return false
}

// isBinaryFile performs a simple check to see if content appears to be binary
func isBinaryFile(content []byte) bool {
	if len(content) == 0 {
		return false
	}
	
	// Check first 512 bytes for null bytes (common in binary files)
	checkLen := 512
	if len(content) < checkLen {
		checkLen = len(content)
	}
	
	nullCount := 0
	for i := 0; i < checkLen; i++ {
		if content[i] == 0 {
			nullCount++
		}
	}
	
	// If more than 10% null bytes, probably binary
	return nullCount > checkLen/10
}

// replaceTemplateVars replaces all template variables with actual values from analysis
func (a *AnalyzeLLM) replaceTemplateVars(template string, analysis *llmAnalysis) string {
	result := template
	
	// Replace basic fields
	result = strings.ReplaceAll(result, "{{.Summary}}", analysis.Summary)
	result = strings.ReplaceAll(result, "{{.Issue}}", analysis.Issue)
	result = strings.ReplaceAll(result, "{{.Solution}}", analysis.Solution)
	result = strings.ReplaceAll(result, "{{.RootCause}}", analysis.RootCause)
	result = strings.ReplaceAll(result, "{{.Severity}}", analysis.Severity)
	result = strings.ReplaceAll(result, "{{.Confidence}}", fmt.Sprintf("%.0f%%", analysis.Confidence*100))
	
	// Replace array fields
	if len(analysis.Commands) > 0 {
		result = strings.ReplaceAll(result, "{{.Commands}}", strings.Join(analysis.Commands, "; "))
	}
	if len(analysis.AffectedPods) > 0 {
		result = strings.ReplaceAll(result, "{{.AffectedPods}}", strings.Join(analysis.AffectedPods, ", "))
	}
	if len(analysis.NextSteps) > 0 {
		result = strings.ReplaceAll(result, "{{.NextSteps}}", strings.Join(analysis.NextSteps, "; "))
	}
	if len(analysis.RelatedIssues) > 0 {
		result = strings.ReplaceAll(result, "{{.RelatedIssues}}", strings.Join(analysis.RelatedIssues, "; "))
	}
	
	return result
}
