package analyzer

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

func analyzeClusterVersion(analyzer *troubleshootv1beta1.ClusterVersion, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	clusterInfo, err := getCollectedFileContents("cluster-info/cluster_version.json")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get contents of cluster_version.json")
	}

	collectorClusterVersion := collect.ClusterVersion{}
	if err := json.Unmarshal(clusterInfo, &collectorClusterVersion); err != nil {
		return nil, errors.Wrap(err, "failed to parse cluster_version.json")
	}

	k8sVersion, err := semver.Make(strings.TrimLeft(collectorClusterVersion.String, "v"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse semver from cluster_version.json")
	}

	return analyzeClusterVersionResult(k8sVersion, analyzer.Outcomes, analyzer.CheckName)
}

func analyzeClusterVersionResult(k8sVersion semver.Version, outcomes []*troubleshootv1beta1.Outcome, checkName string) (*AnalyzeResult, error) {
	result := AnalyzeResult{}
	for _, outcome := range outcomes {
		when := ""
		message := ""
		uri := ""

		title := checkName
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

		// When is usually empty as the final case and should be treated as true
		if when == "" {
			result.Message = message
			result.URI = uri

			return &result, nil
		}

		whenRange, err := semver.ParseRange(when)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse semver range")
		}

		if whenRange(k8sVersion) {
			result.Message = message
			result.URI = uri

			return &result, nil
		}
	}

	return &AnalyzeResult{}, nil
}
