package util

import (
	"net/url"
	"os"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func HomeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func IsURL(str string) bool {
	parsed, err := url.ParseRequestURI(str)
	if err != nil {
		return false
	}

	return parsed.Scheme != ""
}

func AppName(name string) string {
	words := strings.Split(cases.Title(language.English).String(strings.ReplaceAll(name, "-", " ")), " ")
	casedWords := []string{}
	for i, word := range words {
		if strings.ToLower(word) == "ai" {
			casedWords = append(casedWords, "AI")
		} else if strings.ToLower(word) == "io" && i > 0 {
			casedWords[i-1] += ".io"
		} else {
			casedWords = append(casedWords, word)
		}
	}

	return strings.Join(casedWords, " ")
}

func SplitYAML(doc string) []string {
	return strings.Split(doc, "\n---\n")
}

func EstimateNumberOfLines(text string) int {
	n := strings.Count(text, "\n")
	if len(text) > 0 && !strings.HasSuffix(text, "\n") {
		n++
	}
	return n
}

// Append appends elements in src to target.
// We have this function because of how append()
// treats nil slices the same as empty slices.
// An empty array in YAML like below is not the
// same as when the array is not specified.
//
//	 spec:
//		  collectors: []
func Append[T any](target []T, src []T) []T {
	// Do nothing only if src is nil
	if src == nil {
		return target
	}

	// In case target is nil, we need to initialize it
	// since append() will not do it for us when len(src) == 0
	if target == nil {
		target = []T{}
	}
	return append(target, src...)
}
