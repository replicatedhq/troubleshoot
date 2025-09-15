package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/replicatedhq/troubleshoot/cmd/internal/util"
	"github.com/replicatedhq/troubleshoot/internal/traces"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"github.com/replicatedhq/troubleshoot/pkg/updater"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "support-bundle [urls...]",
		Args:  cobra.MinimumNArgs(0),
		Short: "Generate a support bundle from a Kubernetes cluster or specified sources",
		Long: `Generate a support bundle, an archive containing files, output, metrics, and cluster state to aid in troubleshooting Kubernetes clusters.

If no arguments are provided, specs are automatically loaded from the cluster by default.

**Argument Types**:
1. **Secret**: Load specs from a Kubernetes Secret. Format: "secret/namespace-name/secret-name[/data-key]"
2. **ConfigMap**: Load specs from a Kubernetes ConfigMap. Format: "configmap/namespace-name/configmap-name[/data-key]"
3. **File**: Load specs from a local file. Format: Local file path
4. **Standard Input**: Read specs from stdin. Format: "-"
5. **URL**: Load specs from a URL. Supports HTTP and OCI registry URLs.`,
		SilenceUsage: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			v := viper.GetViper()
			v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
			v.BindPFlags(cmd.Flags())

			logger.SetupLogger(v)

			if err := util.StartProfiling(); err != nil {
				klog.Errorf("Failed to start profiling: %v", err)
			}

			// Auto-update support-bundle unless disabled by flag or env
			envAuto := os.Getenv("TROUBLESHOOT_AUTO_UPDATE")
			autoFromEnv := true
			if envAuto != "" {
				if strings.EqualFold(envAuto, "0") || strings.EqualFold(envAuto, "false") {
					autoFromEnv = false
				}
			}
			if v.GetBool("auto-update") && autoFromEnv {
				exe, err := os.Executable()
				if err == nil {
					_ = updater.CheckAndUpdate(cmd.Context(), updater.Options{
						BinaryName:  "support-bundle",
						CurrentPath: exe,
						Printf:      func(f string, a ...interface{}) { klog.V(1).Infof(f, a...) },
					})
				}
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			// If there are no locations to load specs from passed in the cli args, we should
			// load them from the cluster by setting "load-cluster-specs=true". If the caller
			// provided "--load-cluster-specs" cli option, we should respect it.
			if len(args) == 0 {
				// Check if --load-cluster-specs was set by the cli caller to avoid overriding it
				flg := cmd.Flags().Lookup("load-cluster-specs")
				if flg != nil && !flg.Changed {
					// Load specs from the cluster if no spec(s) is(are) provided in the cli args
					v.Set("load-cluster-specs", true)
				}
			}

			closer, err := traces.ConfigureTracing("support-bundle")
			if err != nil {
				// Do not fail running support-bundle if tracing fails
				klog.Errorf("Failed to initialize open tracing provider: %v", err)
			} else {
				defer closer()
			}

			err = runTroubleshoot(v, args)
			if !v.IsSet("dry-run") && (v.GetBool("debug") || v.IsSet("v")) {
				fmt.Fprintf(os.Stderr, "\n%s", traces.GetExporterInstance().GetSummary())
			}

			return err
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			if err := util.StopProfiling(); err != nil {
				klog.Errorf("Failed to stop profiling: %v", err)
			}
		},
	}

	cobra.OnInitialize(initConfig)

	cmd.AddCommand(Analyze())
	cmd.AddCommand(Redact())
	cmd.AddCommand(Diff())
	cmd.AddCommand(util.VersionCmd())

	cmd.Flags().StringSlice("redactors", []string{}, "names of the additional redactors to use")
	cmd.Flags().Bool("redact", true, "enable/disable default redactions")

	// Tokenization flags (Phase 4 integration)
	cmd.Flags().Bool("tokenize", false, "enable intelligent tokenization instead of simple masking (replaces ***HIDDEN*** with ***TOKEN_TYPE_HASH***)")
	cmd.Flags().String("redaction-map", "", "generate redaction mapping file at specified path (enables token→original mapping for authorized access)")
	cmd.Flags().Bool("encrypt-redaction-map", false, "encrypt the redaction mapping file using AES-256 (requires --redaction-map)")
	cmd.Flags().String("token-prefix", "", "custom token prefix format (default: ***TOKEN_%s_%s***)")
	cmd.Flags().Bool("verify-tokenization", false, "validation mode: verify tokenization setup without collecting data")
	cmd.Flags().String("bundle-id", "", "custom bundle identifier for token correlation (auto-generated if not provided)")
	cmd.Flags().Bool("tokenization-stats", false, "include detailed tokenization statistics in output")
	cmd.Flags().Bool("interactive", true, "enable/disable interactive mode")
	cmd.Flags().Bool("collect-without-permissions", true, "always generate a support bundle, even if it some require additional permissions")
	cmd.Flags().StringSliceP("selector", "l", []string{"troubleshoot.sh/kind=support-bundle"}, "selector to filter on for loading additional support bundle specs found in secrets within the cluster")
	cmd.Flags().Bool("load-cluster-specs", false, "enable/disable loading additional troubleshoot specs found within the cluster. Do not load by default unless no specs are provided in the cli args")
	cmd.Flags().String("since-time", "", "force pod logs collectors to return logs after a specific date (RFC3339)")
	cmd.Flags().String("since", "", "force pod logs collectors to return logs newer than a relative duration like 5s, 2m, or 3h.")
	cmd.Flags().StringP("output", "o", "", "specify the output file path for the support bundle")
	cmd.Flags().Bool("debug", false, "enable debug logging. This is equivalent to --v=0")
	cmd.Flags().Bool("dry-run", false, "print support bundle spec without collecting anything")
	cmd.Flags().Bool("auto-update", true, "enable automatic binary self-update check and install")

	// Auto-discovery flags
	cmd.Flags().Bool("auto", false, "enable auto-discovery of foundational collectors. When used with YAML specs, adds foundational collectors to YAML collectors. When used alone, collects only foundational data")
	cmd.Flags().Bool("include-images", false, "include container image metadata collection when using auto-discovery")
	cmd.Flags().Bool("rbac-check", true, "enable RBAC permission checking for auto-discovered collectors")
	cmd.Flags().String("discovery-profile", "standard", "auto-discovery profile: minimal, standard, comprehensive, or paranoid")
	cmd.Flags().StringSlice("exclude-namespaces", []string{}, "namespaces to exclude from auto-discovery (supports glob patterns)")
	cmd.Flags().StringSlice("include-namespaces", []string{}, "namespaces to include in auto-discovery (supports glob patterns). If specified, only these namespaces will be included")
	cmd.Flags().Bool("include-system-namespaces", false, "include system namespaces (kube-system, etc.) in auto-discovery")

	// Auto-discovery flags
	cmd.Flags().Bool("auto", false, "enable auto-discovery of foundational collectors. When used with YAML specs, adds foundational collectors to YAML collectors. When used alone, collects only foundational data")
	cmd.Flags().Bool("include-images", false, "include container image metadata collection when using auto-discovery")
	cmd.Flags().Bool("rbac-check", true, "enable RBAC permission checking for auto-discovered collectors")
	cmd.Flags().String("discovery-profile", "standard", "auto-discovery profile: minimal, standard, comprehensive, or paranoid")
	cmd.Flags().StringSlice("exclude-namespaces", []string{}, "namespaces to exclude from auto-discovery (supports glob patterns)")
	cmd.Flags().StringSlice("include-namespaces", []string{}, "namespaces to include in auto-discovery (supports glob patterns). If specified, only these namespaces will be included")
	cmd.Flags().Bool("include-system-namespaces", false, "include system namespaces (kube-system, etc.) in auto-discovery")

	// hidden in favor of the `insecure-skip-tls-verify` flag
	cmd.Flags().Bool("allow-insecure-connections", false, "when set, do not verify TLS certs when retrieving spec and reporting results")
	cmd.Flags().MarkHidden("allow-insecure-connections")

	// `no-uri` references the `followURI` functionality where we can use an upstream spec when creating a support bundle
	// This flag makes sure we can also disable this and fall back to the default spec.
	cmd.Flags().Bool("no-uri", false, "When this flag is used, Troubleshoot does not attempt to retrieve the spec referenced by the uri: field`")

	k8sutil.AddFlags(cmd.Flags())

	// Initialize klog flags
	logger.InitKlogFlags(cmd)

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
