package supportbundle

import (
	"reflect"
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
