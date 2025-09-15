package llm

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
)

// LLMProvider represents the type of LLM provider
type LLMProvider string

const (
	ProviderOllama LLMProvider = "ollama"
)

// LLMConfig contains common configuration for LLM agents
type LLMConfig struct {
	Provider        LLMProvider       `json:"provider"`
	Model           string            `json:"model"`
	Temperature     float64           `json:"temperature"`
	MaxTokens       int               `json:"maxTokens"`
	SystemPrompt    string            `json:"systemPrompt"`
	EnablePIIFilter bool              `json:"enablePIIFilter"`
	AuditLogging    bool              `json:"auditLogging"`
	Timeout         time.Duration     `json:"timeout"`
	CustomSettings  map[string]string `json:"customSettings"`
}

// DefaultLLMConfig returns a default LLM configuration
func DefaultLLMConfig(provider LLMProvider, model string) *LLMConfig {
	return &LLMConfig{
		Provider:        provider,
		Model:           model,
		Temperature:     0.3,
		MaxTokens:       4096,
		SystemPrompt:    GetDefaultSystemPrompt(),
		EnablePIIFilter: true,
		AuditLogging:    true,
		Timeout:         60 * time.Second,
		CustomSettings:  make(map[string]string),
	}
}

// LLMRequest represents a request to an LLM
type LLMRequest struct {
	SystemPrompt string            `json:"systemPrompt"`
	UserPrompt   string            `json:"userPrompt"`
	Context      map[string]string `json:"context"`
	Temperature  float64           `json:"temperature"`
	MaxTokens    int               `json:"maxTokens"`
}

// LLMResponse represents a response from an LLM
type LLMResponse struct {
	Content        string                 `json:"content"`
	Usage          *TokenUsage            `json:"usage"`
	Model          string                 `json:"model"`
	Metadata       map[string]interface{} `json:"metadata"`
	ProcessingTime time.Duration          `json:"processingTime"`
}

