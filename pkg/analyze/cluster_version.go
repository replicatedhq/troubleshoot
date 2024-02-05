package analyzer

import (
	"encoding/json"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeClusterVersion struct {
	analyzer *troubleshootv1beta2.ClusterVersion
}

func (a *AnalyzeClusterVersion) Title() string {
	return title(a.analyzer.CheckName)
}

func (a *AnalyzeClusterVersion) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeClusterVersion) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	result, err := analyzeClusterVersion(a.analyzer, getFile)
	if err != nil {
		return nil, err
	}
	result.Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	return []*AnalyzeResult{result}, nil
}

func analyzeClusterVersion(analyzer *troubleshootv1beta2.ClusterVersion, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	clusterInfo, err := getCollectedFileContents("cluster-info/cluster_version.json")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get contents of cluster_version.json")
	}

	collectorClusterVersion := collect.ClusterVersion{}
	if err := json.Unmarshal(clusterInfo, &collectorClusterVersion); err != nil {
		return nil, errors.Wrap(err, "failed to parse cluster_version.json")
	}

	// Workaround for https://github.com/aws/containers-roadmap/issues/1404
	// for EKS, replace pre-release gitVersion string with the release version
	if strings.Contains(collectorClusterVersion.String, "-eks-") {
		collectorClusterVersion.String = strings.Split(collectorClusterVersion.String, "-")[0]
	}

	k8sVersion, err := semver.Make(strings.TrimLeft(collectorClusterVersion.String, "v"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse semver from cluster_version.json")
	}

	return analyzeClusterVersionResult(k8sVersion, analyzer.Outcomes, analyzer.CheckName)
}

func title(checkName string) string {
	if checkName == "" {
		return "Required Kubernetes Version"
	}

	return checkName
}

func analyzeClusterVersionResult(k8sVersion semver.Version, outcomes []*troubleshootv1beta2.Outcome, checkName string) (*AnalyzeResult, error) {
	for _, outcome := range outcomes {
		when := ""
		message := ""
		uri := ""

		result := AnalyzeResult{
			Title:   title(checkName),
			IconKey: "kubernetes_cluster_version",
			IconURI: "https://troubleshoot.sh/images/analyzer-icons/kubernetes.svg?w=16&h=16",
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
