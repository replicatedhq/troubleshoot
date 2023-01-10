package preflight

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pkg/errors"
	analyzerunner "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"gopkg.in/yaml.v2"
)

// Text results can go to stdout or to an output file

func showTextResults(format string, preflightName string, outputPath string, analyzeResults []*analyzerunner.AnalyzeResult) error {
	results := ""
	var err error
	if format == "human" {
		results, err = showTextResultsHuman(preflightName, analyzeResults)
	} else if format == "json" {
		results, err = showTextResultsJSON(preflightName, analyzeResults)
	} else if format == "yaml" {
		results, err = showTextResultsYAML(preflightName, analyzeResults)
	} else {
		return errors.Errorf("unknown output format: %q", format)
	}
	if err != nil {
		return err
	}

	if outputPath != "" {
		// Write to output file
		resultsBytes := []byte(results)
		err := os.WriteFile(outputPath, resultsBytes, 0644)
		if err != nil {
			return err
		}
		fmt.Printf("Output written to '%s'\n", outputPath)
		return nil
	} else {
		// Print to stdout
		fmt.Printf("%s", results)
		return nil
	}
}

func showTextResultsHuman(preflightName string, analyzeResults []*analyzerunner.AnalyzeResult) (string, error) {
	results := fmt.Sprintln("")
	var failed bool
	for _, analyzeResult := range analyzeResults {
		testResultfailed := false
		results, testResultfailed = outputResult(results, analyzeResult)
		if testResultfailed {
			failed = true
		}
	}
	if failed {
		results = fmt.Sprintf("%s--- FAIL   %s\n", results, preflightName)
		results = fmt.Sprintf("%sFAILED\n", results)
	} else {
		results = fmt.Sprintf("%s--- PASS   %s\n", results, preflightName)
		results = fmt.Sprintf("%sPASS\n", results)
	}
	return results, nil
}

type textResultOutput struct {
	Title   string `json:"title" yaml:"title"`
	Message string `json:"message" yaml:"message"`
	URI     string `json:"uri,omitempty" yaml:"uri,omitempty"`
	Strict  bool   `json:"strict,omitempty" yaml:"strict,omitempty"`
}

type textOutput struct {
	Pass []textResultOutput `json:"pass,omitempty" yaml:"pass,omitempty"`
	Warn []textResultOutput `json:"warn,omitempty" yaml:"warn,omitempty"`
	Fail []textResultOutput `json:"fail,omitempty" yaml:"fail,omitempty"`
}

// Used by both JSON and YAML outputs
func showTextResultsStructured(preflightName string, analyzeResults []*analyzerunner.AnalyzeResult) *textOutput {
	output := textOutput{
		Pass: []textResultOutput{},
		Warn: []textResultOutput{},
		Fail: []textResultOutput{},
	}

	for _, analyzeResult := range analyzeResults {
		resultOutput := textResultOutput{
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

func showTextResultsJSON(preflightName string, analyzeResults []*analyzerunner.AnalyzeResult) (string, error) {
	output := showTextResultsStructured(preflightName, analyzeResults)

	b, err := json.MarshalIndent(*output, "", "  ")
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal results as json")
	}

	return fmt.Sprintf("%s\n", b), nil
}

func showTextResultsYAML(preflightName string, analyzeResults []*analyzerunner.AnalyzeResult) (string, error) {
	output := showTextResultsStructured(preflightName, analyzeResults)

	b, err := yaml.Marshal(*output)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal results as yaml")
	}

	return fmt.Sprintf("%s\n", b), nil
}

func outputResult(results string, analyzeResult *analyzerunner.AnalyzeResult) (string, bool) {
	if analyzeResult.IsPass {
		results = fmt.Sprintf("%s   --- PASS %s\n", results, analyzeResult.Title)
		results = fmt.Sprintf("%s      --- %s\n", results, analyzeResult.Message)
	} else if analyzeResult.IsWarn {
		results = fmt.Sprintf("%s   --- WARN: %s\n", results, analyzeResult.Title)
		results = fmt.Sprintf("%s      --- %s\n", results, analyzeResult.Message)
	} else if analyzeResult.IsFail {
		results = fmt.Sprintf("%s   --- FAIL: %s\n", results, analyzeResult.Title)
		results = fmt.Sprintf("%s      --- %s\n", results, analyzeResult.Message)
	}

	if analyzeResult.Strict {
		results = fmt.Sprintf("%s      --- Strict: %t\n", results, analyzeResult.Strict)
	}

	if analyzeResult.IsFail {
		return results, true
	}
	return results, false
}
