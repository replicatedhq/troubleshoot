package analyzer

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/blang/semver"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

func analyzeClusterVersion(analyzer *troubleshootv1beta1.ClusterVersion, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	clusterInfo, err := getCollectedFileContents("cluster-info/cluster_version.json")
	if err != nil {
		return nil, err
	}

	collectorClusterVersion := collect.ClusterVersion{}
	if err := json.Unmarshal(clusterInfo, &collectorClusterVersion); err != nil {
		return nil, err
	}

	k8sVersion, err := semver.Make(strings.TrimLeft(collectorClusterVersion.String, "v"))
	if err != nil {
		return nil, err
	}

	result := AnalyzeResult{}
	for _, outcome := range analyzer.Outcomes {
		when := ""
		message := ""
		uri := ""

		title := analyzer.CheckName
		if title == "" {
			title = "Required Kubernetes Version"
		}

		result = AnalyzeResult{
			Title: title,
		}

		if outcome.Fail != nil {
			result.IsFail = true
			when = outcome.Fail.When
			message = outcome.Fail.Message
			uri = outcome.Fail.URI
		} else if outcome.Warn != nil {
			result.IsWarn = true
			when = outcome.Warn.When
			message = outcome.Warn.Message
			uri = outcome.Warn.URI
		} else if outcome.Pass != nil {
			result.IsPass = true
			when = outcome.Pass.When
			message = outcome.Pass.Message
			uri = outcome.Pass.URI
		} else {
			return nil, errors.New("empty outcome")
		}

		whenRange, err := semver.ParseRange(when)
		if err != nil {
			return nil, err
		}

		if whenRange(k8sVersion) {
			result.Message = message
			result.URI = uri

			return &result, nil
		}
	}

	return &AnalyzeResult{}, nil
}
