package cli

import (
	"fmt"

	analyzerunner "github.com/replicatedhq/troubleshoot/pkg/analyze"
)

var failed = false

func showStdoutResults(preflightName string, analyzeResults []*analyzerunner.AnalyzeResult) error {
	fmt.Printf("\n=== TEST   %s\n", preflightName)
	for _, analyzeResult := range analyzeResults {
		fmt.Printf("=== RUN:   %s\n", analyzeResult.Title)
	}
	for _, analyzeResult := range analyzeResults {
		outputResult(analyzeResult)
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

func outputResult(analyzeResult *analyzerunner.AnalyzeResult) {
	if analyzeResult.IsPass {
		fmt.Printf("   --- PASS %s\n", analyzeResult.Title)
		fmt.Printf("      --- %s\n", analyzeResult.Message)
	} else if analyzeResult.IsWarn {
		fmt.Printf("   --- WARN: %s\n", analyzeResult.Title)
		fmt.Printf("      --- %s\n", analyzeResult.Message)
	} else if analyzeResult.IsFail {
		failed = true
		fmt.Printf("   --- FAIL: %s\n", analyzeResult.Title)
		fmt.Printf("      --- %s\n", analyzeResult.Message)
	}
}
