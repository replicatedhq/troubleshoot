package convert

import (
	"fmt"
	"regexp"
	"strings"

	multierror "github.com/hashicorp/go-multierror"
	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
)

type Meta struct {
	Name   string            `json:"name,omitempty" yaml:"name,omitempty" hcl:"name,omitempty"`
	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty" hcl:"labels,omitempty"`
}

const (
	SeverityError Severity = "error"
	SeverityWarn  Severity = "warn"
	SeverityInfo  Severity = "info"
	SeverityDebug Severity = "debug"
)

type Severity string

type Insight struct {
	Meta `json:",inline" yaml:",inline" hcl:",inline"`

	Primary  string   `json:"primary" yaml:"primary" hcl:"primary"`
	Detail   string   `json:"detail" yaml:"detail" hcl:"detail"`
	Severity Severity `json:"severity,omitempty" yaml:"severity,omitempty" hcl:"severity,omitempty"`
}

type Result struct {
	Meta `json:",inline" yaml:",inline" hcl:",inline"`

	Insight      *Insight               `json:"insight" yaml:"insight" hcl:"insight"`
	Severity     Severity               `json:"severity" yaml:"severity" hcl:"severity"`
	AnalyzerSpec string                 `json:"analyzerSpec" yaml:"analyzerSpec" hcl:"analyzerSpec"`
	Variables    map[string]interface{} `json:"variables,omitempty" yaml:"variables,omitempty" hcl:"variables,omitempty"`
	Error        string                 `json:"error,omitempty" yaml:"error,omitempty" hcl:"error,omitempty"`
}

func (m *Insight) Render(data interface{}) (*Insight, error) {
	var multiErr *multierror.Error
	var err error
	built := &Insight{
		Meta:     m.Meta,
		Severity: m.Severity,
	}
	built.Primary, err = String(m.Primary, data)
	multiErr = multierror.Append(multiErr, errWrap(err, "build primary"))
	built.Detail, err = String(m.Detail, data)
	multiErr = multierror.Append(multiErr, errWrap(err, "build detail"))
	return built, multiErr.ErrorOrNil()
}

func errWrap(err error, text string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %v", text, err)
}

func FromAnalyzerResult(input []*analyze.AnalyzeResult) []*Result {
	reg := regexp.MustCompile("[^a-zA-Z0-9]+")

	result := make([]*Result, 0)
	for _, i := range input {
		name := reg.ReplaceAllString(strings.ToLower(i.Title), ".")
		r := &Result{
			Meta: Meta{
				Name: name,
				Labels: map[string]string{
					"desiredPosition": "1",
					"iconKey":         "gray_checkmark",
				},
			},
			Insight: &Insight{
				Meta: Meta{
					Name: name,
					Labels: map[string]string{
						"iconKey": "gray_checkmark",
					},
				},
				Primary: i.Title,
				Detail:  i.Message,
			},
			AnalyzerSpec: "",
			Variables:    map[string]interface{}{},
		}
		if i.IsFail {
			r.Severity = SeverityError
			r.Insight.Severity = SeverityError
			r.Error = i.Message
		} else if i.IsWarn {
			r.Severity = SeverityWarn
			r.Insight.Severity = SeverityWarn
		} else if i.IsPass {
			r.Severity = SeverityDebug
			r.Insight.Severity = SeverityDebug
		}
		result = append(result, r)
	}

	return result
}
