package analyzer

import (
	"reflect"
	"testing"

	"github.com/blang/semver/v4"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
)

func Test_analyzeClusterVersionResult(t *testing.T) {
	outcomes := []*troubleshootv1beta2.Outcome{
		{
			Fail: &troubleshootv1beta2.SingleOutcome{
				When:    "< 1.13.0",
				Message: "Sentry requires at Kubernetes 1.13.0 or later, and recommends 1.15.0.",
				URI:     "https://www.kubernetes.io",
			},
		},
		{
			Warn: &troubleshootv1beta2.SingleOutcome{
				When:    "< 1.15.0",
				Message: "Your cluster meets the minimum version of Kubernetes, but we recommend you update to 1.15.0 or later.",
				URI:     "https://www.kubernetes.io",
			},
		},
		{
			Pass: &troubleshootv1beta2.SingleOutcome{
				Message: "Your cluster meets the recommended and required versions of Kubernetes.",
			},
		},
	}

	type args struct {
		k8sVersion semver.Version
		outcomes   []*troubleshootv1beta2.Outcome
		checkName  string
	}
	tests := []struct {
		name    string
		args    args
		want    *AnalyzeResult
		wantErr bool
	}{
		{
			name: "fail",
			args: args{
				k8sVersion: semver.MustParse("1.12.5"),
				outcomes:   outcomes,
				checkName:  "Check Fail",
			},
			want: &AnalyzeResult{
				IsFail:  true,
				Title:   "Check Fail",
				Message: "Sentry requires at Kubernetes 1.13.0 or later, and recommends 1.15.0.",
				URI:     "https://www.kubernetes.io",
				IconKey: "kubernetes_cluster_version",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/kubernetes.svg?w=16&h=16",
			},
		},
		{
			name: "warn",
			args: args{
				k8sVersion: semver.MustParse("1.14.3"),
				outcomes:   outcomes,
				checkName:  "Check Warn",
			},
			want: &AnalyzeResult{
				IsWarn:  true,
				Title:   "Check Warn",
				Message: "Your cluster meets the minimum version of Kubernetes, but we recommend you update to 1.15.0 or later.",
				URI:     "https://www.kubernetes.io",
				IconKey: "kubernetes_cluster_version",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/kubernetes.svg?w=16&h=16",
			},
		},
		{
			name: "fallthrough",
			args: args{
				k8sVersion: semver.MustParse("1.17.0"),
				outcomes:   outcomes,
				checkName:  "Check Pass",
			},
			want: &AnalyzeResult{
				IsPass:  true,
				Title:   "Check Pass",
				Message: "Your cluster meets the recommended and required versions of Kubernetes.",
				IconKey: "kubernetes_cluster_version",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/kubernetes.svg?w=16&h=16",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := analyzeClusterVersionResult(tt.args.k8sVersion, tt.args.outcomes, tt.args.checkName)
			if (err != nil) != tt.wantErr {
				t.Errorf("analyzeClusterVersionResult() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("analyzeClusterVersionResult() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseVersionString(t *testing.T) {
	tests := []struct {
		name       string
		rawVersion string
		want       semver.Version
		wantErr    bool
	}{
		{
			name:       "valid version",
			rawVersion: "1.17.0",
			want:       semver.MustParse("1.17.0"),
		},
		{
			name:       "valid version with v prefix",
			rawVersion: "v1.17.0",
			want:       semver.MustParse("1.17.0"),
		},
		{
			name:       "invalid version",
			rawVersion: "v1.17",
			want:       semver.Version{},
			wantErr:    true,
		},
		{
			name:       "EKS version",
			rawVersion: "v1.25.16-eks-8cb36c9",
			want:       semver.MustParse("1.25.16"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVersionString(tt.rawVersion)
			assert.Equal(t, tt.wantErr, err != nil, "parseVersionString() error = %v, wantErr %v", err, tt.wantErr)
			assert.Equal(t, tt.want, got, "parseVersionString() = %v, want %v", got, tt.want)
		})
	}
}
