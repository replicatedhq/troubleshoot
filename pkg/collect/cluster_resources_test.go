package collect

import (
	"reflect"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_SelectCRDVersionByPriority(t *testing.T) {
	assert.Equal(t, "v1alpha3", selectCRDVersionByPriority([]string{"v1alpha2", "v1alpha3"}))
	assert.Equal(t, "v1alpha3", selectCRDVersionByPriority([]string{"v1alpha3", "v1alpha2"}))
	assert.Equal(t, "v1", selectCRDVersionByPriority([]string{"v1alpha2", "v1alpha3", "v1"}))
	assert.Equal(t, "v1", selectCRDVersionByPriority([]string{"v1", "v1alpha2", "v1alpha3"}))
}

func TestClusterResources_Merge(t *testing.T) {
	tests := []struct {
		name       string
		Collectors []troubleshootv1beta2.Collect
		want       *CollectClusterResources
	}{
		{
			name: "single cluster resources collector with multiple unique namespaces",
			Collectors: []troubleshootv1beta2.Collect{
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Namespaces: []string{"hello", "hello2"},
					},
				},
			},
			want: &CollectClusterResources{
				Collector: &troubleshootv1beta2.ClusterResources{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "collectorname",
					},
					Namespaces: []string{"hello", "hello2"},
				},
			},
		},
		{
			name: "multiple cluster resources collectors with unique namespaces",
			Collectors: []troubleshootv1beta2.Collect{
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Namespaces: []string{"hello"},
					},
				},
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Namespaces: []string{"hello2"},
					},
				},
			},
			want: &CollectClusterResources{
				Collector: &troubleshootv1beta2.ClusterResources{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "collectorname",
					},
					Namespaces: []string{"hello", "hello2"},
				},
			},
		},
		{
			name: "multiple cluster resources collectors with duplicate namespaces",
			Collectors: []troubleshootv1beta2.Collect{
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Namespaces: []string{"hello"},
					},
				},
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Namespaces: []string{"hello2"},
					},
				},
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Namespaces: []string{"hello"},
					},
				},
			},
			want: &CollectClusterResources{
				Collector: &troubleshootv1beta2.ClusterResources{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "collectorname",
					},
					Namespaces: []string{"hello", "hello2"},
				},
			},
		},
		{
			name: "multiple cluster resource collectors with a empty string namespace provided",
			Collectors: []troubleshootv1beta2.Collect{
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Namespaces: []string{"hello"},
					},
				},
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Namespaces: []string{"hello2"},
					},
				},
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Namespaces: []string{""},
					},
				},
			},
			want: &CollectClusterResources{
				Collector: &troubleshootv1beta2.ClusterResources{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "collectorname",
					},
					Namespaces: nil,
				},
			},
		},
		{
			name: "multiple cluster resource collectors with a nil namespace provided",
			Collectors: []troubleshootv1beta2.Collect{
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Namespaces: []string{"hello"},
					},
				},
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Namespaces: []string{"hello2"},
					},
				},
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Namespaces: nil,
					},
				},
			},
			want: &CollectClusterResources{
				Collector: &troubleshootv1beta2.ClusterResources{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "collectorname",
					},
					Namespaces: nil,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			var mergedCollectors []Collector
			allCollectors := make(map[reflect.Type][]Collector)
			collectorType := reflect.TypeOf(CollectClusterResources{})

			for _, collector := range tt.Collectors {
				collectorInterface, _ := GetCollector(&collector, "", "", nil, nil, nil)
				if mergeCollector, ok := collectorInterface.(MergeableCollector); ok {
					allCollectors[collectorType] = append(allCollectors[collectorType], mergeCollector)
				}
			}

			for _, collectors := range allCollectors {
				if mergeCollector, ok := collectors[0].(MergeableCollector); ok {
					mergedCollectors, _ = mergeCollector.Merge(collectors)
				}
			}

			clusterResourceCollector, _ := mergedCollectors[0].(*CollectClusterResources)

			req.EqualValues(tt.want, clusterResourceCollector)
		})
	}
}
