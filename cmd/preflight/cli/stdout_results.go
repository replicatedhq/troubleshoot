package cli

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	analyzerunner "github.com/replicatedhq/troubleshoot/pkg/analyze"
)

func showStdoutResults(format string, preflightName string, analyzeResults []*analyzerunner.AnalyzeResult) error {
	if format == "human" {
		return showStdoutResultsHuman(preflightName, analyzeResults)
	} else if format == "json" {
		return showStdoutResultsJSON(preflightName, analyzeResults)
	}

	return errors.Errorf("unknown output format: %q", format)
}

func showStdoutResultsHuman(preflightName string, analyzeResults []*analyzerunner.AnalyzeResult) error {
	var failed bool
	for _, analyzeResult := range analyzeResults {
		testResultfailed := outputResult(analyzeResult)
		if testResultfailed {
			failed = true
		}
	}
	if failed {
		fmt.Printf("--- FAIL   %s\n", preflightName)
		fmt.Println("FAILED")
	} else {
		fmt.Printf("--- PASS   %s\n", preflightName)
		fmt.Println("PASS")
	}
	return nil
}

func showStdoutResultsJSON(preflightName string, analyzeResults []*analyzerunner.AnalyzeResult) error {
	type ResultOutput struct {
		Title   string `json:"title"`
		Message string `json:"message"`
		URI     string `json:"uri,omitempty"`
	}
	type Output struct {
		Pass []ResultOutput `json:"pass,omitempty"`
		Warn []ResultOutput `json:"warn,omitempty"`
		Fail []ResultOutput `json:"fail,omitempty"`
	}

	output := Output{
		Pass: []ResultOutput{},
		Warn: []ResultOutput{},
		Fail: []ResultOutput{},
	}

	for _, analyzeResult := range analyzeResults {
		resultOutput := ResultOutput{
			Title:   analyzeResult.Title,
			Message: analyzeResult.Message,
			URI:     analyzeResult.URI,
		}

		if analyzeResult.IsPass {
			output.Pass = append(output.Pass, resultOutput)
		} else if analyzeResult.IsWarn {
			output.Warn = append(output.Warn, resultOutput)
		} else if analyzeResult.IsFail {
			output.Fail = append(output.Fail, resultOutput)
		}
	}

	b, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal results")
	}

	fmt.Printf("%s\n", b)

	return nil
}

func outputResult(analyzeResult *analyzerunner.AnalyzeResult) bool {
	if analyzeResult.IsPass {
		fmt.Printf("   --- PASS %s\n", analyzeResult.Title)
		fmt.Printf("      --- %s\n", analyzeResult.Message)
	} else if analyzeResult.IsWarn {
		fmt.Printf("   --- WARN: %s\n", analyzeResult.Title)
		fmt.Printf("      --- %s\n", analyzeResult.Message)
	} else if analyzeResult.IsFail {
		fmt.Printf("   --- FAIL: %s\n", analyzeResult.Title)
		fmt.Printf("      --- %s\n", analyzeResult.Message)
		return true
	}
	return false
}
