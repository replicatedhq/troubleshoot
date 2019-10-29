package main

import "C"

import (
	"fmt"
	"context"
	"encoding/json"
	
	"gopkg.in/yaml.v2"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"github.com/replicatedhq/troubleshoot/pkg/convert"

)

//export Analyze
func Analyze(bundleURL string, analyzers string, outputFormat string, compatibility string) *C.char { 
	logger.SetQuiet(true)

	result, err := analyzer.DownloadAndAnalyze(context.Background(), bundleURL, analyzers)
	if err != nil {
		fmt.Printf("error downloading and analyzing: %s\n", err.Error())
		return C.CString("")
	}

	var data interface{}
	switch compatibility {
	case "support-bundle":
		data = convert.FromAnalyzerResult(result)
	default:
		data = result
	}

	var formatted []byte
	switch outputFormat {
	case "json":
		formatted, err = json.MarshalIndent(data, "", "    ")
	case "", "yaml":
		formatted, err = yaml.Marshal(data)
	default:
		fmt.Printf("unknown output format: %s\n", outputFormat)
		return C.CString("")
	}

	if err != nil {
		fmt.Printf("error formatting output: %#v\n", err)
		return C.CString("")
	}

	return C.CString(string(formatted))
}

func main() {}
