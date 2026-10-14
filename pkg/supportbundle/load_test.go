package supportbundle

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_ParseSupportBundle(t *testing.T) {
	tests := []struct {
		name      string
		doc       []byte
		followURI bool
		want      *troubleshootv1beta2.SupportBundle
		wantErr   bool
	}{
		{
			name: "Parse Host Collectors",
			doc: []byte(`
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: test
spec:
  hostCollectors:
    - hostOS: {}
`),
			followURI: false,
			want: &troubleshootv1beta2.SupportBundle{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "troubleshoot.sh/v1beta2",
					Kind:       "SupportBundle",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: troubleshootv1beta2.SupportBundleSpec{
					HostCollectors: []*troubleshootv1beta2.HostCollect{
						{
							HostOS: &troubleshootv1beta2.HostOS{},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Parse Collectors",
			doc: []byte(`
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: test
spec:
  collectors:
    - clusterInfo: {}
`),
			followURI: false,
			want: &troubleshootv1beta2.SupportBundle{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "troubleshoot.sh/v1beta2",
					Kind:       "SupportBundle",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: troubleshootv1beta2.SupportBundleSpec{
					Collectors: []*troubleshootv1beta2.Collect{
						{
							ClusterInfo: &troubleshootv1beta2.ClusterInfo{},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSupportBundle(tt.doc, tt.followURI)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSupportBundle() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseSupportBundle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadRedactorSpec(t *testing.T) {
	origLoadFromSecret := loadFromSecret
	defer func() { loadFromSecret = origLoadFromSecret }()

	loadFromSecret = func(namespace, secretName, key string) ([]byte, error) {
		return []byte(fmt.Sprintf("namespace=%s,secret=%s,key=%s", namespace, secretName, key)), nil
	}

	tests := []struct {
		name        string
		uri         string
		wantContent string
		wantErr     string
	}{
		{
			name:        "secret URI with default key",
			uri:         "secret/default/my-redactor",
			wantContent: "namespace=default,secret=my-redactor,key=redactor-spec",
		},
		{
			name:        "secret URI with custom key",
			uri:         "secret/default/my-redactor/custom-key",
			wantContent: "namespace=default,secret=my-redactor,key=custom-key",
		},
		{
			name:    "secret URI with too few components",
			uri:     "secret/default",
			wantErr: "must have at least 3 components",
		},
		{
			name:    "secret URI with too many components",
			uri:     "secret/default/my-redactor/custom-key/extra",
			wantErr: "must have at most 4 components",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadRedactorSpec(tt.uri)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("LoadRedactorSpec() expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("LoadRedactorSpec() error = %q, want containing %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadRedactorSpec() unexpected error = %v", err)
			}
			if string(got) != tt.wantContent {
				t.Errorf("LoadRedactorSpec() = %q, want %q", string(got), tt.wantContent)
			}
		})
	}
}
