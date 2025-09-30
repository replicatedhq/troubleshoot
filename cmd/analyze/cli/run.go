package cli

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/internal/util"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/analyze/agents/local"
	"github.com/replicatedhq/troubleshoot/pkg/analyze/agents/ollama"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

func runAnalyzers(v *viper.Viper, bundlePath string) error {
	// Handle Ollama-specific commands first (these don't require a bundle)
	if v.GetBool("setup-ollama") {
		return handleOllamaSetup(v)
	}

	if v.GetBool("check-ollama") {
		return handleOllamaStatus(v)
	}

	if v.GetBool("list-models") {
		return handleListModels(v)
	}

	if v.GetBool("pull-model") {
		return handlePullModel(v)
	}

	// For all other operations, we need a bundle path
	if bundlePath == "" {
		return errors.New("bundle path is required for analysis operations")
	}

	// Check if advanced analysis is requested
	useAdvanced := v.GetBool("advanced-analysis") ||
		v.GetBool("enable-ollama") ||
		(len(v.GetStringSlice("agents")) > 1 ||
			(len(v.GetStringSlice("agents")) == 1 && v.GetStringSlice("agents")[0] != "local"))

	if useAdvanced {
		return runAdvancedAnalysis(v, bundlePath)
	}

	// Only fall back to legacy analysis if no advanced flags are used at all
	return runLegacyAnalysis(v, bundlePath)
}

// handleOllamaSetup automatically sets up Ollama for the user
func handleOllamaSetup(v *viper.Viper) error {
	fmt.Println("🚀 Ollama Setup Assistant")
	fmt.Println("=" + strings.Repeat("=", 50))

	helper := analyzer.NewOllamaHelper()

	// Check current status
	status := helper.GetHealthStatus()
	fmt.Print(status.String())

	if !status.Installed {
		fmt.Println("\n🔧 Installing Ollama...")
		if err := helper.DownloadAndInstall(); err != nil {
			return errors.Wrap(err, "failed to install Ollama")
		}
		fmt.Println("✅ Ollama installed successfully!")
	}

	if !status.Running {
		fmt.Println("\n🚀 Starting Ollama service...")
		if err := helper.StartService(); err != nil {
			return errors.Wrap(err, "failed to start Ollama service")
		}
		fmt.Println("✅ Ollama service started!")
	}

	if len(status.Models) == 0 {
		fmt.Println("\n📚 Downloading recommended model...")
		helper.PrintModelRecommendations()

		model := v.GetString("ollama-model")
		if model == "" {
			model = "llama2:7b"
		}

		fmt.Printf("\n⬇️  Pulling model: %s (this may take several minutes)...\n", model)
		if err := helper.PullModel(model); err != nil {
			return errors.Wrapf(err, "failed to pull model %s", model)
		}
	}

	fmt.Println("\n🎉 Ollama setup complete!")
	fmt.Println("\n💡 Next steps:")
	fmt.Printf("   troubleshoot analyze --enable-ollama %s\n", filepath.Base(os.Args[len(os.Args)-1]))

	return nil
}

// handleOllamaStatus shows current Ollama installation and service status
func handleOllamaStatus(v *viper.Viper) error {
	helper := analyzer.NewOllamaHelper()
	status := helper.GetHealthStatus()

	fmt.Println("🔍 Ollama Status Report")
	fmt.Println("=" + strings.Repeat("=", 50))
	fmt.Print(status.String())

	if !status.Installed {
		fmt.Println("\n🔧 Setup Instructions:")
		fmt.Println(helper.GetInstallInstructions())
		return nil
	}

	if !status.Running {
		fmt.Println("\n🚀 To start Ollama service:")
		fmt.Println("   ollama serve &")
		fmt.Println("   # or")
		fmt.Println("   troubleshoot analyze --setup-ollama")
		return nil
	}

	if len(status.Models) == 0 {
		fmt.Println("\n📚 No models installed. Recommended models:")
		helper.PrintModelRecommendations()
	} else {
		fmt.Println("\n✅ Ready for AI-powered analysis!")
		fmt.Printf("   troubleshoot analyze --enable-ollama your-bundle.tar.gz\n")
	}

	return nil
}

