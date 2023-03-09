package collect

import (
	"reflect"
	"testing"
)

func Test_parseLsblkDeviceOutput(t *testing.T) {
	tests := []struct {
		name    string
		output  []byte
		want    []BlockDeviceInfo
		wantErr bool
	}{
		{
			name:   "ubuntu 20.04",
			output: []byte(`NAME="sdb" KNAME="sdb" PKNAME="" TYPE="disk" MAJ:MIN="8:16" SIZE="107374182400" FSTYPE="" MOUNTPOINT="" SERIAL="persistent-disk-1" RO="0" RM="0"`),
			want: []BlockDeviceInfo{
				{
					Name:             "sdb",
					KernelName:       "sdb",
					ParentKernelName: "",
					Type:             "disk",
					Major:            8,
					Minor:            16,
					Size:             107374182400,
					FilesystemType:   "",
					Mountpoint:       "",
					Serial:           "persistent-disk-1",
					ReadOnly:         false,
					Removable:        false,
				},
			},
			wantErr: false,
		},
		{
			name:   "rhel 9",
			output: []byte(`NAME="sdb" KNAME="sdb" PKNAME="" TYPE="disk" MAJ_MIN="8:16" SIZE="107374182400" FSTYPE="" MOUNTPOINT="" SERIAL="persistent-disk-1" RO="0" RM="0"`),
			want: []BlockDeviceInfo{
				{
					Name:             "sdb",
					KernelName:       "sdb",
					ParentKernelName: "",
					Type:             "disk",
					Major:            8,
					Minor:            16,
					Size:             107374182400,
					FilesystemType:   "",
					Mountpoint:       "",
					Serial:           "persistent-disk-1",
					ReadOnly:         false,
					Removable:        false,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLsblkOutput(tt.output)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseLsblkOutput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseLsblkOutput() = %v, want %v", got, tt.want)
			}
		})
	}
}
