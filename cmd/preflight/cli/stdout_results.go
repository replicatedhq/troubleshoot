package cli

import (
	"fmt"

	analyzerunner "github.com/replicatedhq/troubleshoot/pkg/analyze"
)

func showStdoutResults(preflightName string, analyzeResults []*analyzerunner.AnalyzeResult) error {
	fmt.Printf("\n=== TEST   %s\n", preflightName)
	for _, analyzeResult := range analyzeResults {
		fmt.Printf("=== RUN:   %s\n", analyzeResult.Title)
	}
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
