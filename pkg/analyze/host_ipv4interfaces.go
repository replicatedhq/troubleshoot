package analyzer

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type AnalyzeHostIPV4Interfaces struct {
	hostAnalyzer *troubleshootv1beta2.IPV4InterfacesAnalyze
}

func (a *AnalyzeHostIPV4Interfaces) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "IPv4 Interfaces")
}

func (a *AnalyzeHostIPV4Interfaces) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostIPV4Interfaces) Analyze(getCollectedFileContents func(string) ([]byte, error)) ([]*AnalyzeResult, error) {
	hostAnalyzer := a.hostAnalyzer

	contents, err := getCollectedFileContents("system/ipv4Interfaces.json")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get collected file")
	}

	var ipv4Interfaces []net.Interface
	if err := json.Unmarshal(contents, &ipv4Interfaces); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal ipv4Interfaces")
	}

	var coll resultCollector

	for _, outcome := range hostAnalyzer.Outcomes {
		result := &AnalyzeResult{Title: a.Title()}

		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				coll.push(result)
			}

			isMatch, err := compareHostIPV4InterfacesConditionalToActual(outcome.Fail.When, ipv4Interfaces)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %s", outcome.Fail.When)
			}

			if isMatch {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				coll.push(result)
			}
		} else if outcome.Warn != nil {
			if outcome.Warn.When == "" {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				coll.push(result)
			}

			isMatch, err := compareHostIPV4InterfacesConditionalToActual(outcome.Warn.When, ipv4Interfaces)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %s", outcome.Warn.When)
			}

			if isMatch {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				coll.push(result)
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				coll.push(result)
			}

			isMatch, err := compareHostIPV4InterfacesConditionalToActual(outcome.Pass.When, ipv4Interfaces)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to compare %s", outcome.Pass.When)
			}

			if isMatch {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				coll.push(result)
			}
		}
	}

	return coll.get(a.Title()), nil
}

func compareHostIPV4InterfacesConditionalToActual(conditional string, ipv4Interfaces []net.Interface) (res bool, err error) {
	parts := strings.Split(conditional, " ")
	if len(parts) != 3 {
		return false, fmt.Errorf("Expected exactly 3 parts in conditional, got %d", len(parts))
	}

	keyword := parts[0]
	operator := parts[1]
	desired := parts[2]

	if keyword != "count" {
		return false, fmt.Errorf(`Only supported keyword is "count", got %q`, keyword)
	}

	desiredInt, err := strconv.ParseInt(desired, 10, 64)
	if err != nil {
		return false, errors.Wrapf(err, "failed to parse %q as int", desired)
	}

	actualCount := len(ipv4Interfaces)

	switch operator {
	case "<":
		return actualCount < int(desiredInt), nil
	case "<=":
		return actualCount <= int(desiredInt), nil
	case ">":
		return actualCount > int(desiredInt), nil
	case ">=":
		return actualCount >= int(desiredInt), nil
	case "=", "==", "===":
		return actualCount == int(desiredInt), nil
	}

	return false, fmt.Errorf("Unknown operator %q. Supported operators are: <, <=, ==, >=, >", operator)
}
