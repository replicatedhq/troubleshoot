package analyzer

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"text/template"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/textutil"
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

	packagesFileName := "system/packages.json"
	if a.hostAnalyzer.CollectorName != "" {
		packagesFileName = fmt.Sprintf("system/%s-packages.json", a.hostAnalyzer.CollectorName)
	}

	contents, err := getCollectedFileContents(packagesFileName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get collected file")
	}

	var info collect.SystemPackagesInfo
	if err := json.Unmarshal(contents, &info); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal system packages info")
	}

	allResults := []*AnalyzeResult{}

	for _, pkg := range info.Packages {
		templateMap := getSystemPackageTemplateMap(pkg, info.OS, info.OSVersion)

		for _, outcome := range hostAnalyzer.Outcomes {
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
				println("error: found an empty outcome in a systemPackages analyzer") // don't stop
				continue
			}

			match, err := compareSystemPackagesConditionalToActual(when, templateMap)
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
			err = titleTmpl.Execute(&t, templateMap)
			if err != nil {
				return nil, errors.Wrap(err, "failed to execute title template")
			}
			r.Title = t.String()

			// template the message
			msgTmpl, err := tmpl.Parse(r.Message)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create new message template")
			}
			var m bytes.Buffer
			err = msgTmpl.Execute(&m, templateMap)
			if err != nil {
				return nil, errors.Wrap(err, "failed to execute message template")
			}
			r.Message = m.String()

			// add to results, break and check the next pod
			allResults = append(allResults, &r)
			break
		}
	}

	return allResults, nil
}

func getSystemPackageTemplateMap(pkg collect.SystemPackage, osName string, osVersion string) map[string]interface{} {
	m := map[string]interface{}{
		"OS":          osName,
		"OSVersion":   osVersion,
		"Name":        pkg.Name,
		"Error":       pkg.Error,
		"ExitCode":    pkg.ExitCode,
		"IsInstalled": isSystemPackageInstalled(pkg),
	}

	for k, v := range getSystemPackageDetailsMap(pkg) {
		m[k] = v
	}

	return m
}

func isSystemPackageInstalled(pkg collect.SystemPackage) bool {
	if pkg.Error != "" {
		return false
	}
	if pkg.ExitCode != "0" {
		return false
	}
	if strings.Contains(pkg.Details, "not installed") {
		return false
	}
	if strings.Contains(pkg.Details, "No matching Packages") {
		return false
	}
	return true
}

func getSystemPackageDetailsMap(pkg collect.SystemPackage) map[string]string {
	// TODO: handle multiline values
	m := map[string]string{}

	buffer := bytes.NewBuffer([]byte(pkg.Details))
	scanner := bufio.NewScanner(buffer)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)

		if len(parts) != 2 {
			continue
		}

		key := parts[0]

		// key should start with an upper case
		if !unicode.IsUpper([]rune(key)[0]) {
			continue
		}

		// key shouldn't start with a space
		if unicode.IsSpace([]rune(key)[0]) {
			continue
		}

		// sanitize the key and value
		key = textutil.StripSpaces(key) // remove all spaces, even the ones in between
		key = strings.ReplaceAll(key, "-", "")
		value := strings.TrimSpace(parts[1])

		if key == "" || value == "" {
			continue
		}

		m[key] = value
	}

	return m
}

func compareSystemPackagesConditionalToActual(conditional string, templateMap map[string]interface{}) (res bool, err error) {
	if conditional == "" {
		return true, nil
	}

	tmpl := template.New("conditional")

	conditionalTmpl, err := tmpl.Parse(conditional)
	if err != nil {
		return false, errors.Wrap(err, "failed to create new when template")
	}

	var when bytes.Buffer
	err = conditionalTmpl.Execute(&when, templateMap)
	if err != nil {
		return false, errors.Wrap(err, "failed to execute when template")
	}

	return when.String() == "true", nil
}