// handleListModels lists available and installed Ollama models
func handleListModels(v *viper.Viper) error {
	helper := analyzer.NewOllamaHelper()
	status := helper.GetHealthStatus()

	fmt.Println("🤖 Ollama Model Management")
	fmt.Println("=" + strings.Repeat("=", 50))

	if !status.Installed {
		fmt.Println("❌ Ollama is not installed")
		fmt.Println("💡 Install with: troubleshoot analyze --setup-ollama")
		return nil
	}

	if !status.Running {
		fmt.Println("⚠️  Ollama service is not running")
		fmt.Println("🚀 Start with: ollama serve &")
		return nil
	}

	// Show installed models
	fmt.Println("📚 Installed Models:")
	if len(status.Models) == 0 {
		fmt.Println("   No models installed")
	} else {
		for _, model := range status.Models {
			fmt.Printf("   ✅ %s\n", model)
		}
	}

	// Show available models for download
	fmt.Println("\n🌐 Available Models:")
	helper.PrintModelRecommendations()

	// Show usage examples
	fmt.Println("💡 Usage Examples:")
	fmt.Println("   # Use specific model:")
	fmt.Printf("   troubleshoot analyze --ollama-model llama2:13b bundle.tar.gz\n")
	fmt.Println("   # Use preset models:")
	fmt.Printf("   troubleshoot analyze --use-codellama bundle.tar.gz\n")
	fmt.Printf("   troubleshoot analyze --use-mistral bundle.tar.gz\n")
	fmt.Println("   # Pull a new model:")
	fmt.Printf("   troubleshoot analyze --ollama-model llama2:13b --pull-model\n")

	return nil
}

// handlePullModel pulls a specific model
func handlePullModel(v *viper.Viper) error {
	helper := analyzer.NewOllamaHelper()
	status := helper.GetHealthStatus()

	if !status.Installed {
		fmt.Println("❌ Ollama is not installed")
		fmt.Println("💡 Install with: troubleshoot analyze --setup-ollama")
		return errors.New("Ollama must be installed to pull models")
	}

	if !status.Running {
		fmt.Println("❌ Ollama service is not running")
		fmt.Println("🚀 Start with: ollama serve &")
		return errors.New("Ollama service must be running to pull models")
	}

	// Determine which model to pull
	model := determineOllamaModel(v)

	fmt.Printf("📥 Pulling model: %s\n", model)
	fmt.Println("=" + strings.Repeat("=", 50))

	if err := helper.PullModel(model); err != nil {
		return errors.Wrapf(err, "failed to pull model %s", model)
	}

	fmt.Printf("\n✅ Model %s ready for analysis!\n", model)
	fmt.Println("\n💡 Test it with:")
	fmt.Printf("   troubleshoot analyze --ollama-model %s bundle.tar.gz\n", model)

	return nil
}

// runAdvancedAnalysis uses the new analysis engine with agent support
func runAdvancedAnalysis(v *viper.Viper, bundlePath string) error {
	ctx := context.Background()

	// Create the analysis engine
	engine := analyzer.NewAnalysisEngine()

	// Determine which agents to use
	agents := v.GetStringSlice("agents")

	// Handle Ollama flags
	enableOllama := v.GetBool("enable-ollama")
	disableOllama := v.GetBool("disable-ollama")

	if enableOllama && !disableOllama {
		// Add ollama to agents if not already present
		hasOllama := false
		for _, agent := range agents {
			if agent == "ollama" {
				hasOllama = true
				break
			}
		}
		if !hasOllama {
			agents = append(agents, "ollama")
		}
	}

	if disableOllama {
		// Remove ollama from agents
		filteredAgents := []string{}
		for _, agent := range agents {
			if agent != "ollama" {
				filteredAgents = append(filteredAgents, agent)
			}
		}
		agents = filteredAgents
	}

	// Register requested agents
	registeredAgents := []string{}
	for _, agentName := range agents {
		switch agentName {
		case "ollama":
			if err := registerOllamaAgent(engine, v); err != nil {
				return err
			}
			registeredAgents = append(registeredAgents, agentName)

		case "local":
			opts := &local.LocalAgentOptions{}
			agent := local.NewLocalAgent(opts)
			if err := engine.RegisterAgent("local", agent); err != nil {
				return errors.Wrap(err, "failed to register local agent")
			}
			registeredAgents = append(registeredAgents, agentName)

		default:
			klog.Warningf("Unknown agent type: %s", agentName)
		}
	}

	if len(registeredAgents) == 0 {
		return errors.New("no analysis agents available - check your configuration")
	}

	fmt.Printf("🔍 Using analysis agents: %s\n", strings.Join(registeredAgents, ", "))

	// Load support bundle
	bundle, err := loadSupportBundle(bundlePath)
	if err != nil {
		return errors.Wrap(err, "failed to load support bundle")
	}

	// Load analyzer specs if provided
	var customAnalyzers []*troubleshootv1beta2.Analyze
	if specPath := v.GetString("analyzers"); specPath != "" {
		customAnalyzers, err = loadAnalyzerSpecs(specPath)
		if err != nil {
			return errors.Wrap(err, "failed to load analyzer specs")
		}
	}

	// Configure analysis options
	opts := analyzer.AnalysisOptions{
		Agents:             registeredAgents,
		IncludeRemediation: v.GetBool("include-remediation"),
		CustomAnalyzers:    customAnalyzers,
		Timeout:            5 * time.Minute,
		Concurrency:        2,
	}

	// Run analysis
	fmt.Printf("🚀 Starting advanced analysis of bundle: %s\n", bundlePath)
	result, err := engine.Analyze(ctx, bundle, opts)
	if err != nil {
		return errors.Wrap(err, "analysis failed")
	}

	// Display results
	return displayAdvancedResults(result, v.GetString("output"), v.GetString("output-file"))
}

