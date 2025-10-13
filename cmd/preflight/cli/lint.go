package cli

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/lint"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func LintCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint [spec-files...]",
		Args:  cobra.MinimumNArgs(1),
		Short: "Lint preflight specs for syntax and structural errors",
		Long: `Lint preflight specs for syntax and structural errors.

This command validates troubleshoot specs and checks for:
- YAML syntax errors (missing colons, invalid structure)
- Missing required fields (apiVersion, kind, metadata, spec)
- Invalid template syntax ({{ .Values.* }}, {{ .Release.* }}, etc.)
- Missing analyzers or collectors
- Common structural issues
- Missing docStrings (warning)

Both v1beta2 and v1beta3 apiVersions are supported. Use 'convert' if you need a full structural conversion between schema versions.

The --fix flag can automatically repair:
- Missing colons in YAML (e.g., "metadata" → "metadata:")
- Missing or malformed apiVersion line. If templating ({{ }}) or docString fields are detected, apiVersion is set to v1beta3; otherwise v1beta2.
- Template expressions missing leading dot (e.g., "{{ Values.x }}" → "{{ .Values.x }}")

Examples:
  # Lint a single spec file
  preflight lint my-preflight.yaml

  # Lint multiple spec files
  preflight lint spec1.yaml spec2.yaml spec3.yaml

  # Lint with automatic fixes (may need to run multiple times for complex issues)
  preflight lint --fix my-preflight.yaml

  # Lint and output as JSON for CI/CD integration
  preflight lint --format json my-preflight.yaml

Exit codes:
  0 - No errors found
  2 - Validation errors found`,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			opts := lint.LintOptions{
				FilePaths: args,
				Fix:       v.GetBool("fix"),
				Format:    v.GetString("format"),
			}

			return runLint(opts)
		},
	}

	cmd.Flags().Bool("fix", false, "Automatically fix issues where possible")
	cmd.Flags().String("format", "text", "Output format: text or json")

	return cmd
}

func runLint(opts lint.LintOptions) error {
	// Validate file paths exist
	for _, filePath := range opts.FilePaths {
		if _, err := os.Stat(filePath); err != nil {
			return errors.Wrapf(err, "file not found: %s", filePath)
		}
	}

	// Run linting
	results, err := lint.LintFiles(opts)
	if err != nil {
		return errors.Wrap(err, "failed to lint files")
	}

	// Format and print results
	output := lint.FormatResults(results, opts.Format)
	fmt.Print(output)

	// Return appropriate exit code
	if lint.HasErrors(results) {
		os.Exit(constants.EXIT_CODE_SPEC_ISSUES)
	}

	return nil
}
