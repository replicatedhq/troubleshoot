package util

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/user"
	"strings"
	"text/template"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const HOST_COLLECTORS_RUN_AS_ROOT_PROMPT = "Some host collectors need to be run as root.\nDo you want to exit and rerun the command using sudo?"

type Keyer interface {
	UniqKey() string
}

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

func IsRunningAsRoot() bool {
	currentUser, err := user.Current()
	if err != nil {
		return false
	}

	// Check if the user ID is 0 (root's UID)
	return currentUser.Uid == "0"
}

func PromptYesNo(question string) bool {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s (yes/no): ", question)

		response, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading response:", err)
			continue
		}

		response = strings.TrimSpace(response)
		response = strings.ToLower(response)

		if response == "yes" || response == "y" {
			return true
		} else if response == "no" || response == "n" {
			return false
		} else {
			fmt.Println("Please type 'yes' or 'no'.")
		}
	}
}

func Dedup[T any](objs []T) []T {
	seen := make(map[string]bool)
	out := []T{}

	if len(objs) == 0 {
		return objs
	}

	for _, o := range objs {
		var key string

		// Check if the object implements the Keyer interface
		if k, ok := any(o).(Keyer); ok {
			key = k.UniqKey()
		} else {
			data, err := json.Marshal(o)
			if err != nil {
				out = append(out, o)
				continue
			}
			key = string(data)
		}

		if _, ok := seen[key]; !ok {
			out = append(out, o)
			seen[key] = true
		}
	}

	return out
}