// registerOllamaAgent creates and registers an Ollama agent
func registerOllamaAgent(engine analyzer.AnalysisEngine, v *viper.Viper) error {
	// Check if Ollama is available
	helper := analyzer.NewOllamaHelper()
	status := helper.GetHealthStatus()

	if !status.Installed {
		return showOllamaSetupHelp("Ollama is not installed")
	}

	if !status.Running {
		return showOllamaSetupHelp("Ollama service is not running")
	}

	if len(status.Models) == 0 {
		return showOllamaSetupHelp("No Ollama models are installed")
	}

	// Determine which model to use
	selectedModel := determineOllamaModel(v)

	// Auto-pull model if requested and not available
	if v.GetBool("auto-pull-model") {
		if err := ensureModelAvailable(selectedModel); err != nil {
			return errors.Wrapf(err, "failed to ensure model %s is available", selectedModel)
		}
	}

	// Create Ollama agent
	opts := &ollama.OllamaAgentOptions{
		Endpoint:    v.GetString("ollama-endpoint"),
		Model:       selectedModel,
		Timeout:     5 * time.Minute,
		MaxTokens:   2000,
		Temperature: 0.2,
	}

	agent, err := ollama.NewOllamaAgent(opts)
	if err != nil {
		return errors.Wrap(err, "failed to create Ollama agent")
	}

	// Register with engine
	if err := engine.RegisterAgent("ollama", agent); err != nil {
		return errors.Wrap(err, "failed to register Ollama agent")
	}

	return nil
}

// showOllamaSetupHelp displays helpful setup instructions when Ollama is not available
func showOllamaSetupHelp(reason string) error {
	fmt.Printf("❌ Ollama AI analysis not available: %s\n\n", reason)

	helper := analyzer.NewOllamaHelper()
	fmt.Println("🔧 Quick Setup:")
	fmt.Println("   troubleshoot analyze --setup-ollama")
	fmt.Println()
	fmt.Println("📋 Manual Setup:")
	fmt.Println("   1. Install: curl -fsSL https://ollama.ai/install.sh | sh")
	fmt.Println("   2. Start service: ollama serve &")
	fmt.Println("   3. Pull model: ollama pull llama2:7b")
	fmt.Println("   4. Retry analysis with: --enable-ollama")
	fmt.Println()
	fmt.Println("💡 Check status: troubleshoot analyze --check-ollama")
	fmt.Println()
	fmt.Println(helper.GetInstallInstructions())

	return errors.New("Ollama setup required for AI-powered analysis")
}

// runLegacyAnalysis runs the original analysis logic for backward compatibility
func runLegacyAnalysis(v *viper.Viper, bundlePath string) error {
	specPath := v.GetString("analyzers")

	specContent := ""
	var err error
	if _, err = os.Stat(specPath); err == nil {
		b, err := os.ReadFile(specPath)
		if err != nil {
			return err
		}

		specContent = string(b)
	} else {
		if !util.IsURL(specPath) {
			// TODO: Better error message when we do not have a file/url etc
			return fmt.Errorf("%s is not a URL and was not found", specPath)
		}

		req, err := http.NewRequest("GET", specPath, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "Replicated_Analyzer/v1beta1")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		specContent = string(body)
	}

	analyzeResults, err := analyzer.DownloadAndAnalyze(bundlePath, specContent)
	if err != nil {
		return errors.Wrap(err, "failed to download and analyze bundle")
	}

	for _, analyzeResult := range analyzeResults {
		if analyzeResult.IsPass {
			fmt.Printf("Pass: %s\n %s\n", analyzeResult.Title, analyzeResult.Message)
		} else if analyzeResult.IsWarn {
			fmt.Printf("Warn: %s\n %s\n", analyzeResult.Title, analyzeResult.Message)
		} else if analyzeResult.IsFail {
			fmt.Printf("Fail: %s\n %s\n", analyzeResult.Title, analyzeResult.Message)
		}
	}

	return nil
}

