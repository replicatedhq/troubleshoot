package collect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollector_RunCollectorSyncNoRedact(t *testing.T) {
	tests := []struct {
		name      string
		Collect   *troubleshootv1beta2.Collect
		Redactors []*troubleshootv1beta2.Redact
		want      map[string]string
	}{
		{
			name: "data with custom redactor",
			Collect: &troubleshootv1beta2.Collect{
				Data: &troubleshootv1beta2.Data{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "datacollectorname",
					},
					Name: "data",
					Data: `abc 123
another line here
pwd=somethinggoeshere;`,
				},
			},
			Redactors: []*troubleshootv1beta2.Redact{
				{
					Name: "",
					Removals: troubleshootv1beta2.Removals{
						Values: nil,
						Regex: []troubleshootv1beta2.Regex{
							{Redactor: `abc`},
							{Redactor: `(another)(?P<mask>.*)(here)`},
						},
					},
				},
			},
			want: map[string]string{
				"data/datacollectorname": ` 123
another***HIDDEN***here
pwd=***HIDDEN***;`,
			},
		},
		{
			name: "data with custom redactor at a restricted path",
			Collect: &troubleshootv1beta2.Collect{
				Data: &troubleshootv1beta2.Data{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "datacollectorname",
					},
					Name: "data",
					Data: `abc 123
another line here
pwd=somethinggoeshere;`,
				},
			},
			Redactors: []*troubleshootv1beta2.Redact{
				{
					Name: "",
					FileSelector: troubleshootv1beta2.FileSelector{
						File: "data/*",
					},
					Removals: troubleshootv1beta2.Removals{
						Values: nil,
						Regex: []troubleshootv1beta2.Regex{
							{Redactor: `(another)(?P<mask>.*)(here)`},
						},
					},
				},
			},
			want: map[string]string{
				"data/datacollectorname": `abc 123
another***HIDDEN***here
pwd=***HIDDEN***;`,
			},
		},
		{
			name: "data with custom redactor at other path",
			Collect: &troubleshootv1beta2.Collect{
				Data: &troubleshootv1beta2.Data{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "datacollectorname",
					},
					Name: "data",
					Data: `abc 123
another line here
pwd=somethinggoeshere;`,
				},
			},
			Redactors: []*troubleshootv1beta2.Redact{
				{
					Name: "",
					FileSelector: troubleshootv1beta2.FileSelector{
						File: "notdata/*",
					},
					Removals: troubleshootv1beta2.Removals{
						Values: nil,
						Regex: []troubleshootv1beta2.Regex{
							{Redactor: `(another)(?P<mask>.*)(here)`},
						},
					},
				},
			},
			want: map[string]string{
				"data/datacollectorname": `abc 123
another line here
pwd=***HIDDEN***;`,
			},
		},
		{
			name: "data with custom redactor at second path",
			Collect: &troubleshootv1beta2.Collect{
				Data: &troubleshootv1beta2.Data{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "datacollectorname",
					},
					Name: "data",
					Data: `abc 123
another line here
pwd=somethinggoeshere;`,
				},
			},
			Redactors: []*troubleshootv1beta2.Redact{
				{
					Name: "",
					FileSelector: troubleshootv1beta2.FileSelector{
						Files: []string{
							"notData/*",
							"data/*",
						},
					},
					Removals: troubleshootv1beta2.Removals{
						Values: nil,
						Regex: []troubleshootv1beta2.Regex{
							{Redactor: `(another)(?P<mask>.*)(here)`},
						},
					},
				},
			},
			want: map[string]string{
				"data/datacollectorname": `abc 123
another***HIDDEN***here
pwd=***HIDDEN***;`,
			},
		},
		{
			name: "data with literal string replacer",
			Collect: &troubleshootv1beta2.Collect{
				Data: &troubleshootv1beta2.Data{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "data/collectorname",
					},
					Name: "data",
					Data: `abc 123
another line here
pwd=somethinggoeshere;`,
				},
			},
			Redactors: []*troubleshootv1beta2.Redact{
				{
					Name: "",
					FileSelector: troubleshootv1beta2.FileSelector{
						Files: []string{
							"data/*/*",
						},
					},
					Removals: troubleshootv1beta2.Removals{
						Values: []string{
							`abc`,
							`123`,
							`another`,
						},
					},
				},
			},
			want: map[string]string{
				"data/data/collectorname": `***HIDDEN*** ***HIDDEN***
***HIDDEN*** line here
pwd=***HIDDEN***;`,
			},
		},
		{
			name: "data with custom yaml redactor",
			Collect: &troubleshootv1beta2.Collect{
				Data: &troubleshootv1beta2.Data{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "datacollectorname",
					},
					Name: "data",
					Data: `abc 123
another line here`,
				},
			},
			Redactors: []*troubleshootv1beta2.Redact{
				{
					Removals: troubleshootv1beta2.Removals{
						YamlPath: []string{
							`abc`,
						},
					},
				},
			},
			want: map[string]string{
				"data/datacollectorname": `abc 123
another line here`,
			},
		},
		{
			name: "custom multiline redactor",
			Collect: &troubleshootv1beta2.Collect{
				Data: &troubleshootv1beta2.Data{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "datacollectorname",
					},
					Name: "data",
					Data: `xyz123
abc
xyz123
xyz123
abc`,
				},
			},
			Redactors: []*troubleshootv1beta2.Redact{
				{
					Removals: troubleshootv1beta2.Removals{
						Regex: []troubleshootv1beta2.Regex{
							{
								Selector: "abc",
								Redactor: "xyz(123)",
							},
						},
					},
				},
			},
			want: map[string]string{
				"data/datacollectorname": `xyz123
abc
123
xyz123
abc`,
			},
		},
		{
			name: "excluded data",
			Collect: &troubleshootv1beta2.Collect{
				Data: &troubleshootv1beta2.Data{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "datacollectorname",
						Exclude:       multitype.FromString("true"),
					},
					Name: "data",
					Data: `abc 123
another line here
pwd=somethinggoeshere;`,
				},
			},
			Redactors: []*troubleshootv1beta2.Redact{},
			want:      map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			var result CollectorResult

			collector, _ := GetCollector(tt.Collect, "", "", nil, nil, nil)
			regCollector, _ := collector.(Collector)

			if excluded, err := regCollector.IsExcluded(); !excluded {
				req.NoError(err)

				result, err = regCollector.Collect(nil)
				req.NoError(err)

				err = RedactResult("", result, tt.Redactors)

				req.NoError(err)
			}

			// convert to string to make differences easier to see
			toString := map[string]string{}
			for k, v := range result {
				toString[k] = string(v)
			}
			req.EqualValues(tt.want, toString)
		})
	}
}

