package analyzer

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeCertificates struct {
	analyzer *troubleshootv1beta2.CertificatesAnalyze
}

func (a *AnalyzeCertificates) Title() string {
	title := a.analyzer.CheckName
	if title == "" {
		return "Cerfiticates Verification"
	}

	return title
}

func (a *AnalyzeCertificates) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeCertificates) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	result, err := a.AnalyzeCertificates(a.analyzer, getFile)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (a *AnalyzeCertificates) AnalyzeCertificates(analyzer *troubleshootv1beta2.CertificatesAnalyze, getCollectedFileContents func(string) ([]byte, error)) ([]*AnalyzeResult, error) {
	certificatesInfo, err := getCollectedFileContents("certificates/certificates.json")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get contents of certificates.json")
	}

	collectorCertificates := []collect.CertCollection{}
	if err := json.Unmarshal(certificatesInfo, &collectorCertificates); err != nil {
		return nil, errors.Wrap(err, "failed to parse certificates.json")
	}

	return a.analyzeAnalyzeCertificatesResult(collectorCertificates, analyzer.Outcomes)
}

func (a *AnalyzeCertificates) analyzeAnalyzeCertificatesResult(certifcates []collect.CertCollection, outcomes []*troubleshootv1beta2.Outcome) ([]*AnalyzeResult, error) {
	var results []*AnalyzeResult

	for _, cert := range certifcates {
		for _, certChain := range cert.CertificateChain {

			when := ""
			message := ""
			source := ""

			if cert.Source.ConfigMapName != "" {
				source = fmt.Sprintf("obtained from %s configmap within %s namespace", cert.Source.ConfigMapName, cert.Source.Namespace)
			}

			if cert.Source.SecretName != "" {
				source = fmt.Sprintf("obtained from %s secret within %s namespace", cert.Source.SecretName, cert.Source.Namespace)
			}

			for _, outcome := range outcomes {
				result := AnalyzeResult{
					Title: a.Title(),
				}

				if outcome.Fail != nil {
					result.IsFail = true
					when = outcome.Fail.When
					message = outcome.Fail.Message
				} else if outcome.Warn != nil {
					result.IsWarn = true
					when = outcome.Warn.When
					message = outcome.Warn.Message
				} else if outcome.Pass != nil {
					result.IsPass = true
					when = outcome.Pass.When
					message = outcome.Pass.Message
				} else {
					return nil, errors.New("empty outcome")
				}

				if result.IsPass && certChain.IsValid {
					result.Message = fmt.Sprintf("%s %s, %s", certChain.CertName, message, source)
					results = append(results, &result)
				}

				if result.IsFail && !certChain.IsValid {
					result.Message = fmt.Sprintf("%s %s, %s", certChain.CertName, message, source)
					results = append(results, &result)
				}

				if result.IsWarn && certChain.IsValid {
					warnDate, _ := regexp.Compile(`notAfter \< Today \+ (\d+) days`)
					warnMatch := warnDate.FindStringSubmatch(when)
					if warnMatch != nil {
						warnMatchDays, _ := strconv.Atoi(warnMatch[1])
						targetTime := time.Now().AddDate(0, 0, warnMatchDays)
						if targetTime.After(certChain.NotAfter) {
							result.Message = fmt.Sprintf("%s %s in %d days, %s, ", certChain.CertName, message, warnMatchDays, source)
							results = append(results, &result)
						}
					}
				}
			}
		}
	}

	return results, nil
}
