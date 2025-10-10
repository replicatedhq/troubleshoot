package analyzer

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"
)

// OllamaHelper provides utilities for downloading and managing Ollama
type OllamaHelper struct {
	downloadURL   string
	installPath   string
	checkInterval time.Duration
}

// NewOllamaHelper creates a new Ollama helper with platform-specific defaults
func NewOllamaHelper() *OllamaHelper {
	return &OllamaHelper{
		downloadURL:   getOllamaDownloadURL(),
		installPath:   getOllamaInstallPath(),
		checkInterval: 30 * time.Second,
	}
}

// IsInstalled checks if Ollama is already installed and available
func (h *OllamaHelper) IsInstalled() bool {
	_, err := exec.LookPath("ollama")
	return err == nil
}

// IsRunning checks if Ollama service is currently running
func (h *OllamaHelper) IsRunning() bool {
	cmd := exec.Command("ollama", "ps")
	err := cmd.Run()
	return err == nil
}

// GetInstallInstructions returns platform-specific installation instructions
func (h *OllamaHelper) GetInstallInstructions() string {
	instructions := `
To use Ollama for advanced AI-powered analysis, you need to install Ollama:

🔧 Installation Options:

1. **Automatic Download** (recommended):
   Run: troubleshoot analyze --setup-ollama

2. **Manual Installation**:
`

	switch runtime.GOOS {
	case "darwin":
		instructions += `   • Visit: https://ollama.ai/download
   • Download and install Ollama for macOS
   • Or use Homebrew: brew install ollama`

	case "linux":
		instructions += `   • Run: curl -fsSL https://ollama.ai/install.sh | sh
   • Or download from: https://ollama.ai/download`

	case "windows":
		instructions += `   • Visit: https://ollama.ai/download
   • Download and install Ollama for Windows`

	default:
		instructions += `   • Visit: https://ollama.ai/download
   • Download the appropriate version for your platform`
	}

	instructions += `

3. **Docker** (alternative):
   docker run -d -v ollama:/root/.ollama -p 11434:11434 --name ollama ollama/ollama

📋 After installation:
   1. Start Ollama: ollama serve
   2. Pull a model: ollama pull llama2:7b
   3. Run analysis: troubleshoot analyze --enable-ollama bundle.tar.gz

🔍 Verify installation: ollama --version
`

	return instructions
}

// GetSetupCommand returns the command to start Ollama service
func (h *OllamaHelper) GetSetupCommand() string {
	return `# Start Ollama service in background
ollama serve &

# Pull recommended model for troubleshooting
ollama pull llama2:7b

# Verify it's working
ollama ps`
}

// DownloadAndInstall automatically downloads and installs Ollama
func (h *OllamaHelper) DownloadAndInstall() error {
	if h.IsInstalled() {
		return errors.New("Ollama is already installed")
	}

	klog.Info("Downloading Ollama...")

	switch runtime.GOOS {
	case "darwin":
		// For macOS, try Homebrew first, fall back to direct download
		return h.installMacOS()
	case "linux":
		// For Linux, use the official install script
		klog.Info("Running official Ollama install script...")
		cmd := exec.Command("sh", "-c", "curl -fsSL https://ollama.com/install.sh | sh")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return errors.Wrap(err, "installation script failed")
		}

	case "windows":
		// For Windows, download and run the installer
		return h.downloadAndInstallWindows()

	default:
		return errors.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	klog.Info("Ollama installed successfully!")
	return nil
}

// downloadAndInstallWindows handles Windows-specific installation
func (h *OllamaHelper) downloadAndInstallWindows() error {
	// Create temporary file
	tmpFile, err := os.CreateTemp("", "ollama-installer-*.exe")
	if err != nil {
		return errors.Wrap(err, "failed to create temporary file")
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Download installer
	resp, err := http.Get(h.downloadURL)
	if err != nil {
		return errors.Wrap(err, "failed to download Ollama installer")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Write to temporary file
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to write installer")
	}

	// Close the file before executing it (required on Windows)
	tmpFile.Close()

	// Run installer
	klog.Info("Running Ollama installer...")
	cmd := exec.Command(tmpFile.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "installation failed")
	}

	return nil
}

// installMacOS handles macOS-specific installation using Homebrew
func (h *OllamaHelper) installMacOS() error {
	// Check if Homebrew is available
	if _, err := exec.LookPath("brew"); err != nil {
		return errors.New("Homebrew is required for automatic installation on macOS. Please install Homebrew first or install Ollama manually from https://ollama.com/download")
	}

	klog.Info("Installing Ollama via Homebrew...")
	cmd := exec.Command("brew", "install", "ollama")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "Homebrew installation failed")
	}

	return nil
}

// StartService starts the Ollama service
func (h *OllamaHelper) StartService() error {
	if !h.IsInstalled() {
		return errors.New("Ollama is not installed")
	}

	if h.IsRunning() {
		klog.Info("Ollama service is already running")
		return nil
	}

	klog.Info("Starting Ollama service...")

	// Start ollama serve in background
	cmd := exec.Command("ollama", "serve")

	// Start in background
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start Ollama service")
	}

	// Wait a moment for service to start
	time.Sleep(3 * time.Second)

	// Verify it's running
	if !h.IsRunning() {
		return errors.New("Ollama service failed to start properly")
	}

	klog.Info("Ollama service started successfully!")
	return nil
}

