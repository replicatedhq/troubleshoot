package preflight

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/cmd/util"
	analyzerunner "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/convert"
)

func outputToFile(preflightName string, outputPath string, analyzeResults []*analyzerunner.AnalyzeResult) (string, error) {
	filename := ""
	if outputPath != "" {
		// use override output path
		overridePath, err := convert.ValidateOutputPath(outputPath)
		if err != nil {
			return "", errors.Wrap(err, "override output file path")
		}
		filename = overridePath
	} else {
		// use default output path
		filename = fmt.Sprintf("%s-results-%s.txt", preflightName, time.Now().Format("2006-01-02T15_04_05"))
	}

	_, err := os.Stat(filename)
	if err == nil {
		os.Remove(filename)
	}

	results := fmt.Sprintf("%s Preflight Checks\n\n", util.AppName(preflightName))
	for _, analyzeResult := range analyzeResults {
		result := ""

		if analyzeResult.IsPass {
			result = "Check PASS\n"
		} else if analyzeResult.IsWarn {
			result = "Check WARN\n"
		} else if analyzeResult.IsFail {
			result = "Check FAIL\n"
		}

		result = result + fmt.Sprintf("Title: %s\n", analyzeResult.Title)
		result = result + fmt.Sprintf("Message: %s\n", analyzeResult.Message)

		if analyzeResult.URI != "" {
			result = result + fmt.Sprintf("URI: %s\n", analyzeResult.URI)
		}

		if analyzeResult.Note != "" {
			result = result + fmt.Sprintf("Note: %s\n", analyzeResult.Note)
		}

		if analyzeResult.Strict {
			result = result + fmt.Sprintf("Strict: %t\n", analyzeResult.Strict)
		}

		result = result + "\n------------\n"

		results = results + result
	}

	if err := ioutil.WriteFile(filename, []byte(results), 0644); err != nil {
		return "", errors.Wrap(err, "failed to save preflight results")
	}

	return filename, nil
}
