package cli

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/convert"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

func Analyze() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "analyze a support bundle",
		Long:  `...`,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlag("bundle", cmd.Flags().Lookup("bundle"))
			viper.BindPFlag("spec", cmd.Flags().Lookup("spec"))
			viper.BindPFlag("output", cmd.Flags().Lookup("output"))
			viper.BindPFlag("quiet", cmd.Flags().Lookup("quiet"))
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			logger.SetQuiet(v.GetBool("quiet"))

			filename := v.GetString("spec")
			var analyzersSpec string
			if len(filename) > 0 {
				out, err := ioutil.ReadFile(filename)
				if err != nil {
					return err
				}
				analyzersSpec = string(out)
			}

			result, err := analyzer.DownloadAndAnalyze(v.GetString("bundle"), analyzersSpec)
			if err != nil {
				return err
			}

			var data interface{}
			switch v.GetString("compatibility") {
			case "support-bundle":
				data = convert.FromAnalyzerResult(result)
			default:
				data = result
			}

			var formatted []byte
			switch v.GetString("output") {
			case "json":
				formatted, err = json.MarshalIndent(data, "", "    ")
			case "", "yaml":
				formatted, err = yaml.Marshal(data)
			default:
				return fmt.Errorf("unsupported output format: %q", v.GetString("output"))
			}

			if err != nil {
				return err
			}

			fmt.Printf("%s", formatted)
			return nil
		},
	}

	cmd.Flags().String("bundle", "", "Filename of the support bundle to analyze")
	cmd.MarkFlagRequired("bundle")

	cmd.Flags().String("spec", "", "Filename of the analyze yaml spec")
	cmd.Flags().String("output", "", "output format: json, yaml")
	cmd.Flags().String("compatibility", "", "output compatibility mode: support-bundle")
	cmd.Flags().MarkHidden("compatibility")
	cmd.Flags().Bool("quiet", false, "enable/disable error messaging and only show parseable output")

	viper.BindPFlags(cmd.Flags())

	return cmd
}
