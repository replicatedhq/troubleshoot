package collect

import (
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
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
						Exclude:       multitype.BoolOrString{},
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
pwd=***HIDDEN***;
`,
			},
		},
		{
			name: "data with custom redactor at a restricted path",
			Collect: &troubleshootv1beta2.Collect{
				Data: &troubleshootv1beta2.Data{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "datacollectorname",
						Exclude:       multitype.BoolOrString{},
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
pwd=***HIDDEN***;
`,
			},
		},
		{
			name: "data with custom redactor at other path",
			Collect: &troubleshootv1beta2.Collect{
				Data: &troubleshootv1beta2.Data{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "datacollectorname",
						Exclude:       multitype.BoolOrString{},
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
pwd=***HIDDEN***;
`,
			},
		},
		{
			name: "data with custom redactor at second path",
			Collect: &troubleshootv1beta2.Collect{
				Data: &troubleshootv1beta2.Data{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "datacollectorname",
						Exclude:       multitype.BoolOrString{},
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
pwd=***HIDDEN***;
`,
			},
		},
		{
			name: "data with literal string replacer",
			Collect: &troubleshootv1beta2.Collect{
				Data: &troubleshootv1beta2.Data{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "data/collectorname",
						Exclude:       multitype.BoolOrString{},
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
pwd=***HIDDEN***;
`,
			},
		},
		{
			name: "data with custom yaml redactor",
			Collect: &troubleshootv1beta2.Collect{
				Data: &troubleshootv1beta2.Data{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "datacollectorname",
						Exclude:       multitype.BoolOrString{},
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
another line here
`,
			},
		},
		{
			name: "custom multiline redactor",
			Collect: &troubleshootv1beta2.Collect{
				Data: &troubleshootv1beta2.Data{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "datacollectorname",
						Exclude:       multitype.BoolOrString{},
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
abc
`,
			},
		},
		{
			name: "excluded data",
			Collect: &troubleshootv1beta2.Collect{
				Data: &troubleshootv1beta2.Data{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "datacollectorname",
						Exclude:       multitype.BoolOrString{Type: multitype.String, StrVal: "true"},
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
			c := &Collector{
				Collect: tt.Collect,
				Redact:  true,
			}
			got, err := c.RunCollectorSync(nil, nil, tt.Redactors)
			req.NoError(err)

			// convert to string to make differences easier to see
			toString := map[string]string{}
			for k, v := range got {
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
						Exclude:       multitype.BoolOrString{},
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
			c := &Collector{
				Collect: tt.Collect,
				Redact:  false,
			}
			got, err := c.RunCollectorSync(nil, nil, tt.Redactors)
			req.NoError(err)

			// convert to string to make differences easier to see
			toString := map[string]string{}
			for k, v := range got {
				toString[k] = string(v)
			}
			req.EqualValues(tt.want, toString)
		})
	}
}
