package analyzer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/klog/v2"
	kubeletv1alpha1 "k8s.io/kubelet/pkg/apis/stats/v1alpha1"
)

type AnalyzeNodeMetrics struct {
	analyzer *troubleshootv1beta2.NodeMetricsAnalyze
}

type nodeMetricsComparisonResults struct {
	PVC pvcTemplateData
}

type pvcTemplateData struct {
	UsedPercentage    float64
	ConcatenatedNames string
	Names             []string
}

type pvcUsageStats struct {
	PvcName string
	Used    float64
}

func (a *AnalyzeNodeMetrics) Title() string {
	title := a.analyzer.CheckName
	if title == "" {
		title = a.analyzer.CollectorName
	}
	if title == "" {
		title = "Node Metrics"
	}

	return title
}

func (a *AnalyzeNodeMetrics) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeNodeMetrics) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	// Gather all collected node-metrics files
	collected, err := findFiles(filepath.Join("node-metrics", "*.json"), nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read collected pods")
	}

	// Unmarshal all collected node-metrics files
	summaries := []kubeletv1alpha1.Summary{}
	for _, fileContent := range collected {
		summary := kubeletv1alpha1.Summary{}
		if err := json.Unmarshal(fileContent, &summary); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal node metrics")
		}

		summaries = append(summaries, summary)
	}

	// Run through all outcomes to generate results
	result, err := a.compareCollectedMetricsWithOutcomes(summaries)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compare node metrics with outcomes")
	}
	if result == nil {
		return []*AnalyzeResult{}, nil
	}
	result.Strict = a.analyzer.Strict.BoolOrDefaultFalse()

	return []*AnalyzeResult{result}, nil
}

func (a *AnalyzeNodeMetrics) compareCollectedMetricsWithOutcomes(summaries []kubeletv1alpha1.Summary) (*AnalyzeResult, error) {
	for _, outcome := range a.analyzer.Outcomes {
		result := &AnalyzeResult{
			Title: a.Title(),
		}

		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			} else {
				isMatch, out, err := a.compareNodeMetricConditionalsToStats(outcome.Fail.When, summaries)
				if err != nil {
					return nil, errors.Wrap(err, "failed to compare node metrics conditional with summary stats")
				}

				if isMatch {
					result.IsFail = true
					result.Message = renderTemplate(outcome.Fail.Message, out)
					result.URI = outcome.Fail.URI

					return result, nil
				}
			}

		} else if outcome.Warn != nil {
			if outcome.Warn.When == "" {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return result, nil
			} else {
				isMatch, out, err := a.compareNodeMetricConditionalsToStats(outcome.Warn.When, summaries)
				if err != nil {
					return nil, errors.Wrap(err, "failed to compare node metrics conditional with summary stats")
				}

				if isMatch {
					result.IsWarn = true
					result.Message = renderTemplate(outcome.Warn.Message, out)
					result.URI = outcome.Warn.URI

					return result, nil
				}
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return result, nil
			} else {
				isMatch, out, err := a.compareNodeMetricConditionalsToStats(outcome.Pass.When, summaries)
				if err != nil {
					return nil, errors.Wrap(err, "failed to compare node metrics conditional with summary stats")
				}

				if isMatch {
					result.IsPass = true
					result.Message = renderTemplate(outcome.Pass.Message, out)
					result.URI = outcome.Pass.URI

					return result, nil
				}
			}
		}
	}

	return nil, nil
}

