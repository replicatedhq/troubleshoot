package analyzer

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeHostCertificatesCollection struct {
	hostAnalyzer *troubleshootv1beta2.HostCertificatesCollectionAnalyze
}

func (a *AnalyzeHostCertificatesCollection) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Host Cerfiticates Collection")
}

func (a *AnalyzeHostCertificatesCollection) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostCertificatesCollection) Analyze(getCollectedFileContents func(string) ([]byte, error)) ([]*AnalyzeResult, error) {
	hostAnalyzer := a.hostAnalyzer

	collectorName := hostAnalyzer.CollectorName
	if collectorName == "" {
		collectorName = "certificatesCollection"
	}
	name := filepath.Join("host-collectors/certificatesCollection", collectorName+".json")

	certificatesInfo, err := getCollectedFileContents(name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get contents of certificatesCollection.json")
	}

	collectorCertificates := []collect.HostCertificatesCollection{}
	if err := json.Unmarshal(certificatesInfo, &collectorCertificates); err != nil {
		return nil, errors.Wrap(err, "failed to parse certificatesCollection.json")
	}

	var coll resultCollector

	for _, cert := range collectorCertificates {

		source := ""

		if cert.CertificatePath != "" {
			source = fmt.Sprintf("obtained from %s", cert.CertificatePath)
		}

		if cert.Message == collect.CertMissing {
			// return the result immediately if the certificate is missing
			coll.push(&AnalyzeResult{
				Title:   a.Title(),
				IsFail:  true,
				Message: fmt.Sprintf("Certificate is missing, cannot be %s", source),
			})
		} else {
			results, err := a.analyzeHostAnalyzeCertificatesResult(cert.CertificateChain, hostAnalyzer.Outcomes, source)
			if err != nil {
				return nil, err
			}
			for _, result := range results {
				coll.push(result)
			}
		}
	}

	return coll.get(a.Title()), nil
}

func (a *AnalyzeHostCertificatesCollection) analyzeHostAnalyzeCertificatesResult(certificateChains []collect.ParsedCertificate, outcomes []*troubleshootv1beta2.Outcome, source string) ([]*AnalyzeResult, error) {
	var coll resultCollector
	var passResults []*AnalyzeResult
	when := ""
	message := ""

	for _, certChain := range certificateChains {
		for _, outcome := range outcomes {
			result := &AnalyzeResult{
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
				result.Message = fmt.Sprintf("%s, %s", message, source)
				// if the certificate is valid, we need to wait for the warning check whether the certificate is going to expire
				passResults = append(passResults, result)
			}

			if result.IsFail && !certChain.IsValid {
				result.Message = fmt.Sprintf("%s, %s", message, source)
				// return the result immediately if the certificate is invalid
				coll.push(result)
			}

			if result.IsWarn && certChain.IsValid {
				warnDate, _ := regexp.Compile(`notAfter \< Today \+ (\d+) days`)
				warnMatch := warnDate.FindStringSubmatch(when)
				if warnMatch != nil {
					warnMatchDays, err := strconv.Atoi(warnMatch[1])
					if err != nil {
						return nil, errors.Wrap(err, "failed to convert string to integer")
					}

					targetTime := time.Now().AddDate(0, 0, warnMatchDays)

					if targetTime.After(certChain.NotAfter) {
						result.Message = fmt.Sprintf("%s in %d days, %s", message, warnMatchDays, source)
						// discard passResults if the certificate is going to expire in certain days
						passResults = []*AnalyzeResult{}
						coll.push(result)
					}
				}
			}
		}
		// append passResults if the certificate is valid and not going to expire in certain days
		for _, passResult := range passResults {
			coll.push(passResult)
		}
	}
	return coll.results, nil
}
