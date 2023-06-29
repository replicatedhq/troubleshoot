package util

import (
	"net/url"
	"os"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"k8s.io/klog/v2"
)

func HomeDir() string {
	homedir, err := os.UserHomeDir()
	if err != nil {
		klog.Warningf("Unable to get user home directory: %v", err)
		return ""
	}

	return homedir
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
