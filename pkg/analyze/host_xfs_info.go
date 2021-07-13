package analyzer

import (
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"gopkg.in/yaml.v2"
)

const XFS_FTYPE_DISABLED = "XFS_FTYPE_DISABLED"
const XFS_FTYPE_ENABLED = "XFS_FTYPE_ENABLED"
const NOT_XFS = "NOT_XFS"

type AnalyzeXFSInfo struct {
	hostAnalyzer *troubleshootv1beta2.XFSInfoAnalyze
}

func (a *AnalyzeXFSInfo) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "XFS Info")
}

func (a *AnalyzeXFSInfo) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeXFSInfo) Analyze(getCollectedFileContents func(string) ([]byte, error)) ([]*AnalyzeResult, error) {
	key := collect.GetXFSPath(a.hostAnalyzer.CollectorName)
	contents, err := getCollectedFileContents(key)
	if err != nil {
		return nil, err
	}

	xfsInfo := &collect.XFSInfo{}
	if err := yaml.Unmarshal(contents, xfsInfo); err != nil {
		return nil, err
	}

	var coll resultCollector

	for _, outcome := range a.hostAnalyzer.Outcomes {
		result := &AnalyzeResult{Title: a.Title()}

		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message

				coll.push(result)
				continue
			}

			isMatch := compareHostXFSInfoToActual(outcome.Fail.When, xfsInfo)
			if isMatch {
				result.IsFail = true
				result.Message = outcome.Fail.Message

				coll.push(result)
			}
		} else if outcome.Warn != nil {
			if outcome.Warn.When == "" {
				result.IsWarn = true
				result.Message = outcome.Warn.Message

				coll.push(result)
				continue
			}

			isMatch := compareHostXFSInfoToActual(outcome.Warn.When, xfsInfo)
			if isMatch {
				result.IsWarn = true
				result.Message = outcome.Warn.Message

				coll.push(result)
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message

				coll.push(result)
				continue
			}

			isMatch := compareHostXFSInfoToActual(outcome.Pass.When, xfsInfo)
			if isMatch {
				result.IsPass = true
				result.Message = outcome.Pass.Message

				coll.push(result)
			}
		}
	}

	return coll.get(a.Title()), nil
}

func compareHostXFSInfoToActual(status string, xfsInfo *collect.XFSInfo) bool {
	if status == XFS_FTYPE_DISABLED {
		return xfsInfo.IsXFS && !xfsInfo.IsFtypeEnabled
	}
	if status == XFS_FTYPE_ENABLED {
		return xfsInfo.IsXFS && xfsInfo.IsFtypeEnabled
	}
	if status == NOT_XFS {
		return !xfsInfo.IsXFS
	}

	return false
}
