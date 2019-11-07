package analyzer

import (
	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
)

func statefulsetStatus(analyzer *troubleshootv1beta1.StatefulsetStatus, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {

	return nil, errors.New("not implemented")
}
