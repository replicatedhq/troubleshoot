package analyzer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"text/template"

	"github.com/hashicorp/go-multierror"

	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeInstalledPackage struct {
	hanalyzer *v1beta2.InstalledPackageAnalyze
}

func (a *AnalyzeInstalledPackage) Title() string {
	return hostAnalyzerTitleOrDefault(a.hanalyzer.AnalyzeMeta, "Host Package")
}

// ProcessTemplate parses the content collect by the collector an then applies the result in the 'exclude'
// property from the analyzer.
func (a *AnalyzeInstalledPackage) ProcessTemplate(getFileContents func(string) ([]byte, error)) error {
	fullPath := path.Join("host-collectors", "hostPackages", "hostPackages.json")
	if a.hanalyzer.CollectorName != "" {
		fname := fmt.Sprintf("%s.json", a.hanalyzer.CollectorName)
		fullPath = path.Join("host-collectors", "hostPackages", fname)
	}

	data, err := getFileContents(fullPath)
	if err != nil {
		return fmt.Errorf("failed to read collector content: %w", err)
	}

	var info collect.HostInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return fmt.Errorf("failed to unmarshal collector content: %w", err)
	}

	tpl := template.New("template")
	tpl, err = tpl.Parse(a.hanalyzer.AnalyzeMeta.Exclude.StrVal)
	if err != nil {
		return fmt.Errorf("failed to parse exclude template: %w", err)
	}

	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, info.OSInfo); err != nil {
		return fmt.Errorf("failed to execute exclude template: %w", err)
	}

	a.hanalyzer.AnalyzeMeta.Exclude.StrVal = buf.String()
	return nil
}

func (a *AnalyzeInstalledPackage) IsExcluded() (bool, error) {
	return isExcluded(a.hanalyzer.Exclude)
}

func (a *AnalyzeInstalledPackage) Analyze(getFileContents func(string) ([]byte, error)) ([]*AnalyzeResult, error) {
	fullPath := path.Join("host-collectors", "hostPackages", "hostPackages.json")
	if a.hanalyzer.CollectorName != "" {
		fname := fmt.Sprintf("%s.json", a.hanalyzer.CollectorName)
		fullPath = path.Join("host-collectors", "hostPackages", fname)
	}

	data, err := getFileContents(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read collected file name %s: %w", fullPath, err)
	}

	var ospkgs collect.HostInfo
	if err := json.Unmarshal(data, &ospkgs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal collected data: %w", err)
	}

	pkg := ospkgs.PackageByName(a.hanalyzer.PackageName)
	if pkg == nil {
		res := &AnalyzeResult{
			IsFail:  true,
			Title:   fmt.Sprintf("Package %s not installed", a.hanalyzer.PackageName),
			Message: fmt.Sprintf("Package %s was not found in the system", a.hanalyzer.PackageName),
		}
		return []*AnalyzeResult{res}, nil
	}

	// if no outcome has been provided then we are only checking if the package has been
	// installed. on this case just return an ok.
	if len(a.hanalyzer.Outcomes) == 0 {
		res := &AnalyzeResult{
			IsPass:  true,
			Title:   fmt.Sprintf("Package %s installed", pkg.Name),
			Message: fmt.Sprintf("Package %s is installed (version %s)", pkg.Name, pkg.Version),
		}
		return []*AnalyzeResult{res}, nil
	}

	res, err := a.validateOutcomes(pkg)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze outcomes: %w", err)
	}
	return []*AnalyzeResult{res}, nil
}

func (a *AnalyzeInstalledPackage) prepareResult(outcome *v1beta2.Outcome) (*AnalyzeResult, string) {
	title := a.hanalyzer.CheckName
	if title == "" {
		title = fmt.Sprintf("Package %s required version", a.hanalyzer.PackageName)
	}
	result := &AnalyzeResult{Title: title}

	if outcome.Fail != nil {
		result.IsFail = true
		result.Message = outcome.Fail.Message
		result.URI = outcome.Fail.URI
		return result, outcome.Fail.When
	}

	if outcome.Warn != nil {
		result.IsWarn = true
		result.Message = outcome.Warn.Message
		result.URI = outcome.Warn.URI
		return result, outcome.Warn.When
	}

	if outcome.Pass != nil {
		result.IsPass = true
		result.Message = outcome.Pass.Message
		result.URI = outcome.Pass.URI
		return result, outcome.Pass.When
	}

	return nil, ""
}

func (a *AnalyzeInstalledPackage) validateOutcomes(pkg *collect.HostInstalledPackage) (*AnalyzeResult, error) {
	for _, outcome := range a.hanalyzer.Outcomes {
		result, when := a.prepareResult(outcome)
		if result == nil {
			return nil, fmt.Errorf("empty outcome")
		} else if when == "" {
			return result, nil
		}

		// if something went wrong when evaluating if the package is within the semantic
		// version range then or the range is not valid or the package does not use semantic
		// versions at all, on this case we move on and try to validate using regex.
		var errs *multierror.Error
		if matches, err := pkg.InRange(when); err != nil {
			errs = multierror.Append(errs, err)
		} else if matches {
			return result, nil
		} else {
			continue
		}

		if matches, err := pkg.MatchesRegex(when); err != nil {
			errs = multierror.Append(errs, err)
			return nil, errs
		} else if matches {
			return result, nil
		}
	}
	return &AnalyzeResult{}, nil
}
