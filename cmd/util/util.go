package util

import (
	"net/url"
	"os"
	"strings"
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
	words := strings.Split(strings.Title(strings.Replace(name, "-", " ", -1)), " ")
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
