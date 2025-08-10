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
	"strings"
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

	// Get problem description from context (will be added in Phase 4)
	// For now, use a default
	problemDescription := os.Getenv("PROBLEM_DESCRIPTION")
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
	maxSize := 500 * 1024 // 500KB
	maxFiles := 10
	if a.analyzer.MaxFiles > 0 {
		maxFiles = a.analyzer.MaxFiles
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

			for _, filePath := range matchingFiles {
				if len(files) >= maxFiles {
					break
				}

				content, err := getFile(string(filePath))
				if err != nil {
					continue
				}

				if totalSize+len(content) > maxSize {
					break
				}

				files[string(filePath)] = string(content)
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
	IssueFound  bool   `json:"issue_found"`
	Summary     string `json:"summary"`
	Issue       string `json:"issue"`
	Solution    string `json:"solution"`
	Severity    string `json:"severity"` // "critical", "warning", "info"
	Confidence  float64 `json:"confidence"`
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
	promptBuilder.WriteString("- confidence (number): confidence level from 0.0 to 1.0\n\n")
	promptBuilder.WriteString("Files to analyze:\n\n")

	for path, content := range files {
		promptBuilder.WriteString(fmt.Sprintf("=== %s ===\n", path))
		// Truncate very long files
		if len(content) > 10000 {
			content = content[:10000] + "\n... (truncated)"
		}
		promptBuilder.WriteString(content)
		promptBuilder.WriteString("\n\n")
	}

	// Determine model
	model := "gpt-5"
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

	// Create HTTP request with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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

	// Build message with template variables
	message := analysis.Summary
	if analysis.Issue != "" {
		message = fmt.Sprintf("%s\n\nIssue: %s", message, analysis.Issue)
	}
	if analysis.Solution != "" {
		message = fmt.Sprintf("%s\n\nSolution: %s", message, analysis.Solution)
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
						result.Message = strings.ReplaceAll(outcome.Fail.Message, "{{.Summary}}", analysis.Summary)
					}
					break
				}
			} else if outcome.Warn != nil && analysis.IssueFound && analysis.Severity == "warning" {
				if outcome.Warn.When == "" || outcome.Warn.When == "potential_issue" {
					result.IsFail = false
					result.IsWarn = true
					result.IsPass = false
					if outcome.Warn.Message != "" {
						result.Message = strings.ReplaceAll(outcome.Warn.Message, "{{.Summary}}", analysis.Summary)
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