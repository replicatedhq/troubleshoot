package preflight

import (
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
)

var (
	analyzeMetaStrictFalseStr = troubleshootv1beta2.AnalyzeMeta{
		Strict: &multitype.BoolOrString{
			StrVal: "false",
		},
	}
	analyzeMetaStrictInvalidStr = troubleshootv1beta2.AnalyzeMeta{
		Strict: &multitype.BoolOrString{
			StrVal: "invalid",
		},
	}
	analyzeMetaStrictTrueStr = troubleshootv1beta2.AnalyzeMeta{
		Strict: &multitype.BoolOrString{
			StrVal: "true",
		},
	}
	analyzeMetaStrictFalseBool = troubleshootv1beta2.AnalyzeMeta{
		Strict: &multitype.BoolOrString{
			Type:    multitype.Bool,
			BoolVal: false,
		},
	}
	analyzeMetaStrictTrueBool = troubleshootv1beta2.AnalyzeMeta{
		Strict: &multitype.BoolOrString{
			Type:    multitype.Bool,
			BoolVal: true,
		},
	}
	analyzeMetaStrictFalseInt = troubleshootv1beta2.AnalyzeMeta{
		Strict: &multitype.BoolOrString{
			StrVal: "0",
		},
	}
	analyzeMetaStrictTrueInt = troubleshootv1beta2.AnalyzeMeta{
		Strict: &multitype.BoolOrString{
			Type:   multitype.String,
			StrVal: "1",
		},
	}
	analyzeMetaStrictTrueExcludeTrue = troubleshootv1beta2.AnalyzeMeta{
		Exclude: &multitype.BoolOrString{
			Type:    multitype.Bool,
			BoolVal: true,
		},
		Strict: &multitype.BoolOrString{
			Type:    multitype.Bool,
			BoolVal: true,
		},
	}
)

