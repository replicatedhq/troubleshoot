package collect

import (
	"context"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestCollectNodeMetrics_constructNodesMap(t *testing.T) {
	tests := []struct {
		name        string
		objectMetas []metav1.ObjectMeta
		collector   troubleshootv1beta2.NodeMetrics
		want        map[string]string
	}{
		{
			name: "default collector no nodes",
			want: map[string]string{},
		},
		{
			name: "default collector one node",
			objectMetas: []metav1.ObjectMeta{
				{
					Name: "node1",
				},
			},
			want: map[string]string{
				"node1": "/api/v1/nodes/node1/proxy/stats/summary",
			},
		},
		{
			name: "collector with node list picking one node",
			objectMetas: []metav1.ObjectMeta{
				{
					Name: "node1",
				},
				{
					Name: "node2",
				},
			},
			collector: troubleshootv1beta2.NodeMetrics{
				NodeNames: []string{"node2"},
			},
			want: map[string]string{
				"node2": "/api/v1/nodes/node2/proxy/stats/summary",
			},
		},
		{
			name: "collector with selector picking one node",
			objectMetas: []metav1.ObjectMeta{
				{
					Name: "node1",
					Labels: map[string]string{
						"hostname": "node1.example.com",
					},
				},
				{
					Name: "node2",
				},
			},
			collector: troubleshootv1beta2.NodeMetrics{
				Selector: []string{"hostname=node1.example.com"},
			},
			want: map[string]string{
				"node1": "/api/v1/nodes/node1/proxy/stats/summary",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := testclient.NewSimpleClientset()
			ctx := context.Background()
			collector := tt.collector
			c := &CollectNodeMetrics{
				Collector: &collector,
				Client:    client,
				Context:   ctx,
			}

			for _, objectMeta := range tt.objectMetas {
				_, err := client.CoreV1().Nodes().Create(ctx, &v1.Node{
					ObjectMeta: objectMeta,
				}, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			got := c.constructNodesMap()
			assert.Equalf(t, tt.want, got, "constructNodesMap() = %v, want %v", got, tt.want)
		})
	}
}