// loadSupportBundle loads and parses a support bundle from file
func loadSupportBundle(bundlePath string) (*analyzer.SupportBundle, error) {
	if _, err := os.Stat(bundlePath); os.IsNotExist(err) {
		return nil, errors.Errorf("support bundle not found: %s", bundlePath)
	}

	klog.Infof("Loading support bundle: %s", bundlePath)

	// Open the tar.gz file
	file, err := os.Open(bundlePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open support bundle")
	}
	defer file.Close()

	// Create gzip reader
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create gzip reader")
	}
	defer gzipReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzipReader)

	// Create bundle structure
	bundle := &analyzer.SupportBundle{
		Files: make(map[string][]byte),
		Metadata: &analyzer.SupportBundleMetadata{
			CreatedAt:   time.Now(),
			Version:     "1.0.0",
			GeneratedBy: "troubleshoot-cli",
		},
	}

	// Extract all files from tar
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.Wrap(err, "failed to read tar entry")
		}

		// Skip directories
		if header.Typeflag == tar.TypeDir {
			continue
		}

		// Read file content
		content, err := io.ReadAll(tarReader)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read file %s", header.Name)
		}

		// Remove bundle directory prefix from file path for consistent access
		// e.g., "live-cluster-bundle/cluster-info/version.json" → "cluster-info/version.json"
		cleanPath := header.Name
		if parts := strings.SplitN(header.Name, "/", 2); len(parts) == 2 {
			cleanPath = parts[1]
		}

		bundle.Files[cleanPath] = content
		klog.V(2).Infof("Loaded file: %s (%d bytes)", cleanPath, len(content))
	}

	klog.Infof("Successfully loaded support bundle with %d files", len(bundle.Files))

	return bundle, nil
}

// loadAnalyzerSpecs loads analyzer specifications from file or URL
func loadAnalyzerSpecs(specPath string) ([]*troubleshootv1beta2.Analyze, error) {
	klog.Infof("Loading analyzer specs from: %s", specPath)

	// Read the analyzer spec file (same logic as runLegacyAnalysis)
	specContent := ""
	var err error
	if _, err = os.Stat(specPath); err == nil {
		b, err := os.ReadFile(specPath)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read analyzer spec file")
		}
		specContent = string(b)
	} else {
		if !util.IsURL(specPath) {
			return nil, errors.Errorf("analyzer spec %s is not a URL and was not found", specPath)
		}

		req, err := http.NewRequest("GET", specPath, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create HTTP request")
		}
		req.Header.Set("User-Agent", "Replicated_Analyzer/v1beta2")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, errors.Wrap(err, "failed to fetch analyzer spec")
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read analyzer spec response")
		}
		specContent = string(body)
	}

	// Parse the YAML/JSON into troubleshoot analyzer struct
	var analyzerSpec troubleshootv1beta2.Analyzer
	if err := yaml.Unmarshal([]byte(specContent), &analyzerSpec); err != nil {
		return nil, errors.Wrap(err, "failed to parse analyzer spec")
	}

	// Return the analyzer specs from the parsed document
	return analyzerSpec.Spec.Analyzers, nil
}

