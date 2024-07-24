package collect

import (
	"bytes"
	"reflect"
	"testing"
)

func Test_parseV1ControllerNames(t *testing.T) {
	tests := []struct {
		name       string
		subsystems []byte
		want       []string
		wantErr    bool
	}{
		{
			name:       "no controllers",
			subsystems: []byte(""),
			want:       []string{},
			wantErr:    false,
		},
		{
			name: "multiple enabled controllers",
			subsystems: []byte(
				`
#subsys_name	hierarchy	num_cgroups	enabled
cpuset  5       1
cpu     9       41      1
cpuacct 9       41      1
blkio   11      41      1
memory  8       95      0
devices 13      41      1
freezer 3       2       1
net_cls 4       1       1
perf_event      2       1       0
net_prio        4       1       0
hugetlb 12      1       1
pids    10      46      1
rdma    6       1       0
misc    7       1       0
`),
			want:    []string{"cpu", "cpuacct", "blkio", "devices", "freezer", "net_cls", "hugetlb", "pids"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.subsystems)

			got, err := parseV1ControllerNames(r)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseV1ControllerNames() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseV1ControllerNames() = %v, want %v", got, tt.want)
			}
		})
	}
}
