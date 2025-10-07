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
		Short: "Lint v1beta3 preflight specs for syntax and structural errors",
		Long: `Lint v1beta3 preflight specs for syntax and structural errors.

This command validates v1beta3 preflight specs and checks for:
- YAML syntax errors
- Missing required fields (apiVersion, kind, metadata, spec)
- Invalid template syntax ({{ .Values.* }})
- Missing analyzers or collectors
- Common structural issues
- Missing docStrings (warning)

The linter only validates v1beta3 specs. For v1beta2 specs, use the 'convert' command first.

Examples:
  # Lint a single spec file
  preflight lint my-preflight.yaml

  # Lint multiple spec files
  preflight lint spec1.yaml spec2.yaml spec3.yaml

  # Lint with automatic fixes
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
