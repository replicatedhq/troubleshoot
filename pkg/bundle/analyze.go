package bundle

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/convert"
)

func (a *AnalyzeOutput) ResultsJSON() ([]byte, error) {
	// TODO: This code is duplicated in pkg/supportbundle/collect.go to avoid a circular dependency
	// It'll be centralized in a future PR perhaps here.
	data := convert.FromAnalyzerResult(a.Results)
	analysis, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal analysis")
	}

	return analysis, nil
}