// TokenUsage tracks token consumption
type TokenUsage struct {
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`
}

// EnhancementRequest contains information for enhancing analysis results
type EnhancementRequest struct {
	OriginalResult *analyzer.AnalyzeResult   `json:"originalResult"`
	AllResults     []*analyzer.AnalyzeResult `json:"allResults"`
	BundleContext  *BundleContext            `json:"bundleContext"`
	Options        *EnhancementOptions       `json:"options"`
}

// BundleContext provides context about the support bundle
type BundleContext struct {
	ClusterVersion  string            `json:"clusterVersion"`
	NodeCount       int               `json:"nodeCount"`
	Namespaces      []string          `json:"namespaces"`
	CriticalPods    []PodInfo         `json:"criticalPods"`
	RecentEvents    []EventInfo       `json:"recentEvents"`
	ResourceSummary map[string]int    `json:"resourceSummary"`
	Errors          []string          `json:"errors"`
	Metadata        map[string]string `json:"metadata"`
}

// PodInfo represents information about a pod
type PodInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Restarts  int    `json:"restarts"`
	Age       string `json:"age"`
}

// EventInfo represents a Kubernetes event
type EventInfo struct {
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Object    string `json:"object"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// EnhancementOptions contains options for LLM enhancement
type EnhancementOptions struct {
	IncludeRemediation   bool     `json:"includeRemediation"`
	EnableCorrelation    bool     `json:"enableCorrelation"`
	DetailLevel          string   `json:"detailLevel"` // "basic", "detailed", "comprehensive"
	FocusAreas           []string `json:"focusAreas"`
	ExcludeGenericAdvice bool     `json:"excludeGenericAdvice"`
}

// GetDefaultSystemPrompt returns the default system prompt for troubleshooting
func GetDefaultSystemPrompt() string {
	return `You are an expert Kubernetes and infrastructure troubleshooting assistant. Your role is to analyze diagnostic information and provide actionable insights and remediation steps.

Key responsibilities:
1. Analyze troubleshooting data with precision and context awareness
2. Identify root causes and contributing factors
3. Provide specific, actionable remediation steps
4. Detect correlations between different issues
5. Prioritize issues by impact and urgency
6. Offer both immediate fixes and long-term improvements

Guidelines:
- Be specific and avoid generic advice
- Include exact commands and configurations when possible
- Consider security implications of recommendations
- Account for different environments (dev/staging/production)
- Provide validation steps to verify fixes
- Explain the reasoning behind recommendations
- Use appropriate Kubernetes and infrastructure terminology

Response format: Always structure responses as JSON with specific fields for confidence, impact assessment, detailed explanations, evidence, and step-by-step remediation instructions.`
}

// GetDomainSpecificPrompts returns specialized prompts for different problem domains
func GetDomainSpecificPrompts() map[string]string {
	return map[string]string{
		"kubernetes": `Focus on Kubernetes-specific issues including pod scheduling, resource limits, RBAC, networking, and API server problems. Consider cluster state, node health, and workload distribution.`,

		"storage": `Analyze storage-related issues including PVC binding, storage class configuration, volume mounting, disk space, and performance. Consider storage backends and data persistence.`,

		"network": `Examine network connectivity, DNS resolution, service mesh configuration, ingress/egress rules, and inter-pod communication. Consider security policies and load balancing.`,

		"security": `Review RBAC configurations, security contexts, network policies, admission controllers, and certificate issues. Prioritize security implications in recommendations.`,

		"performance": `Analyze resource utilization, bottlenecks, scaling issues, and optimization opportunities. Consider CPU, memory, I/O, and network performance metrics.`,

		"compliance": `Check for compliance with organizational policies, security standards, and best practices. Ensure recommendations align with governance requirements.`,
	}
}

// PIIFilter removes or masks potentially sensitive information
type PIIFilter struct {
	patterns map[string]*regexp.Regexp
	enabled  bool
}

// NewPIIFilter creates a new PII filter
func NewPIIFilter(enabled bool) *PIIFilter {
	patterns := map[string]*regexp.Regexp{
		"ip":       regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
		"email":    regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`),
		"token":    regexp.MustCompile(`\b[A-Za-z0-9+/]{40,}={0,2}\b`),
		"secret":   regexp.MustCompile(`(?i)(password|secret|key|token)[\s=:]["']?([A-Za-z0-9+/=]{8,})["']?`),
		"hostname": regexp.MustCompile(`\b[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.([a-zA-Z]{2,})\b`),
	}

	return &PIIFilter{
		patterns: patterns,
		enabled:  enabled,
	}
}

// FilterText removes or masks PII in the given text
func (p *PIIFilter) FilterText(text string) string {
	if !p.enabled {
		return text
	}

	result := text

	// Replace IP addresses
	result = p.patterns["ip"].ReplaceAllString(result, "XXX.XXX.XXX.XXX")

	// Replace email addresses
	result = p.patterns["email"].ReplaceAllString(result, "user@example.com")

	// Replace tokens/secrets
	result = p.patterns["token"].ReplaceAllString(result, "[REDACTED-TOKEN]")
	result = p.patterns["secret"].ReplaceAllStringFunc(result, func(match string) string {
		parts := p.patterns["secret"].FindStringSubmatch(match)
		if len(parts) >= 2 {
			return parts[1] + "=[REDACTED]"
		}
		return "[REDACTED]"
	})

	// Replace hostnames (keep domain structure)
	result = p.patterns["hostname"].ReplaceAllString(result, "host.example.com")

	return result
}

// PromptBuilder helps build effective prompts for LLM analysis
type PromptBuilder struct {
	systemPrompt string
	context      *BundleContext
	filter       *PIIFilter
}

// NewPromptBuilder creates a new prompt builder
func NewPromptBuilder(systemPrompt string, enablePIIFilter bool) *PromptBuilder {
	return &PromptBuilder{
		systemPrompt: systemPrompt,
		filter:       NewPIIFilter(enablePIIFilter),
	}
}

// BuildAnalysisPrompt creates a prompt for analyzing a specific result
func (pb *PromptBuilder) BuildAnalysisPrompt(request *EnhancementRequest) (string, error) {
	if request.OriginalResult == nil {
		return "", fmt.Errorf("original result cannot be nil")
	}

	// Build context section
	contextStr := pb.buildContextSection(request.BundleContext)

	// Build result analysis section
	resultStr := pb.buildResultSection(request.OriginalResult)

	// Build related results section
	relatedStr := pb.buildRelatedResultsSection(request.AllResults, request.OriginalResult)

	// Build options section
	optionsStr := pb.buildOptionsSection(request.Options)

	// Construct the full prompt
	prompt := fmt.Sprintf(`Analyze the following Kubernetes troubleshooting result and provide enhanced insights:

%s

TARGET ANALYSIS RESULT:
%s

%s

%s

Please provide a comprehensive analysis in JSON format with the following structure:
{
  "confidence": <float between 0.0 and 1.0>,
  "impact": "<HIGH|MEDIUM|LOW>",
  "explanation": "<detailed explanation of the issue>",
  "evidence": ["<specific evidence point 1>", "<evidence point 2>"],
  "problem": "<concise problem description>",
  "remediation": {
    "id": "<unique-remediation-id>",
    "title": "<remediation title>",
    "description": "<detailed remediation description>",
    "category": "<immediate|short-term|long-term>",
    "priority": <integer 1-10>,
    "commands": ["<command 1>", "<command 2>"],
    "manual": ["<manual step 1>", "<manual step 2>"],
    "links": ["<helpful link 1>"],
    "validation": {
      "description": "<how to verify the fix>",
      "commands": ["<validation command>"],
      "expected": "<expected result>"
    }
  },
  "correlations": ["<related issue 1>", "<related issue 2>"],
  "insights": [{
    "type": "<correlation|recommendation|warning>",
    "title": "<insight title>",
    "description": "<insight description>",
    "confidence": <float>
  }]
}`,
		contextStr, resultStr, relatedStr, optionsStr)

	// Filter PII if enabled
	filteredPrompt := pb.filter.FilterText(prompt)

	return filteredPrompt, nil
}

// buildContextSection creates the context section of the prompt
func (pb *PromptBuilder) buildContextSection(context *BundleContext) string {
	if context == nil {
		return "CLUSTER CONTEXT: No additional context available."
	}

	var parts []string
	parts = append(parts, "CLUSTER CONTEXT:")

	if context.ClusterVersion != "" {
		parts = append(parts, fmt.Sprintf("- Kubernetes Version: %s", context.ClusterVersion))
	}
	if context.NodeCount > 0 {
		parts = append(parts, fmt.Sprintf("- Node Count: %d", context.NodeCount))
	}
	if len(context.Namespaces) > 0 {
		parts = append(parts, fmt.Sprintf("- Namespaces: %s", strings.Join(context.Namespaces, ", ")))
	}
	if len(context.CriticalPods) > 0 {
		parts = append(parts, "- Critical Pods with Issues:")
		for _, pod := range context.CriticalPods {
			parts = append(parts, fmt.Sprintf("  * %s/%s: %s (restarts: %d)", pod.Namespace, pod.Name, pod.Status, pod.Restarts))
		}
	}
	if len(context.RecentEvents) > 0 {
		parts = append(parts, "- Recent Events:")
		for _, event := range context.RecentEvents {
			parts = append(parts, fmt.Sprintf("  * %s %s: %s", event.Type, event.Reason, event.Message))
		}
	}
	if len(context.Errors) > 0 {
		parts = append(parts, fmt.Sprintf("- System Errors: %s", strings.Join(context.Errors, ", ")))
	}

	return strings.Join(parts, "\n")
}

// buildResultSection creates the result analysis section
func (pb *PromptBuilder) buildResultSection(result *analyzer.AnalyzeResult) string {
	status := "UNKNOWN"
	if result.IsPass {
		status = "PASS"
	} else if result.IsFail {
		status = "FAIL"
	} else if result.IsWarn {
		status = "WARN"
	}

	parts := []string{
		fmt.Sprintf("Title: %s", result.Title),
		fmt.Sprintf("Status: %s", status),
		fmt.Sprintf("Message: %s", result.Message),
	}

	if result.URI != "" {
		parts = append(parts, fmt.Sprintf("URI: %s", result.URI))
	}

	return strings.Join(parts, "\n")
}

// buildRelatedResultsSection creates the related results section
func (pb *PromptBuilder) buildRelatedResultsSection(allResults []*analyzer.AnalyzeResult, target *analyzer.AnalyzeResult) string {
	if len(allResults) <= 1 {
		return ""
	}

	var relatedResults []*analyzer.AnalyzeResult
	for _, result := range allResults {
		if result != target && result != nil {
			// Simple heuristic for finding related results
			if pb.areResultsRelated(result, target) {
				relatedResults = append(relatedResults, result)
			}
		}
	}

	if len(relatedResults) == 0 {
		return ""
	}

	parts := []string{"RELATED RESULTS:"}
	for _, result := range relatedResults {
		status := "UNKNOWN"
		if result.IsPass {
			status = "PASS"
		} else if result.IsFail {
			status = "FAIL"
		} else if result.IsWarn {
			status = "WARN"
		}
		parts = append(parts, fmt.Sprintf("- %s: %s (%s)", result.Title, result.Message, status))
	}

	return strings.Join(parts, "\n")
}

// buildOptionsSection creates the options section
func (pb *PromptBuilder) buildOptionsSection(options *EnhancementOptions) string {
	if options == nil {
		return "ANALYSIS OPTIONS: Use default analysis depth and include all standard recommendations."
	}

	parts := []string{"ANALYSIS OPTIONS:"}
	parts = append(parts, fmt.Sprintf("- Detail Level: %s", options.DetailLevel))
	parts = append(parts, fmt.Sprintf("- Include Remediation: %t", options.IncludeRemediation))
	parts = append(parts, fmt.Sprintf("- Enable Correlation: %t", options.EnableCorrelation))

	if len(options.FocusAreas) > 0 {
		parts = append(parts, fmt.Sprintf("- Focus Areas: %s", strings.Join(options.FocusAreas, ", ")))
	}

	return strings.Join(parts, "\n")
}

// areResultsRelated determines if two results are related (simplified heuristic)
func (pb *PromptBuilder) areResultsRelated(result1, result2 *analyzer.AnalyzeResult) bool {
	// Simple keyword-based relation detection
	keywords1 := strings.Fields(strings.ToLower(result1.Title + " " + result1.Message))
	keywords2 := strings.Fields(strings.ToLower(result2.Title + " " + result2.Message))

	// Filter out common words
	commonWords := map[string]bool{
		"check": true, "status": true, "the": true, "and": true, "is": true, "are": true,
		"to": true, "of": true, "in": true, "for": true, "with": true, "at": true,
	}

	for _, k1 := range keywords1 {
		if len(k1) <= 3 || commonWords[k1] {
			continue
		}
		for _, k2 := range keywords2 {
			if len(k2) <= 3 || commonWords[k2] {
				continue
			}
			if k1 == k2 {
				return true
			}
		}
	}

	return false
}

// AuditLog represents an audit log entry for LLM usage
type AuditLog struct {
	Timestamp   time.Time              `json:"timestamp"`
	AgentName   string                 `json:"agentName"`
	Provider    string                 `json:"provider"`
	Model       string                 `json:"model"`
	RequestID   string                 `json:"requestId"`
	InputSize   int                    `json:"inputSize"`
	OutputSize  int                    `json:"outputSize"`
	TokenUsage  *TokenUsage            `json:"tokenUsage"`
	Duration    time.Duration          `json:"duration"`
	PIIFiltered bool                   `json:"piiFiltered"`
	Success     bool                   `json:"success"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// AuditLogger handles logging of LLM operations for compliance
type AuditLogger struct {
	enabled bool
	logs    []AuditLog
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(enabled bool) *AuditLogger {
	return &AuditLogger{
		enabled: enabled,
		logs:    make([]AuditLog, 0),
	}
}

// LogRequest logs an LLM request for audit purposes
func (al *AuditLogger) LogRequest(log AuditLog) {
	if !al.enabled {
		return
	}
	log.Timestamp = time.Now()
	al.logs = append(al.logs, log)
}

// GetLogs returns all audit logs
func (al *AuditLogger) GetLogs() []AuditLog {
	return al.logs
}

// ParseLLMResponse attempts to parse and validate an LLM response
func ParseLLMResponse(content string) (*LLMAnalysisResult, error) {
	// Try to extract JSON from the response
	content = strings.TrimSpace(content)

	// Find JSON block if wrapped in markdown
	if strings.Contains(content, "```json") {
		start := strings.Index(content, "```json") + 7
		end := strings.Index(content[start:], "```")
		if end > 0 {
			content = content[start : start+end]
		}
	}

	var result LLMAnalysisResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// Validate required fields
	if result.Confidence < 0 || result.Confidence > 1 {
		result.Confidence = 0.5 // Default if invalid
	}

	if result.Impact == "" {
		result.Impact = "MEDIUM"
	}

	return &result, nil
}

// LLMAnalysisResult represents the structured result from LLM analysis
type LLMAnalysisResult struct {
	Confidence   float64                    `json:"confidence"`
	Impact       string                     `json:"impact"`
	Explanation  string                     `json:"explanation"`
	Evidence     []string                   `json:"evidence"`
	Problem      string                     `json:"problem"`
	Remediation  *analyzer.RemediationStep  `json:"remediation"`
	Correlations []string                   `json:"correlations"`
	Insights     []analyzer.AnalysisInsight `json:"insights"`
}
