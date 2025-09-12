package cli

import (
	"github.com/replicatedhq/troubleshoot/pkg/preflight"
	"github.com/spf13/cobra"
)

func TemplateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template [template-file]",
		Short: "Render a templated preflight spec with values",
		Long: `Process a templated preflight YAML file, substituting variables and removing conditional sections based on provided values.

Examples:
  # Render template with default values
  preflight template sample-preflight-templated.yaml

  # Render template with values from files
  preflight template sample-preflight-templated.yaml --values values-base.yaml --values values-prod.yaml

  # Render template with inline values
  preflight template sample-preflight-templated.yaml --set postgres.enabled=true --set cluster.minNodes=5

  # Render template and save to file
  preflight template sample-preflight-templated.yaml --output rendered.yaml`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			templateFile := args[0]
			valuesFiles, _ := cmd.Flags().GetStringSlice("values")
			outputFile, _ := cmd.Flags().GetString("output")
			setValues, _ := cmd.Flags().GetStringSlice("set")

			return preflight.RunTemplate(templateFile, valuesFiles, setValues, outputFile)
		},
	}

	cmd.Flags().StringSlice("values", []string{}, "Path to YAML files containing template values (can be used multiple times)")
	cmd.Flags().StringSlice("set", []string{}, "Set template values on the command line (can be used multiple times)")
	cmd.Flags().StringP("output", "o", "", "Output file (default: stdout)")

	return cmd
}
