package analyzer

import (
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	kubeletv1alpha1 "k8s.io/kubelet/pkg/apis/stats/v1alpha1"
	utilptr "k8s.io/utils/ptr"
)

func TestAnalyzeNodeMetrics_findPVCUsageStats(t *testing.T) {
	tests := []struct {
		name      string
		analyzer  troubleshootv1beta2.NodeMetricsAnalyze
		summaries []kubeletv1alpha1.Summary
		want      []pvcUsageStats
		wantErr   bool
	}{
		{
			name:      "no summaries",
			summaries: []kubeletv1alpha1.Summary{},
			want:      []pvcUsageStats{},
		},
		{
			name: "one summary",
			summaries: []kubeletv1alpha1.Summary{
				{
					Pods: []kubeletv1alpha1.PodStats{
						{
							PodRef: kubeletv1alpha1.PodReference{
								Namespace: "default",
								Name:      "my-pod",
							},
							VolumeStats: []kubeletv1alpha1.VolumeStats{
								{
									Name: "volume-1",
									PVCRef: &kubeletv1alpha1.PVCReference{
										Namespace: "default",
										Name:      "my-pvc",
									},
									FsStats: kubeletv1alpha1.FsStats{
										AvailableBytes: utilptr.To(uint64(20)),
										UsedBytes:      utilptr.To(uint64(80)),
										CapacityBytes:  utilptr.To(uint64(100)),
									},
								},
							},
						},
					},
				},
			},
			want: []pvcUsageStats{
				{
					Used:    80,
					PvcName: "default/my-pvc",
				},
			},
		},
		{
			name: "one summary with namespace filter",
			analyzer: troubleshootv1beta2.NodeMetricsAnalyze{
				Filters: troubleshootv1beta2.NodeMetricsAnalyzeFilters{
					PVC: &troubleshootv1beta2.PVCRef{
						Namespace: "another-namespace",
					},
				},
			},
			summaries: []kubeletv1alpha1.Summary{
				{
					Pods: []kubeletv1alpha1.PodStats{
						{
							PodRef: kubeletv1alpha1.PodReference{
								Namespace: "default",
								Name:      "my-pod",
							},
							VolumeStats: []kubeletv1alpha1.VolumeStats{
								{
									Name: "volume-1",
									PVCRef: &kubeletv1alpha1.PVCReference{
										Namespace: "default",
										Name:      "my-pvc",
									},
									FsStats: kubeletv1alpha1.FsStats{
										AvailableBytes: utilptr.To(uint64(20)),
										UsedBytes:      utilptr.To(uint64(80)),
										CapacityBytes:  utilptr.To(uint64(100)),
									},
								},
							},
						},
					},
				},
			},
			want: []pvcUsageStats{},
		},
		{
			name: "one summary with name regex filter",
			analyzer: troubleshootv1beta2.NodeMetricsAnalyze{
				Filters: troubleshootv1beta2.NodeMetricsAnalyzeFilters{
					PVC: &troubleshootv1beta2.PVCRef{
						NameRegex: ".*other.*",
					},
				},
			},
			summaries: []kubeletv1alpha1.Summary{
				{
					Pods: []kubeletv1alpha1.PodStats{
						{
							PodRef: kubeletv1alpha1.PodReference{
								Namespace: "default",
								Name:      "my-pod",
							},
							VolumeStats: []kubeletv1alpha1.VolumeStats{
								{
									Name: "volume-1",
									PVCRef: &kubeletv1alpha1.PVCReference{
										Namespace: "default",
										Name:      "my-pvc",
									},
									FsStats: kubeletv1alpha1.FsStats{
										AvailableBytes: utilptr.To(uint64(20)),
										UsedBytes:      utilptr.To(uint64(80)),
										CapacityBytes:  utilptr.To(uint64(100)),
									},
								},
								{
									Name: "volume-1",
									PVCRef: &kubeletv1alpha1.PVCReference{
										Namespace: "default",
										Name:      "my-other-pvc",
									},
									FsStats: kubeletv1alpha1.FsStats{
										AvailableBytes: utilptr.To(uint64(25)),
										UsedBytes:      utilptr.To(uint64(75)),
										CapacityBytes:  utilptr.To(uint64(100)),
									},
								},
							},
						},
					},
				},
			},
			want: []pvcUsageStats{
				{
					Used:    75,
					PvcName: "default/my-other-pvc",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AnalyzeNodeMetrics{
				analyzer: &tt.analyzer,
			}
			got, err := a.findPVCUsageStats(tt.summaries)
			assert.Equalf(t, tt.wantErr, err != nil, "AnalyzeNodeMetrics.findPVCUsageStats() error = %v, wantErr %v", err, tt.wantErr)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAnalyzeNodeMetrics_Analyze(t *testing.T) {
	tests := []struct {
		name        string
		analyzer    troubleshootv1beta2.NodeMetricsAnalyze
		nodeMetrics string
		want        []*AnalyzeResult
		wantErr     bool
	}{
		{
			name: "no node metrics",
			analyzer: troubleshootv1beta2.NodeMetricsAnalyze{
				Filters: troubleshootv1beta2.NodeMetricsAnalyzeFilters{},
			},
			nodeMetrics: "",
			wantErr:     true,
		},
		{
			name: "invalid node metrics",
			analyzer: troubleshootv1beta2.NodeMetricsAnalyze{
				Filters: troubleshootv1beta2.NodeMetricsAnalyzeFilters{},
			},
			nodeMetrics: "invalid",
			wantErr:     true,
		},
		{
			name: "no summaries",
			analyzer: troubleshootv1beta2.NodeMetricsAnalyze{
				Filters: troubleshootv1beta2.NodeMetricsAnalyzeFilters{},
			},
			nodeMetrics: "{}",
			want:        []*AnalyzeResult{},
		},
		{
			name: "one summary with name regex filter",
			analyzer: troubleshootv1beta2.NodeMetricsAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "pvcUsedPercentage >= 75",
							Message: "PVC space usage is too high for pvcs [{{ .ConcatenatedPVCNames }}]",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "No PVCs are using more than 80% of storage",
						},
					},
				},
				Filters: troubleshootv1beta2.NodeMetricsAnalyzeFilters{
					PVC: &troubleshootv1beta2.PVCRef{
						NameRegex: ".*other.*",
					},
				},
			},
			nodeMetrics: `{
				"pods": [
				  {
					"podRef": {
					  "name": "my-pod",
					  "namespace": "my-namespace"
					},
					"volume": [
					  {
						"capacityBytes": 100,
						"usedBytes": 80,
						"pvcRef": {
						  "name": "backup-pvc",
						  "namespace": "my-namespace"
						}
					  },
					  {
						"capacityBytes": 100,
						"usedBytes": 75,
						"pvcRef": {
						  "name": "another-pvc",
						  "namespace": "my-namespace"
						}
					  },
					  {
						"capacityBytes": 100,
						"usedBytes": 80,
						"pvcRef": {
						  "name": "the-other-pvc",
						  "namespace": "my-namespace"
						}
					  },
					  {
						"capacityBytes": 100,
						"usedBytes": 65,
						"pvcRef": {
						  "name": "to-other-pvc",
						  "namespace": "my-namespace"
						}
					  }
					]
				  }
				]
			  }`,
			want: []*AnalyzeResult{
				{
					Title:   "Node Metrics",
					IsFail:  true,
					Message: "PVC space usage is too high for pvcs [my-namespace/another-pvc, my-namespace/the-other-pvc]",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AnalyzeNodeMetrics{
				analyzer: &tt.analyzer,
			}
			filesFn := func(string, []string) (map[string][]byte, error) {
				return map[string][]byte{
					"node-metrics.json": []byte(tt.nodeMetrics),
				}, nil
			}

			got, err := a.Analyze(nil, filesFn)
			assert.Equalf(t, tt.wantErr, err != nil, "AnalyzeNodeMetrics.Analyze() error = %v, wantErr %v", err, tt.wantErr)
			assert.Equal(t, tt.want, got)
		})
	}
}