func TestCollector_RunCollectorSync(t *testing.T) {
	tests := []struct {
		name      string
		Collect   *troubleshootv1beta2.Collect
		Redactors []*troubleshootv1beta2.Redact
		want      map[string]string
	}{
		{
			name: "data with custom redactor - but redaction disabled",
			Collect: &troubleshootv1beta2.Collect{
				Data: &troubleshootv1beta2.Data{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "datacollectorname",
					},
					Name: "data",
					Data: `abc 123
another line here
pwd=somethinggoeshere;`,
				},
			},
			Redactors: []*troubleshootv1beta2.Redact{
				{
					Name: "",
					Removals: troubleshootv1beta2.Removals{
						Values: nil,
						Regex: []troubleshootv1beta2.Regex{
							{Redactor: `abc`},
							{Redactor: `(another)(?P<mask>.*)(here)`},
						},
					},
				},
			},
			want: map[string]string{
				"data/datacollectorname": `abc 123
another line here
pwd=somethinggoeshere;`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			var result CollectorResult

			collector, _ := GetCollector(tt.Collect, "", "", nil, nil, nil)
			regCollector, _ := collector.(Collector)

			if excluded, err := regCollector.IsExcluded(); !excluded {
				req.NoError(err)

				result, err = regCollector.Collect(nil)
				req.NoError(err)
			}

			// convert to string to make differences easier to see
			toString := map[string]string{}
			for k, v := range result {
				toString[k] = string(v)
			}
			req.EqualValues(tt.want, toString)
		})
	}
}

