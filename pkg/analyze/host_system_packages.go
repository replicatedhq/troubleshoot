package analyzer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"text/template"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeHostSystemPackages struct {
	hostAnalyzer *troubleshootv1beta2.SystemPackagesAnalyze
}

func (a *AnalyzeHostSystemPackages) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "System Packages")
}

func (a *AnalyzeHostSystemPackages) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostSystemPackages) Analyze(getCollectedFileContents func(string) ([]byte, error)) ([]*AnalyzeResult, error) {
	hostAnalyzer := a.hostAnalyzer

	contents, err := getCollectedFileContents("system/packages.json")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get collected file")
	}

	var infos []collect.SystemPackageInfo
	if err := json.Unmarshal(contents, &infos); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal system packages info")
	}

	allResults := []*AnalyzeResult{}

	for _, info := range infos {
		for _, outcome := range hostAnalyzer.Outcomes {
			isInstalled := isPackageInstalled(info)
			r := AnalyzeResult{}
			when := ""

			if outcome.Fail != nil {
				r.IsFail = true
				r.Message = outcome.Fail.Message
				r.URI = outcome.Fail.URI
				when = outcome.Fail.When
			} else if outcome.Warn != nil {
				r.IsWarn = true
				r.Message = outcome.Warn.Message
				r.URI = outcome.Warn.URI
				when = outcome.Warn.When
			} else if outcome.Pass != nil {
				r.IsPass = true
				r.Message = outcome.Pass.Message
				r.URI = outcome.Pass.URI
				when = outcome.Pass.When
			} else {
				fmt.Println("error: found an empty outcome in a systemPackages analyzer") // don't stop
				continue
			}

			match, err := compareHostPackagesConditionalToActual(when, isInstalled)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %s", when)
			}

			if !match {
				continue
			}

			tmpl := template.New("package")

			// template the title
			titleTmpl, err := tmpl.Parse(a.Title())
			if err != nil {
				return nil, errors.Wrap(err, "failed to create new title template")
			}
			var t bytes.Buffer
			err = titleTmpl.Execute(&t, info)
			if err != nil {
				return nil, errors.Wrap(err, "failed to execute template")
			}
			r.Title = t.String()

			// template the message
			msgTmpl, err := tmpl.Parse(r.Message)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create new title template")
			}
			var m bytes.Buffer
			err = msgTmpl.Execute(&m, info)
			if err != nil {
				return nil, errors.Wrap(err, "failed to execute template")
			}
			r.Message = m.String()

			// add to results, break and check the next pod
			allResults = append(allResults, &r)
			break
		}
	}

	return allResults, nil
}

func templateString(value string, info collect.SystemPackageInfo) (string, error) {
	tmpl := template.New("package")

	// template the message
	msgTmpl, err := tmpl.Parse(value)
	if err != nil {
		return "", errors.Wrap(err, "failed to create new title template")
	}
	var m bytes.Buffer
	err = msgTmpl.Execute(&m, info)
	if err != nil {
		return "", errors.Wrap(err, "failed to execute template")
	}

	return m.String(), nil
}

func isPackageInstalled(info collect.SystemPackageInfo) bool {
	if info.Error != "" {
		return false
	}
	if info.ExitCode != "0" {
		return false
	}
	if strings.Contains(info.Details, "not installed") {
		return false
	}
	if strings.Contains(info.Details, "No matching Packages") {
		return false
	}
	return true
}

func compareHostPackagesConditionalToActual(conditional string, isInstalled bool) (res bool, err error) {
	if conditional == "" {
		return true, nil
	}

	switch conditional {
	case "installed":
		return isInstalled, nil
	case "unavailable":
		return !isInstalled, nil
	}

	return false, fmt.Errorf("invalid 'when' format: %s", conditional)
}
