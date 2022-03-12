package preflight

import (
	"encoding/json"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

// HasStrictAnalyzers - checks and returns true if a preflight's analyzer has strict:true, else false
func HasStrictAnalyzers(preflight *troubleshootv1beta2.Preflight) (bool, error) {
	if preflight == nil {
		return false, nil
	}

	marshalledAnalyzers, err := json.Marshal(preflight.Spec.Analyzers) // marshall and remove nil Analyzers eg result: "[{\"clusterVersion\":{\"exclude\":\"\",\"strict\":\"false\",\"outcomes\":null}}]"
	if err != nil {
		return false, errors.Wrap(err, "failed to marshal analyzers")
	}

	analyzersMap := []map[string]interface{}{}
	err = json.Unmarshal(marshalledAnalyzers, &analyzersMap) // Unmarshall again so we can loop over non nil analyzers
	if err != nil {
		return false, errors.Wrap(err, "failed to unmarshal analyzers")
	}

	// analyzerMap will ignore empty Analyzers and loop around Analyzer with data
	for _, analyzers := range analyzersMap { // for each analyzer: map["clusterVersion": map[string]interface{} ["exclude": "", "strict": "true", "outcomes": nil]
		for _, analyzer := range analyzers { // for each analyzerMeta: map[string]interface{} ["exclude": "", "strict": "true", "outcomes": nil]
			marshalledAnalyzer, err := json.Marshal(analyzer)
			if err != nil {
				return false, errors.Wrap(err, "error while marshalling analyzer")
			}
			// return Analyzer.Strict which can be extracted from AnalyzeMeta
			analyzeMeta := troubleshootv1beta2.AnalyzeMeta{}
			err = json.Unmarshal(marshalledAnalyzer, &analyzeMeta)
			if err != nil {
				return false, errors.Wrap(err, "error while un-marshalling marshalledAnalyzers")
			}
			return analyzeMeta.Strict.BoolOrDefaultFalse(), nil
		}
	}
	return false, nil
}

// HasStrictAnalyzersFailed - checks if preflight analyzer's result is strict:true and isFail:true, then returns true else false
func HasStrictAnalyzersFailed(preflightResult *UploadPreflightResults) bool {
	hasStrictAnalyzersFailed := false
	// if results are empty, treat as failure
	if preflightResult == nil || len(preflightResult.Results) == 0 {
		hasStrictAnalyzersFailed = true
	} else {
		for _, result := range preflightResult.Results {
			if result.IsFail && result.Strict {
				hasStrictAnalyzersFailed = true
			}
		}
	}
	return hasStrictAnalyzersFailed
}