func TestCollector_DedupCollectors(t *testing.T) {
	tests := []struct {
		name       string
		Collectors []*troubleshootv1beta2.Collect
		want       []*troubleshootv1beta2.Collect
	}{
		{
			name: "multiple cluster info",
			Collectors: []*troubleshootv1beta2.Collect{
				{
					ClusterInfo: &troubleshootv1beta2.ClusterInfo{},
				},
				{
					ClusterInfo: &troubleshootv1beta2.ClusterInfo{},
				},
			},
			want: []*troubleshootv1beta2.Collect{
				{
					ClusterInfo: &troubleshootv1beta2.ClusterInfo{},
				},
			},
		},
		{
			name: "multiple cluster resources with matching namespace lists",
			Collectors: []*troubleshootv1beta2.Collect{
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						Namespaces: []string{"namespace1", "namespace2"},
					},
				},
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						Namespaces: []string{"namespace1", "namespace2"},
					},
				},
			},
			want: []*troubleshootv1beta2.Collect{
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						Namespaces: []string{"namespace1", "namespace2"},
					},
				},
			},
		},
		{
			name: "multiple cluster resources with unnique namespace lists",
			Collectors: []*troubleshootv1beta2.Collect{
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						Namespaces: []string{"namespace1", "namespace2"},
					},
				},
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						Namespaces: []string{"namespace1000", "namespace2000"},
					},
				},
			},
			want: []*troubleshootv1beta2.Collect{
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						Namespaces: []string{"namespace1", "namespace2"},
					},
				},
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						Namespaces: []string{"namespace1000", "namespace2000"},
					},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DedupCollectors(tc.Collectors)
			assert.Equal(t, tc.want, got)
		})
	}
}
func TestEnsureCopyLast(t *testing.T) {
	tests := []struct {
		name          string
		allCollectors []Collector
		want          []Collector
	}{
		{
			name: "no copy collectors",
			allCollectors: []Collector{
				&CollectClusterInfo{},
				&CollectClusterResources{},
			},
			want: []Collector{
				&CollectClusterInfo{},
				&CollectClusterResources{},
			},
		},
		{
			name: "only copy collectors",
			allCollectors: []Collector{
				&CollectCopy{},
				&CollectCopy{},
			},
			want: []Collector{
				&CollectCopy{},
				&CollectCopy{},
			},
		},
		{
			name: "mixed collectors",
			allCollectors: []Collector{
				&CollectClusterInfo{},
				&CollectCopy{},
				&CollectClusterResources{},
				&CollectCopy{},
			},
			want: []Collector{
				&CollectClusterInfo{},
				&CollectClusterResources{},
				&CollectCopy{},
				&CollectCopy{},
			},
		},
		{
			name: "copy collectors at the beginning",
			allCollectors: []Collector{
				&CollectCopy{},
				&CollectCopy{},
				&CollectClusterInfo{},
				&CollectClusterResources{},
			},
			want: []Collector{
				&CollectClusterInfo{},
				&CollectClusterResources{},
				&CollectCopy{},
				&CollectCopy{},
			},
		},
		{
			name: "copy collectors at the end",
			allCollectors: []Collector{
				&CollectClusterInfo{},
				&CollectClusterResources{},
				&CollectCopy{},
				&CollectCopy{},
			},
			want: []Collector{
				&CollectClusterInfo{},
				&CollectClusterResources{},
				&CollectCopy{},
				&CollectCopy{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EnsureCopyLast(tt.allCollectors)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWriteSkippedCollectors(t *testing.T) {
	tests := []struct {
		name        string
		skipped     []SkippedCollector
		bundlePath  string
		useTempDir  bool
		wantInMap   bool
		wantOnDisk  bool
		wantEntries []SkippedCollector
	}{
		{
			name:       "empty skipped list does nothing",
			skipped:    nil,
			bundlePath: "",
			wantInMap:  false,
		},
		{
			name: "in-memory only when bundlePath is empty",
			skipped: []SkippedCollector{
				{Collector: "clusterResources", Reason: "excluded", Timestamp: "2026-01-01T00:00:00Z"},
			},
			bundlePath: "",
			wantInMap:  true,
			wantOnDisk: false,
			wantEntries: []SkippedCollector{
				{Collector: "clusterResources", Reason: "excluded", Timestamp: "2026-01-01T00:00:00Z"},
			},
		},
		{
			name: "writes to disk when bundlePath is set",
			skipped: []SkippedCollector{
				{Collector: "clusterResources", Reason: "excluded", Timestamp: "2026-01-01T00:00:00Z"},
				{Collector: "logs", Reason: "insufficient RBAC permissions", Errors: []string{"pods is forbidden"}, Timestamp: "2026-01-01T00:00:01Z"},
			},
			useTempDir: true,
			wantInMap:  true,
			wantOnDisk: true,
			wantEntries: []SkippedCollector{
				{Collector: "clusterResources", Reason: "excluded", Timestamp: "2026-01-01T00:00:00Z"},
				{Collector: "logs", Reason: "insufficient RBAC permissions", Errors: []string{"pods is forbidden"}, Timestamp: "2026-01-01T00:00:01Z"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundlePath := tt.bundlePath
			if tt.useTempDir {
				bundlePath = t.TempDir()
			}

			result := CollectorResult{}
			WriteSkippedCollectors(tt.skipped, result, bundlePath)

			if !tt.wantInMap {
				assert.Empty(t, result)
				return
			}

			// Verify in-memory entry exists
			if bundlePath == "" {
				// In-memory mode: data is stored in the map
				data, ok := result["skipped-collectors.json"]
				require.True(t, ok, "skipped-collectors.json should be in result map")
				require.NotNil(t, data)

				var got []SkippedCollector
				require.NoError(t, json.Unmarshal(data, &got))
				assert.Equal(t, tt.wantEntries, got)
			} else {
				// On-disk mode: map entry exists with nil value, file is on disk
				_, ok := result["skipped-collectors.json"]
				require.True(t, ok, "skipped-collectors.json should be in result map")

				diskData, err := os.ReadFile(filepath.Join(bundlePath, "skipped-collectors.json"))
				require.NoError(t, err)

				var got []SkippedCollector
				require.NoError(t, json.Unmarshal(diskData, &got))
				assert.Equal(t, tt.wantEntries, got)
			}
		})
	}
}
