package util

import (
	"bytes"
	"io"
	"net/url"
	"os"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v2"
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

// SplitMultiDoc takes a multi-document yaml string and splits
// it into a slice strings, each containing a single yaml document
//
// This function does not split on "---" delimeter because some yaml documents
// such as ConfiMaps and Secrets can contain multi-docs (split by "---")
// within their data fields. Instead this function, uses the yaml
// package to decode the document and then re-encode each document individually
// to a string. On success, each, string will be single yaml document.
// If any of the documents are invalid yaml, the function will return an error.
func SplitMultiDoc(multidoc string) ([]string, error) {
	// NOTE: This is not the most performant function cause it can potentially
	// allocate a lot of memory while decoding and encoding the yaml documents,
	// as well as converting the strings to bytes and back to strings.
	// Its a convinience function.
	docs := []string{}
	dec := yaml.NewDecoder(bytes.NewReader([]byte(multidoc)))

	for {
		var doc any
		err := dec.Decode(&doc)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		out, err := yaml.Marshal(doc)
		if err != nil {
			return nil, err
		}
		docs = append(docs, string(out))
	}

	return docs, nil
}

// SplitMultiDocs takes a slice of multi-document yaml strings and splits it
// into a slice of strings, each containing a single yaml document.
func SplitMultiDocs(multidocs ...string) ([]string, error) {
	docs := []string{}
	for _, multidoc := range multidocs {
		split, err := SplitMultiDoc(multidoc)
		if err != nil {
			return nil, err
		}
		docs = append(docs, split...)
	}

	return docs, nil
}