func TestHasStrictAnalyzers(t *testing.T) {

	tests := []struct {
		name      string
		preflight *troubleshootv1beta2.Preflight
		want      bool
		wantErr   bool
	}{
		{
			name:      "expect false when preflight is nil",
			preflight: nil,
			want:      false,
			wantErr:   false,
		}, {
			name: "expect false when preflight spec is empty",
			preflight: &troubleshootv1beta2.Preflight{
				Spec: troubleshootv1beta2.PreflightSpec{},
			},
			want:    false,
			wantErr: false,
		}, {
			name: "expect false when preflight spec's analyzers is nil",
			preflight: &troubleshootv1beta2.Preflight{
				Spec: troubleshootv1beta2.PreflightSpec{
					Analyzers: nil,
				},
			},
			want:    false,
			wantErr: false,
		}, {
			name: "expect false when preflight spec's analyzers is empty",
			preflight: &troubleshootv1beta2.Preflight{
				Spec: troubleshootv1beta2.PreflightSpec{
					Analyzers: []*troubleshootv1beta2.Analyze{},
				},
			},
			want:    false,
			wantErr: false,
		}, {
			name: "expect false when preflight spec's analyzer has nil analyzer",
			preflight: &troubleshootv1beta2.Preflight{
				Spec: troubleshootv1beta2.PreflightSpec{
					Analyzers: []*troubleshootv1beta2.Analyze{
						{
							ClusterVersion: nil,
						},
					},
				},
			},
			want:    false,
			wantErr: false,
		}, {
			name: "expect false when preflight spec's analyzer has analyzer with strict str: false",
			preflight: &troubleshootv1beta2.Preflight{
				Spec: troubleshootv1beta2.PreflightSpec{
					Analyzers: []*troubleshootv1beta2.Analyze{
						{
							ClusterVersion: &troubleshootv1beta2.ClusterVersion{AnalyzeMeta: analyzeMetaStrictFalseStr},
						},
					},
				},
			},
			want:    false,
			wantErr: false,
		}, {
			name: "expect false when preflight spec's analyzer has analyzer with strict bool: false",
			preflight: &troubleshootv1beta2.Preflight{
				Spec: troubleshootv1beta2.PreflightSpec{
					Analyzers: []*troubleshootv1beta2.Analyze{
						{
							ClusterVersion: &troubleshootv1beta2.ClusterVersion{AnalyzeMeta: analyzeMetaStrictFalseBool},
						},
					},
				},
			},
			want:    false,
			wantErr: false,
		}, {
			name: "expect false when preflight spec's analyzer has analyzer with strict str: invalid",
			preflight: &troubleshootv1beta2.Preflight{
				Spec: troubleshootv1beta2.PreflightSpec{
					Analyzers: []*troubleshootv1beta2.Analyze{
						{
							ClusterVersion: &troubleshootv1beta2.ClusterVersion{AnalyzeMeta: analyzeMetaStrictInvalidStr},
						},
					},
				},
			},
			want:    false,
			wantErr: false,
		}, {
			name: "expect true when preflight spec's analyzer has analyzer with strict str: true",
			preflight: &troubleshootv1beta2.Preflight{
				Spec: troubleshootv1beta2.PreflightSpec{
					Analyzers: []*troubleshootv1beta2.Analyze{
						{
							ClusterVersion: &troubleshootv1beta2.ClusterVersion{AnalyzeMeta: analyzeMetaStrictTrueStr},
						},
					},
				},
			},
			want:    true,
			wantErr: false,
		}, {
			name: "expect true when preflight spec's analyzer has analyzer with strict bool: true",
			preflight: &troubleshootv1beta2.Preflight{
				Spec: troubleshootv1beta2.PreflightSpec{
					Analyzers: []*troubleshootv1beta2.Analyze{
						{
							ClusterVersion: &troubleshootv1beta2.ClusterVersion{AnalyzeMeta: analyzeMetaStrictTrueBool},
						},
					},
				},
			},
			want:    true,
			wantErr: false,
		}, {
			name: "expect true when preflight spec's analyzer has analyzer with strict int: 1",
			preflight: &troubleshootv1beta2.Preflight{
				Spec: troubleshootv1beta2.PreflightSpec{
					Analyzers: []*troubleshootv1beta2.Analyze{
						{
							ClusterVersion: &troubleshootv1beta2.ClusterVersion{AnalyzeMeta: analyzeMetaStrictTrueInt},
						},
					},
				},
			},
			want:    true,
			wantErr: false,
		}, {
			name: "expect false when preflight spec's analyzer has analyzer with strict int: 0",
			preflight: &troubleshootv1beta2.Preflight{
				Spec: troubleshootv1beta2.PreflightSpec{
					Analyzers: []*troubleshootv1beta2.Analyze{
						{
							ClusterVersion: &troubleshootv1beta2.ClusterVersion{AnalyzeMeta: analyzeMetaStrictFalseInt},
						},
					},
				},
			},
			want:    false,
			wantErr: false,
		}, {
			name: "expect true when preflight spec's analyzer has analyzer with strict true in one of multiple analyzers",
			preflight: &troubleshootv1beta2.Preflight{
				Spec: troubleshootv1beta2.PreflightSpec{
					Analyzers: []*troubleshootv1beta2.Analyze{
						{
							ClusterVersion: &troubleshootv1beta2.ClusterVersion{AnalyzeMeta: analyzeMetaStrictFalseInt},
						}, {
							ClusterVersion: &troubleshootv1beta2.ClusterVersion{AnalyzeMeta: analyzeMetaStrictTrueBool},
						},
					},
				},
			},
			want:    true,
			wantErr: false,
		}, {
			name: "expect true when preflight spec's analyzer has analyzer with strict true in one of multiple analyzers",
			preflight: &troubleshootv1beta2.Preflight{
				Spec: troubleshootv1beta2.PreflightSpec{
					Analyzers: []*troubleshootv1beta2.Analyze{
						{
							ClusterVersion: &troubleshootv1beta2.ClusterVersion{AnalyzeMeta: analyzeMetaStrictFalseInt},
						}, {
							ClusterVersion: &troubleshootv1beta2.ClusterVersion{AnalyzeMeta: analyzeMetaStrictTrueBool},
						}, {
							ClusterVersion: &troubleshootv1beta2.ClusterVersion{AnalyzeMeta: analyzeMetaStrictInvalidStr},
						},
					},
				},
			},
			want:    true,
			wantErr: false,
		}, {
			name: "expect true when preflight spec's analyzer has analyzer with strict true in one of multiple analyzers",
			preflight: &troubleshootv1beta2.Preflight{
				Spec: troubleshootv1beta2.PreflightSpec{
					Analyzers: []*troubleshootv1beta2.Analyze{
						{
							ClusterVersion: &troubleshootv1beta2.ClusterVersion{AnalyzeMeta: analyzeMetaStrictFalseInt},
							StorageClass:   &troubleshootv1beta2.StorageClass{AnalyzeMeta: analyzeMetaStrictFalseInt},
							Secret:         &troubleshootv1beta2.AnalyzeSecret{AnalyzeMeta: analyzeMetaStrictTrueBool},
						},
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "expect false when preflight spec's analyzer has analyzer with strict true and exclude true",
			preflight: &troubleshootv1beta2.Preflight{
				Spec: troubleshootv1beta2.PreflightSpec{
					Analyzers: []*troubleshootv1beta2.Analyze{
						{
							ClusterVersion: &troubleshootv1beta2.ClusterVersion{AnalyzeMeta: analyzeMetaStrictTrueExcludeTrue},
						},
					},
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "expect true when preflight spec's analyzer has analyzer with strict true in one of multiple analyzers, but one analyzer with strict true is exclude true",
			preflight: &troubleshootv1beta2.Preflight{
				Spec: troubleshootv1beta2.PreflightSpec{
					Analyzers: []*troubleshootv1beta2.Analyze{
						{
							ClusterVersion: &troubleshootv1beta2.ClusterVersion{AnalyzeMeta: analyzeMetaStrictTrueExcludeTrue},
							StorageClass:   &troubleshootv1beta2.StorageClass{AnalyzeMeta: analyzeMetaStrictFalseInt},
							Secret:         &troubleshootv1beta2.AnalyzeSecret{AnalyzeMeta: analyzeMetaStrictTrueBool},
						},
					},
				},
			},
			want:    true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HasStrictAnalyzers(tt.preflight)
			if (err != nil) != tt.wantErr {
				t.Errorf("HasStrictAnalyzers error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("HasStrictAnalyzers() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasStrictAnalyzerFailed(t *testing.T) {
	tests := []struct {
		name            string
		preflightResult *UploadPreflightResults
		want            bool
	}{
		{
			name:            "expect true when preflightResult is nil",
			preflightResult: nil,
			want:            true,
		}, {
			name: "expect true when preflightResult.Results is nil",
			preflightResult: &UploadPreflightResults{
				Results: nil,
			},
			want: true,
		}, {
			name: "expect true when preflightResult.Results is empty",
			preflightResult: &UploadPreflightResults{
				Results: []*UploadPreflightResult{},
			},
			want: true,
		}, {
			name: "expect false when preflightResult.Results has result with strict false, IsFail false",
			preflightResult: &UploadPreflightResults{
				Results: []*UploadPreflightResult{
					{Strict: false, IsFail: false},
				},
			},
			want: false,
		}, {
			name: "expect false when preflightResult.Results has result with strict false, IsFail true",
			preflightResult: &UploadPreflightResults{
				Results: []*UploadPreflightResult{
					{Strict: false, IsFail: true},
				},
			},
			want: false,
		}, {
			name: "expect true when preflightResult.Results has result with strict true, IsFail true",
			preflightResult: &UploadPreflightResults{
				Results: []*UploadPreflightResult{
					{Strict: true, IsFail: true},
				},
			},
			want: true,
		}, {
			name: "expect true when preflightResult.Results has multiple results where atleast result has strict true, IsFail true",
			preflightResult: &UploadPreflightResults{
				Results: []*UploadPreflightResult{
					{Strict: true, IsFail: true},
					{Strict: false, IsFail: true},
					{Strict: true, IsFail: false},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasStrictAnalyzersFailed(tt.preflightResult); got != tt.want {
				t.Errorf("HasStrictAnalyzersFailed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasStrictAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		analyzer *troubleshootv1beta2.Analyze
		want     bool
		wantErr  bool
	}{
		{
			name:     "expect strict=false, err=nil when analyzer is nil",
			analyzer: nil,
			want:     false,
			wantErr:  false,
		}, {
			name:     "expect strict=false, err=nil when analyzer is empty",
			analyzer: &troubleshootv1beta2.Analyze{},
			want:     false,
			wantErr:  false,
		}, {
			name: "expect strict=false, err=nil when ClusterVersion analyzer has strict=1",
			analyzer: &troubleshootv1beta2.Analyze{
				ClusterVersion: &troubleshootv1beta2.ClusterVersion{AnalyzeMeta: analyzeMetaStrictFalseInt},
			},
			want:    false,
			wantErr: false,
		}, {
			name: "expect strict=true, err=nil when ClusterVersion analyzer has strict=true",
			analyzer: &troubleshootv1beta2.Analyze{
				ClusterVersion: &troubleshootv1beta2.ClusterVersion{AnalyzeMeta: analyzeMetaStrictTrueInt},
			},
			want:    true,
			wantErr: false,
		}, {
			name: "expect strict=false, err=nil when ClusterVersion analyzer is nil",
			analyzer: &troubleshootv1beta2.Analyze{
				ClusterVersion: nil,
			},
			want:    false,
			wantErr: false,
		}, {
			name: "expect strict=true, err=nil when one of the analyzers has strict=true",
			analyzer: &troubleshootv1beta2.Analyze{
				ClusterVersion: &troubleshootv1beta2.ClusterVersion{AnalyzeMeta: analyzeMetaStrictFalseInt},
				StorageClass:   &troubleshootv1beta2.StorageClass{AnalyzeMeta: analyzeMetaStrictFalseInt},
				Secret:         &troubleshootv1beta2.AnalyzeSecret{AnalyzeMeta: analyzeMetaStrictTrueBool},
			},
			want:    true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HasStrictAnalyzer(tt.analyzer)
			if (err != nil) != tt.wantErr {
				t.Errorf("HasStrictAnalyzer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("HasStrictAnalyzer() = %v, want %v", got, tt.want)
			}
		})
	}
}
