package analyzer

import (
	"bytes"
	"path"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

func cephStatus(analyzer *troubleshootv1beta2.CephStatusAnalyze, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	fileName := path.Join(collect.GetCephCollectorFilepath(analyzer.CollectorName, analyzer.Namespace), "status.txt")
	collected, err := getCollectedFileContents(fileName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read collected ceph status")
	}

	title := analyzer.CheckName
	if title == "" {
		title = "Ceph Status"
	}

	analyzeResult := &AnalyzeResult{
		Title:   title,
		IconKey: "", // TODO
		IconURI: "", // TODO
	}

	// TODO: add more details to message
	switch {
	case bytes.Contains(collected, []byte(" HEALTH_OK")):
		analyzeResult.IsPass = true
		analyzeResult.Message = "Ceph is healthy"
	case bytes.Contains(collected, []byte(" HEALTH_WARN")):
		analyzeResult.IsWarn = true
		analyzeResult.Message = "Ceph status is HEALTH_WARN"
		analyzeResult.URI = "https://rook.io/docs/rook/v1.4/ceph-common-issues.html"
	case bytes.Contains(collected, []byte(" HEALTH_ERR")):
		analyzeResult.IsFail = true
		analyzeResult.Message = "Ceph status is HEALTH_ERR"
		analyzeResult.URI = "https://rook.io/docs/rook/v1.4/ceph-common-issues.html"
	default:
		return nil, errors.New("health not found in collected ceph status")
	}

	return analyzeResult, nil
}
