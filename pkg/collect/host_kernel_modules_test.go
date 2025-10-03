package collect

import (
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type mockKernelModulesCollector struct {
	result map[string]KernelModuleInfo
	err    error
}

func (m mockKernelModulesCollector) collect(kernelRelease string) (map[string]KernelModuleInfo, []byte, error) {
	if m.err != nil {
		return nil, nil, m.err
	}
	return m.result, nil, nil
}

var testKernelModuleErr = errors.New("error collecting modules")

func TestCollectHostKernelModules_Collect(t *testing.T) {
	tests := []struct {
		name          string
		hostCollector *troubleshootv1beta2.HostKernelModules
		loadable      kernelModuleCollector
		loaded        kernelModuleCollector
		want          map[string][]byte
		wantErr       bool
	}{
		{
			name: "loadable",
			loadable: mockKernelModulesCollector{
				result: map[string]KernelModuleInfo{
					"first": {
						Status: KernelModuleLoadable,
					},
					"second": {
						Status: KernelModuleLoadable,
					},
				},
			},
			loaded: mockKernelModulesCollector{},
			want: map[string][]byte{
				"host-collectors/system/kernel_modules.json": []byte("{\"first\":{\"size\":0,\"instances\":0,\"status\":\"loadable\"},\"second\":{\"size\":0,\"instances\":0,\"status\":\"loadable\"}}"),
			},
		},
		{
			name:     "loaded",
			loadable: mockKernelModulesCollector{},
			loaded: mockKernelModulesCollector{
				result: map[string]KernelModuleInfo{
					"first": {
						Status:    KernelModuleLoaded,
						Size:      10,
						Instances: 2,
					},
					"second": {
						Status: KernelModuleLoading,
					},
				},
			},
			want: map[string][]byte{
				"host-collectors/system/kernel_modules.json": []byte("{\"first\":{\"size\":10,\"instances\":2,\"status\":\"loaded\"},\"second\":{\"size\":0,\"instances\":0,\"status\":\"loading\"}}"),
			},
		},
		{
			name: "loaded and unloaded",
			loadable: mockKernelModulesCollector{
				result: map[string]KernelModuleInfo{
					"first": {
						Status: KernelModuleLoadable,
					},
					"second": {
						Status: KernelModuleLoadable,
					},
				},
			},
			loaded: mockKernelModulesCollector{
				result: map[string]KernelModuleInfo{
					"first": {
						Status:    KernelModuleLoaded,
						Size:      10,
						Instances: 2,
					},
				},
			},
			want: map[string][]byte{
				"host-collectors/system/kernel_modules.json": []byte("{\"first\":{\"size\":10,\"instances\":2,\"status\":\"loaded\"},\"second\":{\"size\":0,\"instances\":0,\"status\":\"loadable\"}}"),
			},
		},
		{
			name: "loaded error",
			loadable: mockKernelModulesCollector{
				result: map[string]KernelModuleInfo{
					"first": {
						Status: KernelModuleLoadable,
					},
					"second": {
						Status: KernelModuleLoadable,
					},
				},
			},
			loaded: mockKernelModulesCollector{
				err: testKernelModuleErr,
			},
			wantErr: true,
		},
		{
			name: "loadable error",
			loadable: mockKernelModulesCollector{
				err: testKernelModuleErr,
			},
			loaded: mockKernelModulesCollector{
				result: map[string]KernelModuleInfo{
					"first": {
						Status:    KernelModuleLoaded,
						Size:      10,
						Instances: 2,
					},
					"second": {
						Status: KernelModuleLoading,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "both error",
			loadable: mockKernelModulesCollector{
				err: testKernelModuleErr,
			},
			loaded: mockKernelModulesCollector{
				err: testKernelModuleErr,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CollectHostKernelModules{
				hostCollector: tt.hostCollector,
				loadable:      tt.loadable,
				loaded:        tt.loaded,
			}
			progressCh := make(chan interface{})
			defer close(progressCh)
			go func() {
				for _ = range progressCh {
				}
			}()

			got, err := c.Collect(progressCh)
			if (err != nil) != tt.wantErr {
				t.Errorf("CollectHostKernelModules.Collect() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CollectHostKernelModules.Collect() = \n%v, want \n%v", got, tt.want)
			}
		})
	}
}

func Test_parseBuiltin(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    map[string]KernelModuleInfo
	}{
		{
			name:    "empty",
			content: "",
			want:    map[string]KernelModuleInfo{},
		},
		{
			name: "basic",
			content: `kernel/arch/x86/events/rapl.ko
kernel/arch/x86/events/amd/amd-uncore.ko
kernel/arch/x86/events/intel/intel-uncore.ko
kernel/arch/x86/events/intel/intel-cstate.ko`,
			want: map[string]KernelModuleInfo{
				"rapl": {
					Status: KernelModuleLoaded,
				},
				"amd-uncore": {
					Status: KernelModuleLoaded,
				},
				"intel-uncore": {
					Status: KernelModuleLoaded,
				},
				"intel-cstate": {
					Status: KernelModuleLoaded,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := kernelModulesLoaded{}
			got, err := l.parseBuiltin(strings.NewReader(tt.content))
			if err != nil {
				t.Errorf("parseBuiltin() error = %v", err)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseBuiltin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_kernelModulesLoaded_collect(t *testing.T) {
	tests := []struct {
		name          string
		fs            fs.FS
		kernelRelease string
		want          map[string]KernelModuleInfo
		wantErr       bool
	}{
		{
			name: "lib modules path",
			fs: &fstest.MapFS{
				"proc/modules": &fstest.MapFile{
					Data: []byte(`module1 1000 2 - Live 0x0000000000000000
module2 2000 1 - Loading 0x0000000000000000
`),
					Mode: 0444,
				},
				"lib/modules/5.4.0/modules.builtin": &fstest.MapFile{
					Data: []byte(`kernel/builtin1.ko
kernel/builtin2.ko
`),
					Mode: 0644,
				},
			},
			kernelRelease: "5.4.0",
			want: map[string]KernelModuleInfo{
				"module1": {
					Size:      1000,
					Instances: 2,
					Status:    KernelModuleLoaded,
				},
				"module2": {
					Size:      2000,
					Instances: 1,
					Status:    KernelModuleLoading,
				},
				"builtin1": {
					Status: KernelModuleLoaded,
				},
				"builtin2": {
					Status: KernelModuleLoaded,
				},
			},
		},
		{
			name: "usr lib modules path",
			fs: &fstest.MapFS{
				"proc/modules": &fstest.MapFile{
					Data: []byte(`module1 1000 2 - Live 0x0000000000000000
`),
					Mode: 0444,
				},
				"usr/lib/modules/5.4.0/modules.builtin": &fstest.MapFile{
					Data: []byte(`kernel/builtin1.ko
kernel/builtin2.ko
`),
					Mode: 0644,
				},
			},
			kernelRelease: "5.4.0",
			want: map[string]KernelModuleInfo{
				"module1": {
					Size:      1000,
					Instances: 2,
					Status:    KernelModuleLoaded,
				},
				"builtin1": {
					Status: KernelModuleLoaded,
				},
				"builtin2": {
					Status: KernelModuleLoaded,
				},
			},
		},
		{
			name: "no builtin modules file",
			fs: &fstest.MapFS{
				"proc/modules": &fstest.MapFile{
					Data: []byte(`module1 1000 2 - Live 0x0000000000000000
`),
					Mode: 0444,
				},
			},
			kernelRelease: "5.4.0",
			want: map[string]KernelModuleInfo{
				"module1": {
					Size:      1000,
					Instances: 2,
					Status:    KernelModuleLoaded,
				},
			},
		},
		{
			name: "no proc modules file should error",
			fs: &fstest.MapFS{
				"lib/modules/5.4.0/modules.builtin": &fstest.MapFile{
					Data: []byte(`kernel/builtin1.ko
kernel/builtin2.ko
`),
					Mode: 0644,
				},
			},
			kernelRelease: "5.4.0",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := kernelModulesLoaded{
				fs: tt.fs,
			}
			got, _, err := l.collect(tt.kernelRelease)
			if (err != nil) != tt.wantErr {
				t.Errorf("kernelModulesLoaded.collect() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("kernelModulesLoaded.collect() = %v, want %v", got, tt.want)
			}
		})
	}
}
