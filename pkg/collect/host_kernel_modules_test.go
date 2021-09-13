package collect

import (
	"reflect"
	"testing"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type mockKernelModulesCollector struct {
	result map[string]KernelModuleInfo
	err    error
}

func (m mockKernelModulesCollector) collect() (map[string]KernelModuleInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
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
				"system/kernel_modules.json": []byte("{\"first\":{\"size\":0,\"instances\":0,\"status\":\"loadable\"},\"second\":{\"size\":0,\"instances\":0,\"status\":\"loadable\"}}"),
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
				"system/kernel_modules.json": []byte("{\"first\":{\"size\":10,\"instances\":2,\"status\":\"loaded\"},\"second\":{\"size\":0,\"instances\":0,\"status\":\"loading\"}}"),
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
				"system/kernel_modules.json": []byte("{\"first\":{\"size\":10,\"instances\":2,\"status\":\"loaded\"},\"second\":{\"size\":0,\"instances\":0,\"status\":\"loadable\"}}"),
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
