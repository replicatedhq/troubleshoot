package analyzer

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

// relevant fields from https://github.com/weaveworks/weave/blob/e3712152d2a0fe3bc998964c948e45bdf8ff6144/prog/weaver/http.go#L295
type WeaveReport struct {
	Router WeaveRouter
	IPAM   WeaveIPAM
}

type WeaveRouter struct {
	NickName    string // this is the hostname
	Connections []WeaveConnection
}

type WeaveIPAM struct {
	RangeNumIPs      int
	ActiveIPs        int
	PendingAllocates []string
}

type WeaveConnection struct {
	State string
	Info  string
	Attrs WeaveAttributes
}

type WeaveAttributes struct {
	Encrypted bool   `json:"encrypted"`
	MTU       int    `json:"mtu"`
	Name      string `json:"name"`
}

type AnalyzeWeaveReport struct {
	analyzer *troubleshootv1beta2.WeaveReportAnalyze
}

func (a *AnalyzeWeaveReport) Title() string {
	return "Weave CNI"
}

func (a *AnalyzeWeaveReport) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeWeaveReport) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	results, err := analyzeWeaveReport(a.analyzer, findFiles)
	if err != nil {
		return nil, err
	}
	for i := range results {
		results[i].Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	}
	return results, nil
}

func analyzeWeaveReport(analyzer *troubleshootv1beta2.WeaveReportAnalyze, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	excludeFiles := []string{}
	files, err := findFiles(analyzer.ReportFileGlob, excludeFiles)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find weave report files in %q", analyzer.ReportFileGlob)
	}

	if len(files) == 0 {
		return nil, nil
	}

	reports := map[string]WeaveReport{}

	for name, file := range files {
		report := WeaveReport{}

		if err := json.Unmarshal(file, &report); err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal weave report json from %s", name)
		}

		reports[report.Router.NickName] = report
	}

	results := []*AnalyzeResult{}

	if result := analyzeWeaveIPAMPools(reports); result != nil {
		results = append(results, result)
	}

	if result := analyzeWeavePendingAllocation(reports); result != nil {
		results = append(results, result)
	}

	if result := analyzeWeaveConnections(reports); result != nil {
		results = append(results, result)
	}

	if len(results) == 0 {
		results = append(results, &AnalyzeResult{
			Title:   "Weave Report",
			IsPass:  true,
			Message: "No issues detected in weave report",
		})
	}

	return results, nil
}

func analyzeWeaveIPAMPools(reports map[string]WeaveReport) *AnalyzeResult {
	for _, report := range reports {
		if result := analyzeWeaveIPAMPool(&report); result != nil {
			return result
		}
	}

	return nil
}

func analyzeWeaveIPAMPool(report *WeaveReport) *AnalyzeResult {
	if report.IPAM.RangeNumIPs == 0 {
		return nil
	}

	ipsUsed := float64(report.IPAM.ActiveIPs) / float64(report.IPAM.RangeNumIPs)
	if ipsUsed < 0.85 {
		return nil
	}

	return &AnalyzeResult{
		Title:   "Available Pod IPs",
		IsWarn:  true,
		Message: fmt.Sprintf("%d of %d total available IPs have been assigned", report.IPAM.ActiveIPs, report.IPAM.RangeNumIPs),
	}
}

func analyzeWeavePendingAllocation(reports map[string]WeaveReport) *AnalyzeResult {
	for _, report := range reports {
		if len(report.IPAM.PendingAllocates) > 0 {
			return &AnalyzeResult{
				Title:   "Pending IP Allocation",
				IsWarn:  true,
				Message: "Waiting for IPs to become available",
			}
		}
	}

	return nil
}

// Get the peer hostname for logging purposes from a string like: "encrypted   fastdp 1a:5b:a9:53:2b:11(areed-aka-kkz0)"
var weaveConnectionInfoPeerRegex = regexp.MustCompile(`\(([^)]+)\)$`)

func parseWeaveConnectionInfoHostname(info string) string {
	matches := weaveConnectionInfoPeerRegex.FindStringSubmatch(info)
	if len(matches) == 2 {
		return matches[1]
	}
	return ""
}

func analyzeWeaveConnections(reports map[string]WeaveReport) *AnalyzeResult {
	for host, report := range reports {
		for _, connection := range report.Router.Connections {
			// older versions of weave show connection to self as failed
			if strings.HasPrefix(connection.Info, "cannot connect to ourself") {
				continue
			}

			peer := parseWeaveConnectionInfoHostname(connection.Info)
			if peer == "" {
				peer = "peer"
			}

			if connection.State != "established" {
				return &AnalyzeResult{
					Title:   "Weave Inter-Node Connections",
					IsWarn:  true,
					Message: fmt.Sprintf("Connection from %s to %s is %s", host, peer, connection.State),
				}
			}

			if connection.Attrs.Name != "" && connection.Attrs.Name != "fastdp" {
				return &AnalyzeResult{
					Title:   "Weave Inter-Node Connections",
					IsWarn:  true,
					Message: fmt.Sprintf("Connection from %s to %s protocol is %q, not fastdp", host, peer, connection.Attrs.Name),
				}
			}
		}
	}

	return nil
}
