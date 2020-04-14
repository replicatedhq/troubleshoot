package collect

import (
	"testing"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
	"github.com/stretchr/testify/require"
	"go.undefinedlabs.com/scopeagent"
)

func TestCollector_RunCollectorSyncNoRedact(t *testing.T) {
	tests := []struct {
		name    string
		Collect *troubleshootv1beta1.Collect
		want    map[string]string
	}{
		{
			name: "data with custom redactor",
			Collect: &troubleshootv1beta1.Collect{
				Data: &troubleshootv1beta1.Data{
					CollectorMeta: troubleshootv1beta1.CollectorMeta{
						CollectorName: "datacollectorname",
						Redactors: []*troubleshootv1beta1.Redact{
							{
								Name:   "",
								File:   "",
								Files:  nil,
								Values: nil,
								Regex: []string{
									`abc`,
									`(another)(?P<mask>.*)(here)`,
								},
							},
						},
						Exclude: multitype.BoolOrString{},
					},
					Name: "data",
					Data: `abc 123
another line here
pwd=somethinggoeshere;`,
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
			Collect: &troubleshootv1beta1.Collect{
				Data: &troubleshootv1beta1.Data{
					CollectorMeta: troubleshootv1beta1.CollectorMeta{
						CollectorName: "datacollectorname",
						Redactors: []*troubleshootv1beta1.Redact{
							{
								Name:   "",
								File:   "data/*",
								Values: nil,
								Regex: []string{
									`(another)(?P<mask>.*)(here)`,
								},
							},
						},
						Exclude: multitype.BoolOrString{},
					},
					Name: "data",
					Data: `abc 123
another line here
pwd=somethinggoeshere;`,
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
			Collect: &troubleshootv1beta1.Collect{
				Data: &troubleshootv1beta1.Data{
					CollectorMeta: troubleshootv1beta1.CollectorMeta{
						CollectorName: "datacollectorname",
						Redactors: []*troubleshootv1beta1.Redact{
							{
								Name:   "",
								File:   "notdata/*",
								Values: nil,
								Regex: []string{
									`(another)(?P<mask>.*)(here)`,
								},
							},
						},
						Exclude: multitype.BoolOrString{},
					},
					Name: "data",
					Data: `abc 123
another line here
pwd=somethinggoeshere;`,
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
			Collect: &troubleshootv1beta1.Collect{
				Data: &troubleshootv1beta1.Data{
					CollectorMeta: troubleshootv1beta1.CollectorMeta{
						CollectorName: "datacollectorname",
						Redactors: []*troubleshootv1beta1.Redact{
							{
								Name: "",
								Files: []string{
									"notData/*",
									"data/*",
								},
								Values: nil,
								Regex: []string{
									`(another)(?P<mask>.*)(here)`,
								},
							},
						},
						Exclude: multitype.BoolOrString{},
					},
					Name: "data",
					Data: `abc 123
another line here
pwd=somethinggoeshere;`,
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
			Collect: &troubleshootv1beta1.Collect{
				Data: &troubleshootv1beta1.Data{
					CollectorMeta: troubleshootv1beta1.CollectorMeta{
						CollectorName: "data/collectorname",
						Redactors: []*troubleshootv1beta1.Redact{
							{
								Name: "",
								Files: []string{
									"data/*/*",
								},
								Values: []string{
									`abc`,
									`123`,
									`another`,
								},
							},
						},
						Exclude: multitype.BoolOrString{},
					},
					Name: "data",
					Data: `abc 123
another line here
pwd=somethinggoeshere;`,
				},
			},
			want: map[string]string{
				"data/data/collectorname": `***HIDDEN*** ***HIDDEN***
***HIDDEN*** line here
pwd=***HIDDEN***;
`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scopetest := scopeagent.StartTest(t)
			defer scopetest.End()

			req := require.New(t)
			c := &Collector{
				Collect: tt.Collect,
				Redact:  true,
			}
			got, err := c.RunCollectorSync(nil)
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
		name    string
		Collect *troubleshootv1beta1.Collect
		want    map[string]string
	}{
		{
			name: "data with custom redactor - but redaction disabled",
			Collect: &troubleshootv1beta1.Collect{
				Data: &troubleshootv1beta1.Data{
					CollectorMeta: troubleshootv1beta1.CollectorMeta{
						CollectorName: "datacollectorname",
						Redactors: []*troubleshootv1beta1.Redact{
							{
								Name:   "",
								File:   "",
								Files:  nil,
								Values: nil,
								Regex: []string{
									`abc`,
									`(another)(?P<mask>.*)(here)`,
								},
							},
						},
						Exclude: multitype.BoolOrString{},
					},
					Name: "data",
					Data: `abc 123
another line here
pwd=somethinggoeshere;`,
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
			scopetest := scopeagent.StartTest(t)
			defer scopetest.End()

			req := require.New(t)
			c := &Collector{
				Collect: tt.Collect,
				Redact:  false,
			}
			got, err := c.RunCollectorSync(nil)
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
