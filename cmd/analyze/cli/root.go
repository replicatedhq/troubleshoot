package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/replicatedhq/troubleshoot/cmd/internal/util"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

// validateArgs allows certain flags to run without requiring bundle arguments
func validateArgs(cmd *cobra.Command, args []string) error {
	// Special flags that don't require bundle arguments
	if cmd.Flags().Changed("check-ollama") || cmd.Flags().Changed("setup-ollama") ||
		cmd.Flags().Changed("list-models") || cmd.Flags().Changed("pull-model") {
		return nil
	}

	// For all other cases, require at least 1 argument (the bundle path)
	if len(args) < 1 {
		return fmt.Errorf("requires at least 1 arg(s), only received %d. Usage: analyze [bundle-path] or use --check-ollama/--setup-ollama", len(args))
	}

	return nil
}

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "analyze [url]",
		Args:         validateArgs,
		Short:        "Analyze a support bundle",
		Long:         `Run a series of analyzers on a support bundle archive`,
		SilenceUsage: true,
		PreRun: func(cmd *cobra.Command, args []string) {
			v := viper.GetViper()
			v.BindPFlags(cmd.Flags())

			logger.SetupLogger(v)

			if err := util.StartProfiling(); err != nil {
				klog.Errorf("Failed to start profiling: %v", err)
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			// Handle cases where no bundle argument is provided (for utility flags)
			var bundlePath string
			if len(args) > 0 {
				bundlePath = args[0]
			}

			return runAnalyzers(v, bundlePath)
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			if err := util.StopProfiling(); err != nil {
				klog.Errorf("Failed to stop profiling: %v", err)
			}
		},
	}

	cobra.OnInitialize(initConfig)

	cmd.AddCommand(util.VersionCmd())

	cmd.Flags().String("analyzers", "", "filename or url of the analyzers to use")
	cmd.Flags().Bool("debug", false, "enable debug logging")

	// Advanced analysis flags
	cmd.Flags().Bool("advanced-analysis", false, "use advanced analysis engine with AI capabilities")
	cmd.Flags().StringSlice("agents", []string{"local"}, "analysis agents to use: local, hosted, ollama")
	cmd.Flags().Bool("enable-ollama", false, "enable Ollama AI-powered analysis")
	cmd.Flags().Bool("disable-ollama", false, "explicitly disable Ollama AI-powered analysis")
	cmd.Flags().String("ollama-endpoint", "http://localhost:11434", "Ollama server endpoint")
	cmd.Flags().String("ollama-model", "llama2:7b", "Ollama model to use for analysis")
	cmd.Flags().Bool("use-codellama", false, "use CodeLlama model for code-focused analysis")
	cmd.Flags().Bool("use-mistral", false, "use Mistral model for fast analysis")
	cmd.Flags().Bool("auto-pull-model", true, "automatically pull model if not available")
	cmd.Flags().Bool("list-models", false, "list all available/installed Ollama models and exit")
	cmd.Flags().Bool("pull-model", false, "pull the specified model and exit")
	cmd.Flags().Bool("setup-ollama", false, "automatically setup and configure Ollama")
	cmd.Flags().Bool("check-ollama", false, "check Ollama installation status and exit")
	cmd.Flags().Bool("include-remediation", true, "include remediation suggestions in analysis results")
	cmd.Flags().String("output-file", "", "save analysis results to file (e.g., --output-file results.json)")

	viper.BindPFlags(cmd.Flags())

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// Initialize klog flags
	logger.InitKlogFlags(cmd)

	k8sutil.AddFlags(cmd.Flags())

	// CPU and memory profiling flags
	util.AddProfilingFlags(cmd)

	return cmd
}

func InitAndExecute() {
	if err := RootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func initConfig() {
	viper.SetEnvPrefix("TROUBLESHOOT")
	viper.AutomaticEnv()
}
