package util

import (
	"bytes"
	"net/url"
	"os"
	"strings"
	"text/template"

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
// We have this function because of how the
// builtin append() function works. It treats
// target nil slices the same as empty slices.
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

// IsInCluster returns true if the code is running within a process
// inside a kubernetes pod
func IsInCluster() bool {
	// This is a best effort check, it's not guaranteed to be accurate
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if len(host) == 0 || len(port) == 0 {
		return false
	}

	return true
}

// RenderTemplate renders a template and returns the result as a string
func RenderTemplate(tpl string, data interface{}) (string, error) {
	// Create a new template and parse the letter into it
	t, err := template.New("data").Parse(tpl)
	if err != nil {
		return "", err
	}

	// Create a new buffer
	buf := new(bytes.Buffer)

	// Execute the template and write the bytes to the buffer
	err = t.Execute(buf, data)
	if err != nil {
		return "", err
	}

	// Return the string representation of the buffer
	return buf.String(), nil
}
