package cli

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/cmd/util"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/spf13/viper"
)

func runAnalyzers(v *viper.Viper, bundlePath string) error {
	specPath := v.GetString("analyzers")

	specContent := ""
	var err error
	if _, err = os.Stat(specPath); err == nil {
		b, err := ioutil.ReadFile(specPath)
		if err != nil {
			return err
		}

		specContent = string(b)
	} else {
		if !util.IsURL(specPath) {
			return fmt.Errorf("%s is not a URL and was not found (err %s)", specPath, err)
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
