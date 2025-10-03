package redact

import (
	"bufio"
	"bytes"
	"io"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"
)

type YamlRedactor struct {
	maskPath   []string
	foundMatch bool
	filePath   string
	redactName string
	isDefault  bool
}

func NewYamlRedactor(yamlPath, filePath, name string) *YamlRedactor {
	pathComponents := strings.Split(yamlPath, ".")
	return &YamlRedactor{maskPath: pathComponents, filePath: filepath.ToSlash(filePath), redactName: name}
}

func (r *YamlRedactor) Redact(input io.Reader, targetPath string) io.Reader {
	if r.filePath != "" {
		normalizedTarget := filepath.ToSlash(targetPath)
		match, err := path.Match(r.filePath, normalizedTarget)
		if err != nil {
			klog.Errorf("Failed to match %q and %q: %v", r.filePath, normalizedTarget, err)
			return input
		}
		if !match {
			return input
		}
	}
	r.foundMatch = false
	reader, writer := io.Pipe()
	go func() {
		var err error
		defer func() {
			if err == io.EOF {
				writer.Close()
			} else {
				writer.CloseWithError(err)
			}
		}()
		reader := bufio.NewReader(input)

		var doc []byte
		doc, err = io.ReadAll(reader)
		var yamlInterface interface{}
		err = yaml.Unmarshal(doc, &yamlInterface)
		if err != nil {
			buf := bytes.NewBuffer(doc)
			buf.WriteTo(writer)
			err = nil // this is not a fatal error
			return
		}

		processedPath := filepath.ToSlash(targetPath)
		newYaml := r.redactYaml(yamlInterface, r.maskPath, processedPath)
		if !r.foundMatch {
			// no match found, so make no changes
			buf := bytes.NewBuffer(doc)
			buf.WriteTo(writer)
			return
		}

		var newBytes []byte
		newBytes, err = yaml.Marshal(newYaml)
		if err != nil {
			return
		}

		buf := bytes.NewBuffer(newBytes)
		buf.WriteTo(writer)

		addRedaction(Redaction{
			RedactorName:      r.redactName,
			CharactersRemoved: len(doc) - len(newBytes),
			Line:              0, // line 0 because we have no way to tell what line was impacted
			File:              targetPath,
			IsDefaultRedactor: r.isDefault,
		})
	}()
	return reader
}

func (r *YamlRedactor) redactYaml(in interface{}, path []string, targetPath string) interface{} {
	if len(path) == 0 {
		r.foundMatch = true

		// Use tokenization if enabled
		tokenizer := GetGlobalTokenizer()
		if tokenizer.IsEnabled() {
			// Convert the value to string and tokenize it
			if valueStr, ok := in.(string); ok && valueStr != "" {
				context := r.redactName
				return tokenizer.TokenizeValueWithPath(valueStr, context, targetPath)
			}
		}

		return MASK_TEXT
	}
	switch typed := in.(type) {
	case []interface{}:
		// check if first path element is * - if it is, run redact on all children
		if path[0] == "*" {
			var newArr []interface{}
			for _, child := range typed {
				newChild := r.redactYaml(child, path[1:], targetPath)
				newArr = append(newArr, newChild)
			}
			return newArr
		}
		// check if first path element is an integer - if it is, run redact on that child
		pathIdx, err := strconv.Atoi(path[0])
		if err != nil {
			return typed
		}
		if len(typed) > pathIdx {
			child := typed[pathIdx]
			typed[pathIdx] = r.redactYaml(child, path[1:], targetPath)
			return typed
		}
		return typed
	case map[interface{}]interface{}:
		if path[0] == "*" && len(typed) > 0 {
			newMap := map[interface{}]interface{}{}
			for key, child := range typed {
				newMap[key] = r.redactYaml(child, path[1:], targetPath)
			}
			return newMap
		}

		child, ok := typed[path[0]]
		if ok {
			newChild := r.redactYaml(child, path[1:], targetPath)
			typed[path[0]] = newChild
		}
		return typed
	default:
		return typed
	}
}
