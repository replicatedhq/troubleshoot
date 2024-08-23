package collect

import (
	"fmt"
	"reflect"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/utils/ptr"
)

func TestGetPercentileIndex(t *testing.T) {
	tests := []struct {
		length int
		p      float64
		answer int
	}{
		{
			length: 2,
			p:      0.49,
			answer: 0,
		},
		{
			length: 2,
			p:      0.5,
			answer: 0,
		},
		{
			length: 2,
			p:      0.51,
			answer: 1,
		},
		{
			length: 100,
			p:      0.01,
			answer: 0,
		},
		{
			length: 100,
			p:      0.99,
			answer: 98,
		},
		{
			length: 100,
			p:      0.995,
			answer: 99,
		},
		{
			length: 10000,
			p:      0.999,
			answer: 9989,
		},
	}
	for _, test := range tests {
		name := fmt.Sprintf("(%f, %d) == %d", test.p, test.length, test.answer)
		t.Run(name, func(t *testing.T) {
			output := getPercentileIndex(test.p, test.length)
			if output != test.answer {
				t.Errorf("Got %d, want %d", output, test.answer)
			}
		})
	}
}

func Test_parseCollectorOptions(t *testing.T) {
	type args struct {
		hostCollector *troubleshootv1beta2.FilesystemPerformance
	}
	tests := []struct {
		name        string
		args        args
		wantCommand []string
		wantOptions *FioJobOptions
		wantErr     bool
	}{
		{
			name: "Happy spec",
			args: args{
				hostCollector: &troubleshootv1beta2.FilesystemPerformance{
					HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
						CollectorName: "fsperf",
					},
					OperationSizeBytes:          1024,
					Directory:                   "/var/lib/etcd",
					FileSize:                    "22Mi",
					Sync:                        true,
					Datasync:                    true,
					Timeout:                     "120",
					EnableBackgroundIOPS:        true,
					BackgroundIOPSWarmupSeconds: 10,
					BackgroundWriteIOPS:         100,
					BackgroundReadIOPS:          100,
					BackgroundWriteIOPSJobs:     1,
					BackgroundReadIOPSJobs:      1,
				},
			},
			wantCommand: []string{
				"fio",
				"--name=fsperf",
				"--bs=1024",
				"--directory=/var/lib/etcd",
				"--rw=write",
				"--ioengine=sync",
				"--fdatasync=1",
				"--size=23068672",
				"--runtime=120",
				"--output-format=json",
			},
			wantOptions: &FioJobOptions{
				RW:        "write",
				IOEngine:  "sync",
				FDataSync: "1",
				Directory: "/var/lib/etcd",
				Size:      "23068672",
				BS:        "1024",
				Name:      "fsperf",
				RunTime:   "120",
			},
			wantErr: false,
		},
		{
			name: "Disable runtime",
			args: args{
				hostCollector: &troubleshootv1beta2.FilesystemPerformance{
					HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
						CollectorName: "fsperf",
					},
					OperationSizeBytes:          1024,
					Directory:                   "/var/lib/etcd",
					FileSize:                    "22Mi",
					Sync:                        true,
					Datasync:                    true,
					Timeout:                     "120",
					EnableBackgroundIOPS:        true,
					BackgroundIOPSWarmupSeconds: 10,
					BackgroundWriteIOPS:         100,
					BackgroundReadIOPS:          100,
					BackgroundWriteIOPSJobs:     1,
					BackgroundReadIOPSJobs:      1,
					RunTime:                     ptr.To("0"),
				},
			},
			wantCommand: []string{
				"fio",
				"--name=fsperf",
				"--bs=1024",
				"--directory=/var/lib/etcd",
				"--rw=write",
				"--ioengine=sync",
				"--fdatasync=1",
				"--size=23068672",
				"--output-format=json",
			},
			wantOptions: &FioJobOptions{
				RW:        "write",
				IOEngine:  "sync",
				FDataSync: "1",
				Directory: "/var/lib/etcd",
				Size:      "23068672",
				BS:        "1024",
				Name:      "fsperf",
				RunTime:   "",
			},
			wantErr: false,
		},
		{
			name: "Empty spec fails",
			args: args{
				hostCollector: &troubleshootv1beta2.FilesystemPerformance{
					HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
						CollectorName: "fsperf",
					},
				},
			},
			wantCommand: nil,
			wantOptions: nil,
			wantErr:     true,
		},
		{
			name: "Invalid filesize",
			args: args{
				hostCollector: &troubleshootv1beta2.FilesystemPerformance{
					HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
						CollectorName: "fsperf",
					},
					OperationSizeBytes: 1024,
					Directory:          "/var/lib/etcd",
					FileSize:           "abcd",
					Sync:               true,
					Datasync:           true,
					Timeout:            "120",
				},
			},
			wantCommand: nil,
			wantOptions: nil,
			wantErr:     true,
		},
		{
			name: "invalid path parameter",
			args: args{
				hostCollector: &troubleshootv1beta2.FilesystemPerformance{
					HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
						CollectorName: "fsperf",
					},
					OperationSizeBytes: 1024,
					Directory:          "",
					FileSize:           "22Mi",
					Sync:               true,
					Datasync:           true,
					Timeout:            "120",
				},
			},
			wantCommand: nil,
			wantOptions: nil,
			wantErr:     true,
		},
		{
			name: "embedded cluster spec",
			args: args{
				hostCollector: &troubleshootv1beta2.FilesystemPerformance{
					HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
						CollectorName: "fsperf",
					},
					OperationSizeBytes: 2300,
					Directory:          "/var/lib/k0s/etcd",
					FileSize:           "22Mi",
					Sync:               true,
					Datasync:           true,
					Timeout:            "120",
					RunTime:            ptr.To("0"),
				},
			},
			wantCommand: []string{
				"fio",
				"--name=fsperf",
				"--bs=2300",
				"--directory=/var/lib/k0s/etcd",
				"--rw=write",
				"--ioengine=sync",
				"--fdatasync=1",
				"--size=23068672",
				"--output-format=json",
			},
			wantOptions: &FioJobOptions{
				RW:        "write",
				IOEngine:  "sync",
				FDataSync: "1",
				Directory: "/var/lib/k0s/etcd",
				Size:      "23068672",
				BS:        "2300",
				Name:      "fsperf",
				RunTime:   "",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCommand, gotOptions, err := parseCollectorOptions(tt.args.hostCollector)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCollectorOptions() error = %v, wantErr %v", err, tt.wantErr)
			} else {
				if !reflect.DeepEqual(gotCommand, tt.wantCommand) {
					t.Errorf("parseCollectorOptions() got command = %v, want %v", gotCommand, tt.wantCommand)
				}
				if !reflect.DeepEqual(gotOptions, tt.wantOptions) {
					t.Errorf("parseCollectorOptions() got options = %v, want %v", gotOptions, tt.wantOptions)
				}
			}
		})
	}
}

func Test_getFioRuntime(t *testing.T) {
	type args struct {
		runTime *string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "nil runTime should use default",
			args: args{
				runTime: nil,
			},
			want:    "120",
			wantErr: false,
		},
		{
			name: "empty runTime should disable",
			args: args{
				runTime: ptr.To(""),
			},
			want:    "",
			wantErr: false,
		},
		{
			name: "invalid runTime should return error",
			args: args{
				runTime: ptr.To("abc"),
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "valid runTime should return value",
			args: args{
				runTime: ptr.To("30"),
			},
			want:    "30",
			wantErr: false,
		},
		{
			name: "0 runTime should disable",
			args: args{
				runTime: ptr.To("0"),
			},
			want:    "",
			wantErr: false,
		},
		{
			name: "negative runTime should disable",
			args: args{
				runTime: ptr.To("-1"),
			},
			want:    "",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getFioRuntime(tt.args.runTime)
			if (err != nil) != tt.wantErr {
				t.Errorf("getFioRuntime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getFioRuntime() = %v, want %v", got, tt.want)
			}
		})
	}
}
