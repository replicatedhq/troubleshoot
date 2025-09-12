package main

import (
	"fmt"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

func main() {
	fmt.Println("=== Debugging Collector Test Issue ===")

	// Exact same test data as the failing test
	testData := `abc 123
another line here
pwd=somethinggoeshere;`

	fmt.Println("Original input:")
	fmt.Printf("%q\n", testData)

	// Same custom redactors as the test
	redactors := []*troubleshootv1beta2.Redact{
		{
			Name: "",
			Removals: troubleshootv1beta2.Removals{
				Values: nil,
				Regex: []troubleshootv1beta2.Regex{
					{Redactor: `abc`},
					{Redactor: `(another)(?P<mask>.*)(here)`},
				},
			},
		},
	}

	// Simulate what the collector test does
	result := map[string][]byte{
		"data/datacollectorname": []byte(testData),
	}

	fmt.Println("\nBefore redaction:")
	for k, v := range result {
		fmt.Printf("%s: %q\n", k, string(v))
	}

	// Apply redaction like the test does
	err := collect.RedactResult("", result, redactors)
	if err != nil {
		fmt.Printf("Error during redaction: %v\n", err)
		return
	}

	fmt.Println("\nAfter redaction:")
	for k, v := range result {
		fmt.Printf("%s: %q\n", k, string(v))
	}

	// Check what the test expects vs what we got
	expected := " 123\nanother***HIDDEN***here\npwd=***HIDDEN***;\n"
	actual := string(result["data/datacollectorname"])

	fmt.Printf("\nExpected: %q\n", expected)
	fmt.Printf("Actual:   %q\n", actual)

	if expected == actual {
		fmt.Println("✅ SUCCESS: Output matches expected!")
	} else {
		fmt.Println("❌ FAILURE: Output doesn't match expected")
		fmt.Printf("Missing: %q\n", expected[len(actual):])
	}
}