// displayAdvancedResults formats and displays analysis results
func displayAdvancedResults(result *analyzer.AnalysisResult, outputFormat, outputFile string) error {
	if result == nil {
		return errors.New("no analysis results to display")
	}

	// Display summary
	fmt.Println("\n📊 Analysis Summary")
	fmt.Println("=" + strings.Repeat("=", 50))
	fmt.Printf("Total Analyzers: %d\n", result.Summary.TotalAnalyzers)
	fmt.Printf("✅ Pass: %d\n", result.Summary.PassCount)
	fmt.Printf("⚠️  Warn: %d\n", result.Summary.WarnCount)
	fmt.Printf("❌ Fail: %d\n", result.Summary.FailCount)
	fmt.Printf("🚫 Errors: %d\n", result.Summary.ErrorCount)
	fmt.Printf("⏱️  Duration: %s\n", result.Summary.Duration)
	fmt.Printf("🤖 Agents Used: %s\n", strings.Join(result.Summary.AgentsUsed, ", "))

	if result.Summary.Confidence > 0 {
		fmt.Printf("🎯 Confidence: %.1f%%\n", result.Summary.Confidence*100)
	}

	// Display results based on format
	switch outputFormat {
	case "json":
		jsonData, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return errors.Wrap(err, "failed to marshal results to JSON")
		}
		fmt.Println("\n📄 Full Results (JSON):")
		fmt.Println(string(jsonData))

	default:
		// Human-readable format
		fmt.Println("\n🔍 Analysis Results")
		fmt.Println("=" + strings.Repeat("=", 50))

		for _, analyzerResult := range result.Results {
			status := "❓"
			if analyzerResult.IsPass {
				status = "✅"
			} else if analyzerResult.IsWarn {
				status = "⚠️"
			} else if analyzerResult.IsFail {
				status = "❌"
			}

			fmt.Printf("\n%s %s", status, analyzerResult.Title)
			if analyzerResult.AgentName != "" {
				fmt.Printf(" [%s]", analyzerResult.AgentName)
			}
			if analyzerResult.Confidence > 0 {
				fmt.Printf(" (%.0f%% confidence)", analyzerResult.Confidence*100)
			}
			fmt.Println()

			if analyzerResult.Message != "" {
				fmt.Printf("   %s\n", analyzerResult.Message)
			}

			if analyzerResult.Category != "" {
				fmt.Printf("   Category: %s\n", analyzerResult.Category)
			}

			// Display insights if available
			if len(analyzerResult.Insights) > 0 {
				fmt.Println("   💡 Insights:")
				for _, insight := range analyzerResult.Insights {
					fmt.Printf("   • %s\n", insight)
				}
			}

			// Display remediation if available
			if analyzerResult.Remediation != nil {
				fmt.Printf("   🔧 Remediation: %s\n", analyzerResult.Remediation.Description)
				if analyzerResult.Remediation.Command != "" {
					fmt.Printf("   💻 Command: %s\n", analyzerResult.Remediation.Command)
				}
			}
		}

		// Display overall remediation suggestions
		if len(result.Remediation) > 0 {
			fmt.Println("\n🔧 Recommended Actions")
			fmt.Println("=" + strings.Repeat("=", 50))
			for i, remedy := range result.Remediation {
				fmt.Printf("%d. %s\n", i+1, remedy.Description)
				if remedy.Command != "" {
					fmt.Printf("   Command: %s\n", remedy.Command)
				}
				if remedy.Documentation != "" {
					fmt.Printf("   Docs: %s\n", remedy.Documentation)
				}
			}
		}

		// Display errors if any
		if len(result.Errors) > 0 {
			fmt.Println("\n⚠️  Errors During Analysis")
			fmt.Println("=" + strings.Repeat("=", 30))
			for _, analysisError := range result.Errors {
				fmt.Printf("• [%s] %s: %s\n", analysisError.Agent, analysisError.Category, analysisError.Error)
			}
		}

		// Display agent metadata
		if len(result.Metadata.Agents) > 0 {
			fmt.Println("\n🤖 Agent Performance")
			fmt.Println("=" + strings.Repeat("=", 40))
			for _, agent := range result.Metadata.Agents {
				fmt.Printf("• %s: %d results, %s duration", agent.Name, agent.ResultCount, agent.Duration)
				if agent.ErrorCount > 0 {
					fmt.Printf(" (%d errors)", agent.ErrorCount)
				}
				fmt.Println()
			}
		}
	}

	// Save results to file if requested
	if outputFile != "" {
		jsonData, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return errors.Wrap(err, "failed to marshal results for file output")
		}

		if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
			return errors.Wrapf(err, "failed to write results to %s", outputFile)
		}

		fmt.Printf("\n💾 Analysis results saved to: %s\n", outputFile)
	}

	return nil
}

// determineOllamaModel selects the appropriate model based on flags
func determineOllamaModel(v *viper.Viper) string {
	// Check for specific model flags first
	if v.GetBool("use-codellama") {
		return "codellama:7b"
	}
	if v.GetBool("use-mistral") {
		return "mistral:7b"
	}

	// Fall back to explicit model specification or default
	return v.GetString("ollama-model")
}

// ensureModelAvailable checks if model exists and pulls it if needed
func ensureModelAvailable(model string) error {
	// Check if model is already available
	cmd := exec.Command("ollama", "list")
	output, err := cmd.Output()
	if err != nil {
		return errors.Wrap(err, "failed to check available models")
	}

	// Parse model list to see if our model exists
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, model) {
			klog.Infof("Model %s is already available", model)
			return nil
		}
	}

	// Model not found, pull it
	fmt.Printf("📚 Model %s not found, pulling automatically...\n", model)
	pullCmd := exec.Command("ollama", "pull", model)
	pullCmd.Stdout = os.Stdout
	pullCmd.Stderr = os.Stderr

	if err := pullCmd.Run(); err != nil {
		return errors.Wrapf(err, "failed to pull model %s", model)
	}

	fmt.Printf("✅ Model %s pulled successfully!\n", model)
	return nil
}
