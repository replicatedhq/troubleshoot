package main

import (
	"fmt"
	"regexp"
)

func main() {
	// Test the regex pattern against the test data
	pattern := `(?i)(\s*(?:password|pwd|pass)\s*:\s*["\']?)(?P<mask>[^"\'\s\n\r]+)(["\']?\s*$)`
	testLine := `password1: "secret123"`

	re := regexp.MustCompile(pattern)
	fmt.Printf("Pattern: %s\n", pattern)
	fmt.Printf("Test line: %s\n", testLine)
	fmt.Printf("Does it match? %v\n", re.MatchString(testLine))

	if matches := re.FindStringSubmatch(testLine); matches != nil {
		fmt.Printf("Matches found: %v\n", matches)
		for i, name := range re.SubexpNames() {
			if i < len(matches) {
				fmt.Printf("Group %d (%s): %s\n", i, name, matches[i])
			}
		}
	} else {
		fmt.Println("No matches found")
	}

	// Let's also try a simpler test
	simplePattern := `(?i)(password\d+\s*:\s*["\']?)(?P<mask>[^"\']+)(["\']?\s*)`
	re2 := regexp.MustCompile(simplePattern)
	fmt.Printf("\nSimple pattern: %s\n", simplePattern)
	fmt.Printf("Does it match? %v\n", re2.MatchString(testLine))

	if matches := re2.FindStringSubmatch(testLine); matches != nil {
		fmt.Printf("Matches found: %v\n", matches)
		for i, name := range re2.SubexpNames() {
			if i < len(matches) {
				fmt.Printf("Group %d (%s): %s\n", i, name, matches[i])
			}
		}
	}
}
