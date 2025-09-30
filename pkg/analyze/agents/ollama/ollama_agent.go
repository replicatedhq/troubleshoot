package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"k8s.io/klog/v2"
)

// OllamaAgent implements the Agent interface for self-hosted LLM analysis via Ollama
type OllamaAgent struct {
	name         string
	endpoint     string
	model        string
	client       *http.Client
	capabilities []string
	enabled      bool
	version      string
	maxTokens    int
	temperature  float32
	timeout      time.Duration
}

// OllamaAgentOptions configures the Ollama agent
type OllamaAgentOptions struct {
	Endpoint    string        // Ollama server endpoint (default: http://localhost:11434)
	Model       string        // Model name (e.g., "codellama:13b", "llama2:7b")
	Timeout     time.Duration // Request timeout
	MaxTokens   int           // Maximum tokens in response
	Temperature float32       // Response creativity (0.0 to 1.0)
}

// OllamaRequest represents a request to the Ollama API
type OllamaRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Stream  bool                   `json:"stream"`
	Options map[string]interface{} `json:"options,omitempty"`
	Context []int                  `json:"context,omitempty"`
}

// OllamaResponse represents a response from the Ollama API
type OllamaResponse struct {
	Model              string `json:"model"`
	CreatedAt          string `json:"created_at"`
	Response           string `json:"response"`
	Done               bool   `json:"done"`
	Context            []int  `json:"context,omitempty"`
	TotalDuration      int64  `json:"total_duration,omitempty"`
	LoadDuration       int64  `json:"load_duration,omitempty"`
	PromptEvalCount    int    `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64  `json:"prompt_eval_duration,omitempty"`
	EvalCount          int    `json:"eval_count,omitempty"`
	EvalDuration       int64  `json:"eval_duration,omitempty"`
}

// OllamaModelInfo represents model information from Ollama
type OllamaModelInfo struct {
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	Digest     string    `json:"digest"`
	ModifiedAt time.Time `json:"modified_at"`
}

// OllamaModelsResponse represents the response from the models endpoint
type OllamaModelsResponse struct {
	Models []OllamaModelInfo `json:"models"`
}

// AnalysisPrompt represents different types of analysis prompts
type AnalysisPrompt struct {
	Type        string
	Template    string
	MaxTokens   int
	Temperature float32
}

// Predefined analysis prompts for different scenarios
var analysisPrompts = map[string]AnalysisPrompt{
	"pod-analysis": {
		Type: "pod-analysis",
		Template: `You are a Kubernetes expert analyzing pod data. Analyze the following pod information and provide insights:

Pod Data:
%s

Please analyze this data and provide:
1. Overall health status
2. Any issues or concerns identified
3. Specific recommendations for improvement
4. Remediation steps if problems are found

Respond in JSON format:
{
  "status": "pass|warn|fail",
  "title": "Brief title",
  "message": "Detailed analysis message",
  "insights": ["insight1", "insight2"],
  "remediation": {
    "description": "What to do",
    "action": "action-type", 
    "command": "command to run",
    "priority": 1-10
  }
}`,
		MaxTokens:   1000,
		Temperature: 0.2,
	},
	"deployment-analysis": {
		Type: "deployment-analysis",
		Template: `You are a Kubernetes expert analyzing deployment data. Analyze the following deployment information:

Deployment Data:
%s

Please analyze and provide:
1. Deployment health and readiness
2. Scaling and resource issues
3. Configuration problems
4. Actionable recommendations

Respond in JSON format with status, title, message, insights, and remediation.`,
		MaxTokens:   1000,
		Temperature: 0.2,
	},
	"log-analysis": {
		Type: "log-analysis",
		Template: `You are a system administrator analyzing application logs. Analyze the following log content:

Log Content (last 50 lines):
%s

Please analyze and provide:
1. Error patterns and frequency
2. Warning patterns that need attention  
3. Performance indicators
4. Security concerns
5. Recommendations for investigation

Respond in JSON format with status, title, message, insights, and remediation.`,
		MaxTokens:   1200,
		Temperature: 0.3,
	},
	"event-analysis": {
		Type: "event-analysis",
		Template: `You are a Kubernetes expert analyzing cluster events. Analyze the following events:

Events Data:
%s

Please analyze and provide:
1. Critical events requiring immediate attention
2. Warning patterns and their implications
3. Resource constraint indicators
4. Networking or scheduling issues
5. Prioritized remediation steps

Respond in JSON format with status, title, message, insights, and remediation.`,
		MaxTokens:   1200,
		Temperature: 0.2,
	},
	"resource-analysis": {
		Type: "resource-analysis",
		Template: `You are a Kubernetes expert analyzing node and resource data. Analyze the following resource information:

Resource Data:
%s

Please analyze and provide:
1. Resource utilization and capacity planning
2. Node health and availability issues
3. Performance bottlenecks
4. Scaling recommendations
5. Resource optimization suggestions

Respond in JSON format with status, title, message, insights, and remediation.`,
		MaxTokens:   1100,
		Temperature: 0.2,
	},
	"general-analysis": {
		Type: "general-analysis",
		Template: `You are a Kubernetes and infrastructure expert. Analyze the following data and provide insights:

Data:
%s

Context: %s

Please provide:
1. Overall assessment
2. Key issues identified
3. Impact analysis
4. Detailed recommendations
5. Next steps

Respond in JSON format with status, title, message, insights, and remediation.`,
		MaxTokens:   1000,
		Temperature: 0.3,
	},
}

// NewOllamaAgent creates a new Ollama-powered analysis agent
func NewOllamaAgent(opts *OllamaAgentOptions) (*OllamaAgent, error) {
	if opts == nil {
		opts = &OllamaAgentOptions{}
	}

	// Set defaults
	if opts.Endpoint == "" {
		opts.Endpoint = "http://localhost:11434"
	}
	if opts.Model == "" {
		opts.Model = "llama2:7b"
	}
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Minute
	}
	if opts.MaxTokens == 0 {
		opts.MaxTokens = 2000
	}
	if opts.Temperature == 0 {
		opts.Temperature = 0.2
	}

	// Validate endpoint
	_, err := url.Parse(opts.Endpoint)
	if err != nil {
		return nil, errors.Wrap(err, "invalid Ollama endpoint URL")
	}

	agent := &OllamaAgent{
		name:     "ollama",
		endpoint: strings.TrimSuffix(opts.Endpoint, "/"),
		model:    opts.Model,
		client: &http.Client{
			Timeout: opts.Timeout,
		},
		capabilities: []string{
			"ai-powered-analysis",
			"natural-language-insights",
			"context-aware-remediation",
			"intelligent-correlation",
			"multi-modal-analysis",
			"self-hosted-llm",
			"privacy-preserving",
		},
		enabled:     true,
		version:     "1.0.0",
		maxTokens:   opts.MaxTokens,
		temperature: opts.Temperature,
		timeout:     opts.Timeout,
	}

	return agent, nil
}

// Name returns the agent name
func (a *OllamaAgent) Name() string {
	return a.name
}

// IsAvailable checks if Ollama is available and the model is loaded
func (a *OllamaAgent) IsAvailable() bool {
	if !a.enabled {
		return false
	}

	// Quick health check
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return a.HealthCheck(ctx) == nil
}

// Capabilities returns the agent's capabilities
func (a *OllamaAgent) Capabilities() []string {
	return append([]string{}, a.capabilities...)
}

// HealthCheck verifies Ollama is accessible and the model is available
func (a *OllamaAgent) HealthCheck(ctx context.Context) error {
	ctx, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, "OllamaAgent.HealthCheck")
	defer span.End()

	if !a.enabled {
		return errors.New("Ollama agent is disabled")
	}

	// Check if Ollama server is running
	healthURL := fmt.Sprintf("%s/api/tags", a.endpoint)
	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		span.SetStatus(codes.Error, "failed to create health check request")
		return errors.Wrap(err, "failed to create health check request")
	}

	resp, err := a.client.Do(req)
	if err != nil {
		span.SetStatus(codes.Error, "Ollama server not accessible")
		return errors.Wrap(err, "Ollama server not accessible")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		span.SetStatus(codes.Error, fmt.Sprintf("Ollama server returned status %d", resp.StatusCode))
		return errors.Errorf("Ollama server returned status %d", resp.StatusCode)
	}

	// Parse models response to check if our model is available
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read models response")
	}

	var modelsResp OllamaModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return errors.Wrap(err, "failed to parse models response")
	}

	// Check if our model is available
	modelFound := false
	for _, model := range modelsResp.Models {
		if model.Name == a.model {
			modelFound = true
			break
		}
	}

	if !modelFound {
		span.SetStatus(codes.Error, fmt.Sprintf("model %s not found", a.model))
		return errors.Errorf("model %s not found in Ollama", a.model)
	}

	span.SetAttributes(
		attribute.String("model", a.model),
		attribute.String("endpoint", a.endpoint),
		attribute.Int("available_models", len(modelsResp.Models)),
	)

	return nil
}

// Analyze performs AI-powered analysis using Ollama
func (a *OllamaAgent) Analyze(ctx context.Context, data []byte, analyzers []analyzer.AnalyzerSpec) (*analyzer.AgentResult, error) {
	startTime := time.Now()

	ctx, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, "OllamaAgent.Analyze")
	defer span.End()

	if !a.enabled {
		return nil, errors.New("Ollama agent is not enabled")
	}

	// Parse the bundle data
	bundle := &analyzer.SupportBundle{}
	if err := json.Unmarshal(data, bundle); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal bundle data")
	}

	results := &analyzer.AgentResult{
		Results: make([]*analyzer.AnalyzerResult, 0),
		Metadata: analyzer.AgentResultMetadata{
			AnalyzerCount: len(analyzers),
			Version:       a.version,
		},
		Errors: make([]string, 0),
	}

	// If no specific analyzers, discover from bundle content
	if len(analyzers) == 0 {
		analyzers = a.discoverAnalyzers(bundle)
	}

	// Process each analyzer with LLM
	for _, analyzerSpec := range analyzers {
		result, err := a.runLLMAnalysis(ctx, bundle, analyzerSpec)
		if err != nil {
			klog.Errorf("Failed to run LLM analysis for %s: %v", analyzerSpec.Name, err)
			results.Errors = append(results.Errors, fmt.Sprintf("LLM analysis %s failed: %v", analyzerSpec.Name, err))
			continue
		}

		if result != nil {
			// Enhance result with AI agent metadata
			result.AgentName = a.name
			result.AnalyzerType = analyzerSpec.Type
			result.Category = analyzerSpec.Category
			result.Confidence = a.calculateConfidence(result.Message)

			results.Results = append(results.Results, result)
		}
	}

	results.Metadata.Duration = time.Since(startTime)

	span.SetAttributes(
		attribute.Int("total_analyzers", len(analyzers)),
		attribute.Int("successful_results", len(results.Results)),
		attribute.Int("errors", len(results.Errors)),
		attribute.String("model", a.model),
	)

	return results, nil
}

// discoverAnalyzers automatically discovers analyzers based on bundle content
func (a *OllamaAgent) discoverAnalyzers(bundle *analyzer.SupportBundle) []analyzer.AnalyzerSpec {
	var specs []analyzer.AnalyzerSpec

	// Analyze bundle contents to determine what types of analysis to perform
	for filePath := range bundle.Files {
		filePath = strings.ToLower(filePath)

		switch {
		case strings.Contains(filePath, "pods") && strings.HasSuffix(filePath, ".json"):
			specs = append(specs, analyzer.AnalyzerSpec{
				Name:     "ai-pod-analysis",
				Type:     "ai-workload",
				Category: "pods",
				Priority: 10,
				Config:   map[string]interface{}{"filePath": filePath, "promptType": "pod-analysis"},
			})

		case strings.Contains(filePath, "deployments") && strings.HasSuffix(filePath, ".json"):
			specs = append(specs, analyzer.AnalyzerSpec{
				Name:     "ai-deployment-analysis",
				Type:     "ai-workload",
				Category: "deployments",
				Priority: 9,
				Config:   map[string]interface{}{"filePath": filePath, "promptType": "deployment-analysis"},
			})

		case strings.Contains(filePath, "events") && strings.HasSuffix(filePath, ".json"):
			specs = append(specs, analyzer.AnalyzerSpec{
				Name:     "ai-event-analysis",
				Type:     "ai-events",
				Category: "events",
				Priority: 8,
				Config:   map[string]interface{}{"filePath": filePath, "promptType": "event-analysis"},
			})

		case strings.Contains(filePath, "logs") && strings.HasSuffix(filePath, ".log"):
			specs = append(specs, analyzer.AnalyzerSpec{
				Name:     "ai-log-analysis",
				Type:     "ai-logs",
				Category: "logging",
				Priority: 7,
				Config:   map[string]interface{}{"filePath": filePath, "promptType": "log-analysis"},
			})

		case strings.Contains(filePath, "nodes") && strings.HasSuffix(filePath, ".json"):
			specs = append(specs, analyzer.AnalyzerSpec{
				Name:     "ai-resource-analysis",
				Type:     "ai-resources",
				Category: "nodes",
				Priority: 8,
				Config:   map[string]interface{}{"filePath": filePath, "promptType": "resource-analysis"},
			})
		}
	}

	return specs
}

// runLLMAnalysis executes analysis using LLM for a specific analyzer spec
func (a *OllamaAgent) runLLMAnalysis(ctx context.Context, bundle *analyzer.SupportBundle, spec analyzer.AnalyzerSpec) (*analyzer.AnalyzerResult, error) {
	ctx, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, fmt.Sprintf("OllamaAgent.%s", spec.Name))
	defer span.End()

	// Smart file detection for enhanced analyzer compatibility
	var filePath string
	var fileData []byte
	var exists bool

	// First try to get explicit filePath from config
	if fp, ok := spec.Config["filePath"].(string); ok {
		filePath = fp
		fileData, exists = bundle.Files[filePath]
	}

	// If no explicit filePath, auto-detect based on analyzer type
	if !exists {
		filePath, fileData, exists = a.autoDetectFileForAnalyzer(bundle, spec)
	}

	if !exists {
		result := &analyzer.AnalyzerResult{
			Title:    spec.Name,
			IsWarn:   true,
			Message:  fmt.Sprintf("File not found: %s", filePath),
			Category: spec.Category,
		}
		return result, nil
	}

	promptType, _ := spec.Config["promptType"].(string)
	if promptType == "" {
		promptType = "general-analysis"
	}

	// Get appropriate prompt template
	prompt, exists := analysisPrompts[promptType]
	if !exists {
		prompt = analysisPrompts["general-analysis"]
	}

	// Prepare data for analysis (truncate if too large)
	dataStr := string(fileData)
	if len(dataStr) > 4000 { // Limit input size
		if promptType == "log-analysis" {
			// For logs, take the last N lines
			lines := strings.Split(dataStr, "\n")
			if len(lines) > 50 {
				lines = lines[len(lines)-50:]
			}
			dataStr = strings.Join(lines, "\n")
		} else {
			// For other data, truncate from beginning
			dataStr = dataStr[:4000] + "\n... (truncated)"
		}
	}

	// Format the prompt
	var formattedPrompt string
	if promptType == "general-analysis" {
		formattedPrompt = fmt.Sprintf(prompt.Template, dataStr, spec.Category)
	} else {
		formattedPrompt = fmt.Sprintf(prompt.Template, dataStr)
	}

	// Query Ollama
	response, err := a.queryOllama(ctx, formattedPrompt, prompt)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query Ollama for %s", spec.Name)
	}

	// Parse LLM response into AnalyzerResult
	result, err := a.parseLLMResponse(response, spec)
	if err != nil {
		klog.Warningf("Failed to parse LLM response for %s, using fallback: %v", spec.Name, err)
		// Fallback result
		result = &analyzer.AnalyzerResult{
			Title:    spec.Name,
			IsWarn:   true,
			Message:  fmt.Sprintf("AI analysis completed but response format was unexpected. Raw response: %s", response),
			Category: spec.Category,
			Insights: []string{"LLM analysis provided insights but in unexpected format"},
		}
	}

	return result, nil
}

// queryOllama sends a query to the Ollama API
func (a *OllamaAgent) queryOllama(ctx context.Context, prompt string, promptConfig AnalysisPrompt) (string, error) {
	request := OllamaRequest{
		Model:  a.model,
		Prompt: prompt,
		Stream: false,
		Options: map[string]interface{}{
			"num_predict":    promptConfig.MaxTokens,
			"temperature":    promptConfig.Temperature,
			"top_p":          0.9,
			"top_k":          40,
			"repeat_penalty": 1.1,
		},
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal Ollama request")
	}

	generateURL := fmt.Sprintf("%s/api/generate", a.endpoint)
	req, err := http.NewRequestWithContext(ctx, "POST", generateURL, bytes.NewReader(requestBody))
	if err != nil {
		return "", errors.Wrap(err, "failed to create Ollama request")
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "Ollama request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", errors.Errorf("Ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read Ollama response")
	}

	var response OllamaResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", errors.Wrap(err, "failed to parse Ollama response")
	}

	return response.Response, nil
}

// autoDetectFileForAnalyzer intelligently finds the appropriate file for each analyzer type
func (a *OllamaAgent) autoDetectFileForAnalyzer(bundle *analyzer.SupportBundle, spec analyzer.AnalyzerSpec) (string, []byte, bool) {
	switch spec.Name {
	case "cluster-version":
		// ClusterVersion analyzers expect cluster-info/cluster_version.json
		if data, exists := bundle.Files["cluster-info/cluster_version.json"]; exists {
			return "cluster-info/cluster_version.json", data, true
		}

	case "node-resources", "node-resources-check":
		// NodeResources analyzers expect cluster-resources/nodes.json
		if data, exists := bundle.Files["cluster-resources/nodes.json"]; exists {
			return "cluster-resources/nodes.json", data, true
		}

	case "text-analyze":
		// TextAnalyze analyzers - find log files based on traditional analyzer config
		if traditionalAnalyzer, ok := spec.Config["analyzer"]; ok {
			if textAnalyzer, ok := traditionalAnalyzer.(*troubleshootv1beta2.TextAnalyze); ok {
				// Construct file path from CollectorName and FileName
				var targetPath string
				if textAnalyzer.CollectorName != "" {
					targetPath = fmt.Sprintf("%s/%s", textAnalyzer.CollectorName, textAnalyzer.FileName)
				} else {
					targetPath = textAnalyzer.FileName
				}

				if data, exists := bundle.Files[targetPath]; exists {
					return targetPath, data, true
				}

				// Try to find log files automatically
				for path, data := range bundle.Files {
					if strings.HasSuffix(path, ".log") && strings.Contains(path, textAnalyzer.FileName) {
						return path, data, true
					}
				}
			}
		}

	case "postgres", "mysql", "redis", "mssql":
		// Database analyzers - find connection files
		if traditionalAnalyzer, ok := spec.Config["analyzer"]; ok {
			if dbAnalyzer, ok := traditionalAnalyzer.(*troubleshootv1beta2.DatabaseAnalyze); ok {
				if dbAnalyzer.FileName != "" {
					if data, exists := bundle.Files[dbAnalyzer.FileName]; exists {
						return dbAnalyzer.FileName, data, true
					}
				}

				// Auto-detect database files
				for path, data := range bundle.Files {
					if strings.Contains(path, spec.Name) && strings.HasSuffix(path, ".json") {
						return path, data, true
					}
				}
			}
		}

	case "deployment-status":
		// Deployment analyzers - find deployment files based on namespace
		if traditionalAnalyzer, ok := spec.Config["analyzer"]; ok {
			if deploymentAnalyzer, ok := traditionalAnalyzer.(*troubleshootv1beta2.DeploymentStatus); ok {
				deploymentPath := fmt.Sprintf("cluster-resources/deployments/%s.json", deploymentAnalyzer.Namespace)
				if data, exists := bundle.Files[deploymentPath]; exists {
					return deploymentPath, data, true
				}
			}
		}

	case "event", "event-analysis":
		// Event analyzers expect cluster-resources/events.json
		if data, exists := bundle.Files["cluster-resources/events.json"]; exists {
			return "cluster-resources/events.json", data, true
		}

	case "configmap":
		// ConfigMap analyzers - find configmap files based on namespace
		if traditionalAnalyzer, ok := spec.Config["analyzer"]; ok {
			if configMapAnalyzer, ok := traditionalAnalyzer.(*troubleshootv1beta2.AnalyzeConfigMap); ok {
				configMapPath := fmt.Sprintf("cluster-resources/configmaps/%s.json", configMapAnalyzer.Namespace)
				if data, exists := bundle.Files[configMapPath]; exists {
					return configMapPath, data, true
				}
			}
		}

	case "secret":
		// Secret analyzers - find secret files based on namespace
		if traditionalAnalyzer, ok := spec.Config["analyzer"]; ok {
			if secretAnalyzer, ok := traditionalAnalyzer.(*troubleshootv1beta2.AnalyzeSecret); ok {
				secretPath := fmt.Sprintf("cluster-resources/secrets/%s.json", secretAnalyzer.Namespace)
				if data, exists := bundle.Files[secretPath]; exists {
					return secretPath, data, true
				}
			}
		}

	case "crd", "customResourceDefinition":
		// CRD analyzers - look for custom resource files
		if traditionalAnalyzer, ok := spec.Config["analyzer"]; ok {
			if crdAnalyzer, ok := traditionalAnalyzer.(*troubleshootv1beta2.CustomResourceDefinition); ok {
				// Look for specific CRD name in custom-resources directory
				crdName := crdAnalyzer.CustomResourceDefinitionName
				for path, data := range bundle.Files {
					if strings.Contains(path, "custom-resources") &&
						(strings.Contains(strings.ToLower(path), strings.ToLower(crdName)) ||
							strings.Contains(strings.ToLower(path), "crd")) {
						return path, data, true
					}
				}
			}
		}

	case "container-runtime":
		// Container runtime analyzers - look for node information
		if data, exists := bundle.Files["cluster-resources/nodes.json"]; exists {
			return "cluster-resources/nodes.json", data, true
		}

	case "distribution":
		// Distribution analyzers - primarily use node information
		if data, exists := bundle.Files["cluster-resources/nodes.json"]; exists {
			return "cluster-resources/nodes.json", data, true
		}
		// Also check cluster info as backup
		if data, exists := bundle.Files["cluster-info/cluster_version.json"]; exists {
			return "cluster-info/cluster_version.json", data, true
		}

	case "storage-class":
		// Storage class analyzers - look for storage class resources
		for path, data := range bundle.Files {
			if strings.Contains(path, "storage") && strings.HasSuffix(path, ".json") {
				return path, data, true
			}
		}

	case "ingress":
		// Ingress analyzers - look for ingress resources
		for path, data := range bundle.Files {
			if strings.Contains(path, "ingress") && strings.HasSuffix(path, ".json") {
				return path, data, true
			}
		}

	case "http":
		// HTTP analyzers can work with any network-related data
		for path, data := range bundle.Files {
			if strings.Contains(path, "services") || strings.Contains(path, "ingress") {
				return path, data, true
			}
		}

	case "job-status":
		// Job analyzers - look for job resources
		for path, data := range bundle.Files {
			if strings.Contains(path, "jobs") && strings.HasSuffix(path, ".json") {
				return path, data, true
			}
		}

	case "statefulset-status":
		// StatefulSet analyzers
		for path, data := range bundle.Files {
			if strings.Contains(path, "statefulsets") && strings.HasSuffix(path, ".json") {
				return path, data, true
			}
		}

	case "replicaset-status":
		// ReplicaSet analyzers
		for path, data := range bundle.Files {
			if strings.Contains(path, "replicasets") && strings.HasSuffix(path, ".json") {
				return path, data, true
			}
		}

	case "cluster-pod-statuses":
		// Pod status analyzers
		for path, data := range bundle.Files {
			if strings.Contains(path, "pods") && strings.HasSuffix(path, ".json") {
				return path, data, true
			}
		}

	case "image-pull-secret":
		// Image pull secret analyzers
		for path, data := range bundle.Files {
			if strings.Contains(path, "secrets") && strings.HasSuffix(path, ".json") {
				return path, data, true
			}
		}

	case "yaml-compare", "json-compare":
		// Comparison analyzers - can work with any structured data
		for path, data := range bundle.Files {
			if strings.HasSuffix(path, ".json") || strings.HasSuffix(path, ".yaml") {
				return path, data, true
			}
		}

	case "certificates":
		// Certificate analyzers
		for path, data := range bundle.Files {
			if strings.Contains(path, "cert") || strings.Contains(path, "tls") {
				return path, data, true
			}
		}

	case "velero", "longhorn", "ceph-status":
		// Storage system analyzers
		for path, data := range bundle.Files {
			if strings.Contains(strings.ToLower(path), spec.Name) {
				return path, data, true
			}
		}

	case "sysctl", "goldpinger", "weave-report", "registry-images":
		// Infrastructure analyzers
		for path, data := range bundle.Files {
			if strings.Contains(strings.ToLower(path), strings.ToLower(spec.Name)) {
				return path, data, true
			}
		}

	case "cluster-resource":
		// Generic cluster resource analyzer - can work with any cluster data
		if data, exists := bundle.Files["cluster-resources/nodes.json"]; exists {
			return "cluster-resources/nodes.json", data, true
		}
		// Fallback to any cluster resource
		for path, data := range bundle.Files {
			if strings.Contains(path, "cluster-resources") && strings.HasSuffix(path, ".json") {
				return path, data, true
			}
		}
	}

	// Fallback: try to find any relevant file for this analyzer type
	for path, data := range bundle.Files {
		if strings.Contains(strings.ToLower(path), spec.Type) || strings.Contains(strings.ToLower(path), spec.Name) {
			return path, data, true
		}
	}

	return "", nil, false
}

// parseLLMResponse parses the LLM response into an AnalyzerResult
func (a *OllamaAgent) parseLLMResponse(response string, spec analyzer.AnalyzerSpec) (*analyzer.AnalyzerResult, error) {
	// First try JSON parsing
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")

	if jsonStart != -1 && jsonEnd != -1 && jsonEnd > jsonStart {
		jsonStr := response[jsonStart : jsonEnd+1]

		var llmResult struct {
			Status      string   `json:"status"`
			Title       string   `json:"title"`
			Message     string   `json:"message"`
			Insights    []string `json:"insights"`
			Remediation struct {
				Description string `json:"description"`
				Action      string `json:"action"`
				Command     string `json:"command"`
				Priority    int    `json:"priority"`
			} `json:"remediation"`
		}

		if err := json.Unmarshal([]byte(jsonStr), &llmResult); err == nil {
			// Successfully parsed JSON
			result := &analyzer.AnalyzerResult{
				Title:    llmResult.Title,
				Message:  llmResult.Message,
				Category: spec.Category,
				Insights: llmResult.Insights,
			}

			switch strings.ToLower(llmResult.Status) {
			case "pass":
				result.IsPass = true
			case "warn":
				result.IsWarn = true
			case "fail":
				result.IsFail = true
			default:
				result.IsWarn = true
			}

			if llmResult.Remediation.Description != "" {
				result.Remediation = &analyzer.RemediationStep{
					Description:   llmResult.Remediation.Description,
					Action:        llmResult.Remediation.Action,
					Command:       llmResult.Remediation.Command,
					Priority:      llmResult.Remediation.Priority,
					Category:      "ai-suggested",
					IsAutomatable: false,
				}
			}

			return result, nil
		} else {
			// JSON was found but malformed
			return nil, errors.Wrap(err, "failed to parse LLM JSON response")
		}
	}

	// Fall back to markdown parsing when JSON fails
	return a.parseMarkdownResponse(response, spec)
}

// parseMarkdownResponse handles markdown-formatted LLM responses
func (a *OllamaAgent) parseMarkdownResponse(response string, spec analyzer.AnalyzerSpec) (*analyzer.AnalyzerResult, error) {
	lines := strings.Split(response, "\n")

	result := &analyzer.AnalyzerResult{
		Title:    fmt.Sprintf("AI Analysis: %s", spec.Name),
		Category: spec.Category,
		Insights: []string{},
	}

	var title, message string
	var insights []string
	var recommendations []string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Extract title
		if strings.HasPrefix(line, "**Title:**") || strings.HasPrefix(line, "Title:") {
			title = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "**Title:**"), "Title:"))
		}

		// Extract message/assessment
		if strings.HasPrefix(line, "**Message:**") || strings.HasPrefix(line, "Message:") {
			message = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "**Message:**"), "Message:"))
		}

		// Extract insights (numbered or bulleted lists)
		if strings.Contains(line, ". ") && (strings.Contains(strings.ToLower(line), "issue") ||
			strings.Contains(strings.ToLower(line), "problem") ||
			strings.Contains(strings.ToLower(line), "warning") ||
			strings.Contains(strings.ToLower(line), "outdated") ||
			strings.Contains(strings.ToLower(line), "inconsistent")) {
			insight := strings.TrimSpace(line)
			if len(insight) > 10 { // Only add substantial insights
				insights = append(insights, insight)
			}
		}

		// Extract recommendations
		if strings.Contains(strings.ToLower(line), "recommend") ||
			strings.Contains(strings.ToLower(line), "upgrade") ||
			strings.Contains(strings.ToLower(line), "update") ||
			strings.Contains(strings.ToLower(line), "ensure") {
			recommendation := strings.TrimSpace(line)
			if len(recommendation) > 15 {
				recommendations = append(recommendations, recommendation)
			}
		}
	}

	// Build result
	if title != "" {
		result.Title = title
	}

	if message != "" {
		result.Message = message
	} else {
		// Create summary from insights
		if len(insights) > 0 {
			result.Message = fmt.Sprintf("AI analysis identified %d potential issues or observations", len(insights))
		} else {
			result.Message = "AI analysis completed successfully"
		}
	}

	result.Insights = insights

	// Determine status based on content
	if strings.Contains(strings.ToLower(response), "critical") ||
		strings.Contains(strings.ToLower(response), "error") ||
		strings.Contains(strings.ToLower(response), "fail") {
		result.IsFail = true
	} else if len(insights) > 0 || strings.Contains(strings.ToLower(response), "warn") {
		result.IsWarn = true
	} else {
		result.IsPass = true
	}

	// Add remediation from recommendations
	if len(recommendations) > 0 {
		result.Remediation = &analyzer.RemediationStep{
			Description:   strings.Join(recommendations[:1], ". "), // Use first recommendation
			Category:      "ai-suggested",
			Priority:      5,
			IsAutomatable: false,
		}
	}

	// Check if we found any meaningful content to parse
	if title == "" && message == "" && len(insights) == 0 && len(recommendations) == 0 {
		// If nothing meaningful was found, return an error
		if !strings.Contains(response, "**") && !strings.Contains(response, "Title:") &&
			!strings.Contains(response, "Message:") && !strings.Contains(response, "{") {
			return nil, errors.New("no valid JSON found in LLM response and no parseable markdown content")
		}
	}

	return result, nil
}

// calculateConfidence estimates confidence based on response characteristics
func (a *OllamaAgent) calculateConfidence(message string) float64 {
	// Simple heuristic based on response characteristics
	baseConfidence := 0.7 // Base confidence for AI analysis

	// Increase confidence for detailed responses
	if len(message) > 200 {
		baseConfidence += 0.1
	}

	// Increase confidence if specific technical terms are used
	technicalTerms := []string{"kubernetes", "pod", "deployment", "container", "node", "cluster"}
	termCount := 0
	lowerMessage := strings.ToLower(message)
	for _, term := range technicalTerms {
		if strings.Contains(lowerMessage, term) {
			termCount++
		}
	}

	if termCount >= 2 {
		baseConfidence += 0.1
	}

	// Cap at 0.95 since AI analysis is never 100% certain
	if baseConfidence > 0.95 {
		baseConfidence = 0.95
	}

	return baseConfidence
}

// SetEnabled enables or disables the Ollama agent
func (a *OllamaAgent) SetEnabled(enabled bool) {
	a.enabled = enabled
}

// UpdateModel changes the model used for analysis
func (a *OllamaAgent) UpdateModel(model string) error {
	if model == "" {
		return errors.New("model cannot be empty")
	}
	a.model = model
	return nil
}

// GetModel returns the current model name
func (a *OllamaAgent) GetModel() string {
	return a.model
}

// GetEndpoint returns the current Ollama endpoint
func (a *OllamaAgent) GetEndpoint() string {
	return a.endpoint
}