func (a *AnalyzeNodeMetrics) findPVCUsageStats(summaries []kubeletv1alpha1.Summary) ([]pvcUsageStats, error) {
	// We just collect usage percentages for now. If other stats are needed, we can add them.
	stats := []pvcUsageStats{}
	var nameRegex *regexp.Regexp
	var ns string
	var err error

	pvcFilter := a.analyzer.Filters.PVC
	if pvcFilter != nil {
		if pvcFilter.NameRegex != "" {
			nameRegex, err = regexp.Compile(pvcFilter.NameRegex)
			if err != nil {
				return nil, errors.Wrap(err, "failed to compile PVC name regex")
			}
		}

		ns = pvcFilter.Namespace
	}

	// Analyze PVCs
	for _, summary := range summaries {
		for i := range summary.Pods {
			pod := summary.Pods[i]
			if ns != "" && ns != pod.PodRef.Namespace {
				klog.V(2).Infof("Skipping pvcs in %s/%s pod due to namespace filter", pod.PodRef.Namespace, pod.PodRef.Name)
				continue
			}

			for j := range pod.VolumeStats {
				volume := pod.VolumeStats[j]

				// This is a persistent volume
				if volume.PVCRef != nil {
					if nameRegex != nil && !nameRegex.MatchString(volume.PVCRef.Name) {
						klog.V(2).Infof("Skipping pvc %s/%s due to name regex filter", volume.PVCRef.Namespace, volume.PVCRef.Name)
						continue
					}

					// Calculate the usage
					pvcName := fmt.Sprintf("%s/%s", volume.PVCRef.Namespace, volume.PVCRef.Name)

					used := volume.UsedBytes
					capacity := volume.CapacityBytes
					if used != nil && capacity != nil {
						pvcUsedPercentage := float64(*used) / float64(*capacity) * 100
						stats = append(stats, pvcUsageStats{
							PvcName: pvcName,
							Used:    pvcUsedPercentage,
						})
						klog.V(2).Infof("PVC usage for %s: %0.2f%%", pvcName, pvcUsedPercentage)
					} else {
						klog.V(2).Infof("Missing capacity or used bytes for PVC %s", pvcName)
					}
				}
			}
		}
	}

	return stats, nil
}

// compareNodeMetricConditionalsToStats compares the conditional with the collected node metrics
// and returns true if the conditional is met. At the moment we only support comparing PVC usage
func (a *AnalyzeNodeMetrics) compareNodeMetricConditionalsToStats(conditional string, summaries []kubeletv1alpha1.Summary) (bool, nodeMetricsComparisonResults, error) {
	klog.V(2).Infof("Comparing node metrics with conditional: %s", conditional)
	parts := strings.Split(strings.TrimSpace(conditional), " ")
	out := nodeMetricsComparisonResults{}

	if len(parts) != 3 {
		return false, out, errors.New("unable to parse conditional")
	}

	switch parts[0] {
	case "pvcUsedPercentage":
		// e.g pvcUsedPercentage >= 50.4

		klog.V(2).Infof("Analyzing volume usage stats for PVCs")

		op, err := ParseComparisonOperator(parts[1])
		if err != nil {
			return false, out, errors.Wrap(err, "failed to parse comparison operator")
		}

		expected, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return false, out, errors.Wrap(err, "failed to parse bool")
		}

		// Pick all PVCs from all summaries. Filters will be applied here
		pvcUsageStats, err := a.findPVCUsageStats(summaries)
		if err != nil {
			return false, out, errors.Wrap(err, "failed to find PVC usage stats")
		}
		matchedPVCs := []string{}

		for _, pvcUsage := range pvcUsageStats {
			value := pvcUsage.Used
			switch op {
			case Equal:
				if value == expected {
					matchedPVCs = append(matchedPVCs, pvcUsage.PvcName)
				}
			case NotEqual:
				if value != expected {
					matchedPVCs = append(matchedPVCs, pvcUsage.PvcName)
				}
			case LessThan:
				if value < expected {
					matchedPVCs = append(matchedPVCs, pvcUsage.PvcName)
				}
			case GreaterThan:
				if value > expected {
					matchedPVCs = append(matchedPVCs, pvcUsage.PvcName)
				}
			case LessThanOrEqual:
				if value <= expected {
					matchedPVCs = append(matchedPVCs, pvcUsage.PvcName)
				}
			case GreaterThanOrEqual:
				if value >= expected {
					matchedPVCs = append(matchedPVCs, pvcUsage.PvcName)
				}
			}
		}

		// Concatenate all matched PVC names
		out.PVC = pvcTemplateData{
			Names:             matchedPVCs,
			ConcatenatedNames: strings.Join(matchedPVCs, ", "),
		}
		return len(matchedPVCs) > 0, out, nil
	}

	return false, out, errors.New("unknown node metric conditional")
}

func renderTemplate(tmpMsg string, data any) string {
	if data == nil {
		return tmpMsg
	}

	t, err := template.New("msg").Parse(tmpMsg)
	if err != nil {
		klog.V(2).Infof("Failed to parse template: %s", err)
		return tmpMsg
	}

	var m bytes.Buffer
	err = t.Execute(&m, data)
	if err != nil {
		klog.V(2).Infof("Failed to execute template: %s", err)
		return tmpMsg
	}

	return m.String()
}