// PullModel downloads a specific model for use with Ollama
func (h *OllamaHelper) PullModel(model string) error {
	if !h.IsRunning() {
		return errors.New("Ollama service is not running. Start it with: ollama serve")
	}

	klog.Infof("Pulling model: %s (this may take several minutes)...", model)

	cmd := exec.Command("ollama", "pull", model)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "failed to pull model %s", model)
	}

	klog.Infof("Model %s pulled successfully!", model)
	return nil
}

// ListAvailableModels returns a list of recommended models for troubleshooting
func (h *OllamaHelper) ListAvailableModels() []ModelInfo {
	return []ModelInfo{
		{
			Name:        "llama2:7b",
			Size:        "3.8GB",
			Description: "General purpose model, good balance of performance and resource usage",
			Recommended: true,
		},
		{
			Name:        "llama2:13b",
			Size:        "7.3GB",
			Description: "Better analysis quality but requires more memory",
			Recommended: false,
		},
		{
			Name:        "codellama:7b",
			Size:        "3.8GB",
			Description: "Specialized for code analysis and technical content",
			Recommended: true,
		},
		{
			Name:        "codellama:13b",
			Size:        "7.3GB",
			Description: "Advanced code analysis, higher quality but resource intensive",
			Recommended: false,
		},
		{
			Name:        "mistral:7b",
			Size:        "4.1GB",
			Description: "Fast and efficient model for quick analysis",
			Recommended: false,
		},
	}
}

// ModelInfo contains information about available models
type ModelInfo struct {
	Name        string
	Size        string
	Description string
	Recommended bool
}

// PrintModelRecommendations prints user-friendly model selection guide
func (h *OllamaHelper) PrintModelRecommendations() {
	fmt.Println("\n📚 Recommended Models for Troubleshooting:")
	fmt.Println("=" + strings.Repeat("=", 50))

	for _, model := range h.ListAvailableModels() {
		status := " "
		if model.Recommended {
			status = "⭐"
		}

		fmt.Printf("%s %s (%s)\n", status, model.Name, model.Size)
		fmt.Printf("   %s\n", model.Description)

		if model.Recommended {
			fmt.Printf("   💡 Pull with: ollama pull %s\n", model.Name)
		}
		fmt.Println()
	}

	fmt.Println("💡 For beginners: Start with 'llama2:7b' or 'codellama:7b'")
	fmt.Println("🔧 For advanced users: Try 'llama2:13b' if you have enough RAM")
}

// OllamaHealthStatus represents the current state of Ollama
type OllamaHealthStatus struct {
	Installed bool
	Running   bool
	Models    []string
	Endpoint  string
}

// GetHealthStatus returns the current status of Ollama installation and service
func (h *OllamaHelper) GetHealthStatus() OllamaHealthStatus {
	status := OllamaHealthStatus{
		Installed: h.IsInstalled(),
		Running:   false,
		Models:    []string{},
		Endpoint:  "http://localhost:11434",
	}

	if status.Installed {
		status.Running = h.IsRunning()

		if status.Running {
			// Get list of installed models
			cmd := exec.Command("ollama", "list")
			output, err := cmd.Output()
			if err == nil {
				lines := strings.Split(string(output), "\n")
				for _, line := range lines {
					if strings.Contains(line, ":") && !strings.Contains(line, "NAME") {
						parts := strings.Fields(line)
						if len(parts) > 0 {
							status.Models = append(status.Models, parts[0])
						}
					}
				}
			}
		}
	}

	return status
}

// String returns a human-readable status summary
func (hs OllamaHealthStatus) String() string {
	var status strings.Builder

	status.WriteString("🔍 Ollama Status:\n")

	if hs.Installed {
		status.WriteString("✅ Installed\n")
		if hs.Running {
			status.WriteString("✅ Service Running\n")
			status.WriteString(fmt.Sprintf("🌐 Endpoint: %s\n", hs.Endpoint))

			if len(hs.Models) > 0 {
				status.WriteString(fmt.Sprintf("📚 Models Available: %s\n", strings.Join(hs.Models, ", ")))
			} else {
				status.WriteString("⚠️  No models installed. Run: ollama pull llama2:7b\n")
			}
		} else {
			status.WriteString("⚠️  Service Not Running. Start with: ollama serve\n")
		}
	} else {
		status.WriteString("❌ Not Installed\n")
		status.WriteString("💡 Install with: troubleshoot analyze --setup-ollama\n")
	}

	return status.String()
}

// getOllamaDownloadURL returns the platform-specific download URL
func getOllamaDownloadURL() string {
	switch runtime.GOOS {
	case "darwin", "linux":
		// Use the official install script for both macOS and Linux
		return "https://ollama.com/install.sh"
	case "windows":
		return "https://ollama.com/download/OllamaSetup.exe"
	default:
		return "https://ollama.com/install.sh"
	}
}

// getOllamaInstallPath returns the platform-specific install path
func getOllamaInstallPath() string {
	switch runtime.GOOS {
	case "darwin":
		return "/usr/local/bin/ollama"
	case "linux":
		return "/usr/local/bin/ollama"
	case "windows":
		return "C:\\Program Files\\Ollama\\ollama.exe"
	default:
		return "/usr/local/bin/ollama"
	}
}
