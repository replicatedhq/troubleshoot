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
	"text/template"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

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

	// Get problem description from analyzer config, then environment as fallback
	problemDescription := a.analyzer.ProblemDescription
	if problemDescription == "" {
		// Check for CLI-provided problem description via environment
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
	Model          string          `json:"model"`
	Messages       []openAIMessage `json:"messages"`
	Timeout        int             `json:"timeout,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type       string      `json:"type"`
	JSONSchema *jsonSchema `json:"json_schema,omitempty"`
}

type jsonSchema struct {
	Name   string                 `json:"name"`
	Strict bool                   `json:"strict"`
	Schema map[string]interface{} `json:"schema"`
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

// buildAnalysisSchema returns the JSON schema for structured outputs
func buildAnalysisSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"issue_found": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether an issue was identified",
			},
			"summary": map[string]interface{}{
				"type":        "string",
				"description": "Brief summary of findings",
			},
			"issue": map[string]interface{}{
				"type":        "string",
				"description": "Detailed description of the issue if found",
			},
			"solution": map[string]interface{}{
				"type":        "string",
				"description": "Recommended solution if an issue was found",
			},
			"severity": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"critical", "warning", "info"},
				"description": "Severity level of the issue",
			},
			"confidence": map[string]interface{}{
				"type":        "number",
				"minimum":     0.0,
				"maximum":     1.0,
				"description": "Confidence level from 0.0 to 1.0",
			},
			"commands": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "kubectl commands that could help resolve the issue",
			},
			"documentation": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "Relevant Kubernetes documentation URLs",
			},
			"root_cause": map[string]interface{}{
				"type":        "string",
				"description": "The identified root cause of the problem",
			},
			"affected_pods": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "List of affected pod names",
			},
			"next_steps": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "Ordered list of recommended actions",
			},
			"related_issues": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "Other potential problems found",
			},
		},
		"required": []string{
			"issue_found",
			"summary",
			"severity",
			"confidence",
		},
		"additionalProperties": false,
	}
}

func (a *AnalyzeLLM) callLLM(apiKey, problemDescription string, files map[string]string) (*llmAnalysis, error) {
	// Build prompt
	var promptBuilder strings.Builder
	fmt.Fprintf(&promptBuilder, `You are analyzing a Kubernetes support bundle to identify issues.

Problem Description: %s

Please analyze the following files and provide a comprehensive analysis.

Files to analyze:

`, problemDescription)

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

	// Check if we should use structured outputs (default true for compatible models)
	// Structured outputs are supported by gpt-4o, gpt-4o-mini, and newer models
	// but NOT by gpt-3.5-turbo or older completion models
	useStructuredOutput := true
	if a.analyzer.UseStructuredOutput == false {
		useStructuredOutput = false
	}
	// Only disable for models we know don't support structured outputs
	// This way, future models (like gpt-5, gpt-4-turbo-2024, etc.) will default to using it
	if strings.Contains(strings.ToLower(model), "gpt-3.5") || 
	   strings.Contains(strings.ToLower(model), "davinci") || 
	   strings.Contains(strings.ToLower(model), "curie") || 
	   strings.Contains(strings.ToLower(model), "babbage") || 
	   strings.Contains(strings.ToLower(model), "ada") {
		useStructuredOutput = false
	}

	// Create request
	reqBody := openAIRequest{
		Model: model,
		Messages: []openAIMessage{
			{
				Role:    "system",
				Content: "You are a Kubernetes troubleshooting expert. Analyze support bundles and identify issues.",
			},
			{
				Role:    "user",
				Content: promptBuilder.String(),
			},
		},
	}

	// Add structured output format if supported
	if useStructuredOutput {
		reqBody.ResponseFormat = &responseFormat{
			Type: "json_schema",
			JSONSchema: &jsonSchema{
				Name:   "kubernetes_analysis",
				Strict: true,
				Schema: buildAnalysisSchema(),
			},
		}
		// Mention JSON requirement in system message for clarity
		reqBody.Messages[0].Content += " Respond with a JSON object following the provided schema."
	} else {
		// Fall back to prompting for JSON
		reqBody.Messages[0].Content += " Always respond with valid JSON containing: issue_found, summary, issue, solution, severity, confidence, commands, documentation, root_cause, affected_pods, next_steps, related_issues."
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal request")
	}

	// Determine API endpoint (allow override for testing/proxies)
	apiEndpoint := "https://api.openai.com/v1/chat/completions"
	if a.analyzer.APIEndpoint != "" {
		apiEndpoint = a.analyzer.APIEndpoint
	}

	// Create HTTP request with timeout (120s for large analyses)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", apiEndpoint, bytes.NewBuffer(jsonData))
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

	// With structured outputs, JSON should always be valid
	if useStructuredOutput {
		if err := json.Unmarshal([]byte(content), &analysis); err != nil {
			// This should rarely happen with structured outputs enabled
			return nil, errors.Wrapf(err, "failed to parse structured JSON response: %s", content)
		}
	} else {
		// Without structured outputs, try to extract JSON and handle failures gracefully
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

// isBinaryFile uses http.DetectContentType to determine if content is binary
func isBinaryFile(content []byte) bool {
	if len(content) == 0 {
		return false
	}
	
	// Use http.DetectContentType for proper MIME type detection
	// It examines up to the first 512 bytes
	contentType := http.DetectContentType(content)
	
	// Check if it's a text type
	if strings.HasPrefix(contentType, "text/") {
		return false
	}
	
	// Check for specific text-based formats that might be misidentified
	textTypes := []string{
		"application/json",
		"application/xml",
		"application/yaml",
		"application/x-yaml",
		"application/javascript",
	}
	
	for _, textType := range textTypes {
		if strings.HasPrefix(contentType, textType) {
			return false
		}
	}
	
	// If it's application/octet-stream, do additional checking
	if contentType == "application/octet-stream" {
		// Check for null bytes as a fallback heuristic
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
		// This threshold is kept for backward compatibility
		return nullCount > checkLen/10
	}
	
	// Assume binary for all other content types
	return true
}

// templateData provides data for template rendering with helper methods
type templateData struct {
	*llmAnalysis
}

// ConfidencePercent returns confidence as a percentage string
func (t templateData) ConfidencePercent() string {
	return fmt.Sprintf("%.0f%%", t.Confidence*100)
}

// CommandsList returns commands as a semicolon-separated string
func (t templateData) CommandsList() string {
	return strings.Join(t.Commands, "; ")
}

// AffectedPodsList returns affected pods as a comma-separated string
func (t templateData) AffectedPodsList() string {
	return strings.Join(t.AffectedPods, ", ")
}

// NextStepsList returns next steps as a semicolon-separated string
func (t templateData) NextStepsList() string {
	return strings.Join(t.NextSteps, "; ")
}

// RelatedIssuesList returns related issues as a semicolon-separated string
func (t templateData) RelatedIssuesList() string {
	return strings.Join(t.RelatedIssues, "; ")
}

// replaceTemplateVars uses Go's text/template for safe template rendering
func (a *AnalyzeLLM) replaceTemplateVars(templateStr string, analysis *llmAnalysis) string {
	// Create template with custom functions
	tmpl, err := template.New("outcome").Funcs(template.FuncMap{
		"join": strings.Join,
	}).Parse(templateStr)
	if err != nil {
		// If template parsing fails, return the original string
		// Log error for debugging
		fmt.Fprintf(os.Stderr, "Warning: Failed to parse template: %v\n", err)
		return templateStr
	}
	
	// Prepare template data
	data := templateData{llmAnalysis: analysis}
	
	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		// If template execution fails, return the original string
		fmt.Fprintf(os.Stderr, "Warning: Failed to execute template: %v\n", err)
		return templateStr
	}
	
	return buf.String()
}
