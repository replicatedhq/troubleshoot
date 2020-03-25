package cli

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/replicatedhq/troubleshoot/cmd/util"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/convert"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

func Analyze() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze [url]",
		Args:  cobra.MinimumNArgs(1),
		Short: "analyze a support bundle",
		Long:  `Analyze a support bundle using the Analyzer definitions provided`,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlag("bundle", cmd.Flags().Lookup("bundle"))
			viper.BindPFlag("output", cmd.Flags().Lookup("output"))
			viper.BindPFlag("quiet", cmd.Flags().Lookup("quiet"))
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			logger.SetQuiet(v.GetBool("quiet"))

			specPath := args[0]
			analyzerSpec, err := downloadAnalyzerSpec(specPath)
			if err != nil {
				return err
			}

			result, err := analyzer.DownloadAndAnalyze(v.GetString("bundle"), analyzerSpec)
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

	cmd.Flags().String("bundle", "", "filename of the support bundle to analyze")
	cmd.MarkFlagRequired("bundle")
	cmd.Flags().String("output", "", "output format: json, yaml")
	cmd.Flags().String("compatibility", "", "output compatibility mode: support-bundle")
	cmd.Flags().MarkHidden("compatibility")
	cmd.Flags().Bool("quiet", false, "enable/disable error messaging and only show parseable output")

	viper.BindPFlags(cmd.Flags())

	return cmd
}

func downloadAnalyzerSpec(specPath string) (string, error) {
	specContent := ""
	var err error
	if _, err = os.Stat(specPath); err == nil {
		if _, err := os.Stat(specPath); os.IsNotExist(err) {
			return "", fmt.Errorf("%s was not found", specPath)
		}

		b, err := ioutil.ReadFile(specPath)
		if err != nil {
			return "", err
		}

		specContent = string(b)
	} else {
		if !util.IsURL(specPath) {
			return "", fmt.Errorf("%s is not a URL and was not found (err %s)", specPath, err)
		}

		req, err := http.NewRequest("GET", specPath, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", "Replicated_Analyzer/v1beta1")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}

		specContent = string(body)
	}
	return specContent, nil
}
