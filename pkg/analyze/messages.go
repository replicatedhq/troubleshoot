package analyzer

import (
	"bytes"
	"text/template"
)

type analyzerValue struct {
	ActualValue interface{}
}

// buildMessage parses {{ .actualValue }} in message strings
func buildMessage(msg string, actualValue interface{}) string {
	value := analyzerValue{
		ActualValue: actualValue,
	}

	templ, err := template.New("message").Parse(msg)
	if err != nil {
		return msg
	}

	var result bytes.Buffer
	err = templ.Execute(&result, value)
	if err != nil {
		return msg
	}

	return result.String()
}
