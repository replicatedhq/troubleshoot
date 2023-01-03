package preflight

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	analyzerunner "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"gopkg.in/yaml.v2"
)

func showStdoutResults(format string, preflightName string, analyzeResults []*analyzerunner.AnalyzeResult) error {
	if format == "human" {
		return showStdoutResultsHuman(preflightName, analyzeResults)
	} else if format == "json" {
		return showStdoutResultsJSON(preflightName, analyzeResults)
	} else if format == "yaml" {
		return showStdoutResultsYAML(preflightName, analyzeResults)
	}

	return errors.Errorf("unknown output format: %q", format)
}

func showStdoutResultsHuman(preflightName string, analyzeResults []*analyzerunner.AnalyzeResult) error {
	fmt.Println("")
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

type StdoutResultOutput struct {
	Title   string `json:"title" yaml:"title"`
	Message string `json:"message" yaml:"message"`
	URI     string `json:"uri,omitempty" yaml:"uri,omitempty"`
	Strict  bool   `json:"strict,omitempty" yaml:"strict,omitempty"`
}

type StdoutOutput struct {
	Pass []StdoutResultOutput `json:"pass,omitempty" yaml:"pass,omitempty"`
	Warn []StdoutResultOutput `json:"warn,omitempty" yaml:"warn,omitempty"`
	Fail []StdoutResultOutput `json:"fail,omitempty" yaml:"fail,omitempty"`
}

// Used by both JSON and YAML outputs
func showStdoutResultsStructured(preflightName string, analyzeResults []*analyzerunner.AnalyzeResult) *StdoutOutput {
	output := StdoutOutput{
		Pass: []StdoutResultOutput{},
		Warn: []StdoutResultOutput{},
		Fail: []StdoutResultOutput{},
	}

	for _, analyzeResult := range analyzeResults {
		resultOutput := StdoutResultOutput{
			Title:   analyzeResult.Title,
			Message: analyzeResult.Message,
			URI:     analyzeResult.URI,
		}

		if analyzeResult.Strict {
			resultOutput.Strict = analyzeResult.Strict
		}

		if analyzeResult.IsPass {
			output.Pass = append(output.Pass, resultOutput)
		} else if analyzeResult.IsWarn {
			output.Warn = append(output.Warn, resultOutput)
		} else if analyzeResult.IsFail {
			output.Fail = append(output.Fail, resultOutput)
		}
	}

	return &output
}

func showStdoutResultsJSON(preflightName string, analyzeResults []*analyzerunner.AnalyzeResult) error {
	output := showStdoutResultsStructured(preflightName, analyzeResults)

	b, err := json.MarshalIndent(*output, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal results as json")
	}

	fmt.Printf("%s\n", b)

	return nil
}

func showStdoutResultsYAML(preflightName string, analyzeResults []*analyzerunner.AnalyzeResult) error {
	output := showStdoutResultsStructured(preflightName, analyzeResults)

	b, err := yaml.Marshal(*output)
	if err != nil {
		return errors.Wrap(err, "failed to marshal results as yaml")
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
	}

	if analyzeResult.Strict {
		fmt.Printf("      --- Strict: %t\n", analyzeResult.Strict)
	}

	if analyzeResult.IsFail {
		return true
	}
	return false
}
