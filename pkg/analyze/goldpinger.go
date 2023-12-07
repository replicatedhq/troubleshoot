package analyzer

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type AnalyzeGoldpinger struct {
	analyzer *troubleshootv1beta2.GoldpingerAnalyze
}

type checkAllOutput struct {
	Hosts []struct {
		HostIP  string `json:"hostIP"`
		PodIP   string `json:"podIP"`
		PodName string `json:"podName"`
	} `json:"hosts"`
	Responses map[string]struct {
		HostIP   string `json:"hostIP"`
		PodIP    string `json:"podIP"`
		OK       bool   `json:"OK"`
		Response struct {
			PodResults map[string]struct {
				HostIP   string `json:"hostIP"`
				OK       bool   `json:"OK"`
				PingTime string `json:"pingTime"`
				PodIP    string `json:"podIP"`
				Response struct {
					BootTime string `json:"boot_time"`
				} `json:"response"`
				Error          string `json:"error"`
				ResponseTimeMS int    `json:"response-time-ms"`
				StatusCode     int    `json:"status-code"`
			} `json:"podResults"`
		} `json:"response"`
	} `json:"responses"`
}

func (a *AnalyzeGoldpinger) Title() string {
	title := a.analyzer.CheckName
	if title == "" {
		title = a.collectorName()
	}

	return title
}

func (a *AnalyzeGoldpinger) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeGoldpinger) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	collected, err := getFile(a.analyzer.FileName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read collected file name: %q", a.analyzer.FileName)
	}

	var cao checkAllOutput
	err = json.Unmarshal(collected, &cao)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal collected goldpinger output")
	}

	return a.podPingsAnalysis(&cao), nil
}

func (a *AnalyzeGoldpinger) collectorName() string {
	if a.analyzer.CollectorName != "" {
		return a.analyzer.CollectorName
	}
	return "goldpinger"
}

func (a *AnalyzeGoldpinger) podPingsAnalysis(cao *checkAllOutput) []*AnalyzeResult {
	results := []*AnalyzeResult{}

	for _, host := range cao.Hosts {
		// Check if the pod from a host has any ping errors from other pods
		targetPod := host.PodName
		pingsSucceeded := true
		for srcPod, resp := range cao.Responses {
			res := &AnalyzeResult{
				IconKey: "kubernetes",
				Strict:  a.analyzer.Strict.BoolOrDefaultFalse(),
			}

			// Get ping result for the pod
			podResult, ok := resp.Response.PodResults[targetPod]
			if !ok {
				// Pod not found in ping results from the source pod
				res.IsWarn = true
				res.Title = fmt.Sprintf("Missing ping results for %q pod", targetPod)
				res.Message = fmt.Sprintf("Ping result for %q pod from %q pod is missing", targetPod, srcPod)
				pingsSucceeded = false

				results = append(results, res)
				continue
			}

			if !podResult.OK {
				// Ping was not successful
				res.IsFail = true
				res.Title = fmt.Sprintf("Ping from %q pod to %q pod failed", srcPod, targetPod)
				res.Message = fmt.Sprintf("Ping error: %s", podResult.Error)
				pingsSucceeded = false

				results = append(results, res)
				continue
			}
		}

		// If all pings succeeded, add a pass result
		if pingsSucceeded {
			results = append(results, &AnalyzeResult{
				IconKey: "kubernetes",
				Strict:  a.analyzer.Strict.BoolOrDefaultFalse(),
				IsPass:  true,
				Title:   fmt.Sprintf("Pings to %q pod from all other pods succeeded", targetPod),
				Message: fmt.Sprintf("Pings to %q pod from all other pods succeeded", targetPod),
			})
		}
	}

	return results
}
