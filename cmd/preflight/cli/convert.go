package cli

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/convert"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func ConvertCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "convert [input-file]",
		Args:  cobra.ExactArgs(1),
		Short: "Convert v1beta2 preflight specs to v1beta3 format",
		Long: `Convert v1beta2 preflight specs to v1beta3 format with templating and values.

This command converts a v1beta2 preflight spec to the new v1beta3 templated format. It will:
- Update the apiVersion to troubleshoot.sh/v1beta3
- Extract hardcoded values and create a values.yaml file
- Add conditional templating ({{- if .Values.feature.enabled }})
- Add placeholder docString comments for you to fill in
- Template hardcoded values with {{ .Values.* }} expressions

The conversion will create two files:
- [input-file]-v1beta3.yaml: The templated v1beta3 spec
- [input-file]-values.yaml: The values file with extracted configuration

Example:
  preflight convert my-preflight.yaml

This creates:
  my-preflight-v1beta3.yaml
  my-preflight-values.yaml`,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			inputFile := args[0]
			outputSpec := v.GetString("output-spec")
			outputValues := v.GetString("output-values")

			// Generate default output filenames if not specified
			if outputSpec == "" {
				ext := filepath.Ext(inputFile)
				base := strings.TrimSuffix(inputFile, ext)
				outputSpec = base + "-v1beta3" + ext
			}

			if outputValues == "" {
				ext := filepath.Ext(inputFile)
				base := strings.TrimSuffix(inputFile, ext)
				outputValues = base + "-values" + ext
			}

			return runConvert(v, inputFile, outputSpec, outputValues)
		},
	}

	cmd.Flags().String("output-spec", "", "Output file for the templated v1beta3 spec (default: [input]-v1beta3.yaml)")
	cmd.Flags().String("output-values", "", "Output file for the values (default: [input]-values.yaml)")
	cmd.Flags().Bool("dry-run", false, "Preview the conversion without writing files")

	return cmd
}

func runConvert(v *viper.Viper, inputFile, outputSpec, outputValues string) error {
	// Read input file
	inputData, err := ioutil.ReadFile(inputFile)
	if err != nil {
		return errors.Wrapf(err, "failed to read input file %s", inputFile)
	}

	// Check if it's a valid v1beta2 preflight spec
	if !strings.Contains(string(inputData), "troubleshoot.sh/v1beta2") {
		return fmt.Errorf("input file does not appear to be a v1beta2 troubleshoot spec")
	}

	if !strings.Contains(string(inputData), "kind: Preflight") {
		return fmt.Errorf("input file does not appear to be a Preflight spec")
	}

	// Convert to v1beta3
	result, err := convert.ConvertToV1Beta3(inputData)
	if err != nil {
		return errors.Wrap(err, "failed to convert spec")
	}

	dryRun := v.GetBool("dry-run")

	if dryRun {
		fmt.Println("=== Templated v1beta3 Spec ===")
		fmt.Println(result.TemplatedSpec)
		fmt.Println("\n=== Values File ===")
		fmt.Println(result.ValuesFile)
		fmt.Println("\n=== Conversion Summary ===")
		fmt.Printf("Would write templated spec to: %s\n", outputSpec)
		fmt.Printf("Would write values to: %s\n", outputValues)
		return nil
	}

	// Write templated spec
	err = ioutil.WriteFile(outputSpec, []byte(result.TemplatedSpec), 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to write templated spec to %s", outputSpec)
	}

	// Write values file
	err = ioutil.WriteFile(outputValues, []byte(result.ValuesFile), 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to write values to %s", outputValues)
	}

	fmt.Printf("Successfully converted %s to v1beta3 format:\n", inputFile)
	fmt.Printf("  Templated spec: %s\n", outputSpec)
	fmt.Printf("  Values file: %s\n", outputValues)
	fmt.Println("\nNext steps:")
	fmt.Println("1. Add docStrings with Title, Requirement, and rationale for each check")
	fmt.Println("2. Customize the values in the values file")
	fmt.Println("3. Test the conversion with:")
	fmt.Printf("   preflight template %s --values %s\n", outputSpec, outputValues)
	fmt.Println("4. Run the templated preflight:")
	fmt.Printf("   preflight run %s --values %s\n", outputSpec, outputValues)

	return nil
}
