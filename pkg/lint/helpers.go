package lint

import (
	"fmt"
	"regexp"
	"strings"
)

// findLineNumber returns the first 1-based line number containing the search string
func findLineNumber(content, search string) int {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, search) {
			return i + 1
		}
	}
	return 0
}

func findAnalyzerLine(content string, index int) int {
	return findListItemLine(content, "analyzers", index)
}

// findCollectorLine locates the starting line of the Nth entry in a collectors list
func findCollectorLine(content string, field string, index int) int {
	return findListItemLine(content, field, index)
}

// findListItemLine locates the starting line of the Nth entry in a list under listKey
func findListItemLine(content, listKey string, index int) int {
	lines := strings.Split(content, "\n")
	count := 0
	inList := false
	for i, line := range lines {
		if strings.Contains(line, listKey+":") {
			inList = true
			continue
		}
		if inList && strings.HasPrefix(strings.TrimSpace(line), "- ") {
			if count == index {
				return i + 1
			}
			count++
		}
		if inList && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && strings.TrimSpace(line) != "" {
			break
		}
	}
	return 0
}

// extractLineFromError tries to parse a YAML error message for a line number
func extractLineFromError(err error) int {
	// Try to extract line number from YAML error message
	re := regexp.MustCompile(`line (\d+)`)
	matches := re.FindStringSubmatch(err.Error())
	if len(matches) > 1 {
		var line int
		fmt.Sscanf(matches[1], "%d", &line)
		return line
	}
	return 0
}
