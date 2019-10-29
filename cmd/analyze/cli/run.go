package cli

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"github.com/pkg/errors"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/spf13/viper"
)

func runAnalyzers(v *viper.Viper, bundlePath string) error {
	specPath := v.GetString("analyzers")

	specContent := ""
	if !isURL(specPath) {
		if _, err := os.Stat(specPath); os.IsNotExist(err) {
			return fmt.Errorf("%s was not found", specPath)
		}

		b, err := ioutil.ReadFile(specPath)
		if err != nil {
			return err
		}

		specContent = string(b)
	} else {
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

	analyzeResults, err := analyzer.DownloadAndAnalyze(specContent, bundlePath)
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

func isURL(str string) bool {
	parsed, err := url.ParseRequestURI(str)
	if err != nil {
		return false
	}

	return parsed.Scheme != ""
}
