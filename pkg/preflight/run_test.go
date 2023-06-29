//go:build integration

// NOTE: requires a Kubernetes cluster in place currently, hence hidden behind a tag above
// TODOLATER: get a mocked or ephemeral/Docker based K8s for use? see below some approaches which haven't played out yet
// Test using: go test --tags=integration

package preflight

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/replicatedhq/troubleshoot/internal/testutils"
	analyzerunner "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	/*
		See TODOLATER below

		"k8s.io/client-go/kubernetes/fake"
		k8sruntime "k8s.io/apimachinery/pkg/runtime"
		discoveryfake "k8s.io/client-go/discovery/fake"
	*/)

func TestRunPreflights(t *testing.T) {
	t.Parallel()

	// A very simple preflight spec (local file)
	preflightFile := filepath.Join(testutils.FileDir(), "../../testdata/preflightspec/troubleshoot_v1beta2_preflight_gotest.yaml")

	wantOutputContentHuman := `
   --- PASS Compare JSON Example
      --- The collected data matches the value.
--- PASS   go-test-preflight
PASS
`

	wantOutputContentJson := `{
  "pass": [
    {
      "title": "Compare JSON Example",
      "message": "The collected data matches the value."
    }
  ]
}
`

	wantOutputContentYaml := `pass:
- title: Compare JSON Example
  message: The collected data matches the value.

`

	tests := []struct {
		name        string
		interactive bool
		output      string
		format      string
		args        []string
		//
		wantExitCode int
		wantErr      bool
		// May be in stdout or file, depending on above value
		wantOutputContent string
	}{
		// TODOLATER: test interactive true as well
		{
			name:              "noninteractive-stdout-human",
			interactive:       false,
			output:            "",
			format:            "human",
			args:              []string{preflightFile},
			wantExitCode:      0,
			wantErr:           false,
			wantOutputContent: wantOutputContentHuman,
		},
		{
			name:              "noninteractive-stdout-json",
			interactive:       false,
			output:            "",
			format:            "json",
			args:              []string{preflightFile},
			wantExitCode:      0,
			wantErr:           false,
			wantOutputContent: wantOutputContentJson,
		},
		{
			name:              "noninteractive-stdout-yaml",
			interactive:       false,
			output:            "",
			format:            "yaml",
			args:              []string{preflightFile},
			wantExitCode:      0,
			wantErr:           false,
			wantOutputContent: wantOutputContentYaml,
		},
		{
			name:              "noninteractive-tofile-human",
			interactive:       false,
			output:            testutils.TempFilename("preflight_out_test_"),
			format:            "human",
			args:              []string{preflightFile},
			wantExitCode:      0,
			wantErr:           false,
			wantOutputContent: wantOutputContentHuman,
		},
		{
			name:              "noninteractive-tofile-json",
			interactive:       false,
			output:            testutils.TempFilename("preflight_out_test_"),
			format:            "json",
			args:              []string{preflightFile},
			wantExitCode:      0,
			wantErr:           false,
			wantOutputContent: wantOutputContentJson,
		},
		{
			name:              "noninteractive-tofile-yaml",
			interactive:       false,
			output:            testutils.TempFilename("preflight_out_test_"),
			format:            "yaml",
			args:              []string{preflightFile},
			wantExitCode:      0,
			wantErr:           false,
			wantOutputContent: wantOutputContentYaml,
		},
	}

	// Use a fake/mocked K8s API, since some collectors are mandatory and need an API server to hit
	// TODOLATER: for this to work, we need to refactor all of the collectors and analyzers to allow passing in a fake clientset?
	// ...or we need to find a way to expose the fake clientset with a mocked API server and a respective kubeconfig for use
	// Using gnomock k3s for now instead (requires Docker for test execution)
	/*
		k8sObjects := []k8sruntime.Object{
			// TODO: populate with things that mandatory collectors need
		}
		k8sApi := fake.NewSimpleClientset(k8sObjects...)
		k8sApi.Discovery().(*discoveryfake.FakeDiscovery).FakedServerVersion = &version.Info{
			Major: "1",
			Minor: "26",
		}
		fmt.Printf("%+v\n", k8sApi)
	*/

	// A K3s instance in Docker for testing, for mandatory collectors
	// NOTE: only has amd64 images, doesn't work on arm64 (MacOS)? need to build and specify custom images for it
	// ...plus there's no new images since 2020~?
	/*
		p := k3s.Preset(k3s.WithVersion("v1.19.12"))
		c, err := gnomock.Start(
			p,
			gnomock.WithContainerName("k3s"),
			gnomock.WithDebugMode(),
		)
		if err != nil {
			t.Fatal(err)
		}
		kubeconfig, err := k3s.Config(c)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("kubeconfig: %+v\n", kubeconfig)
	*/

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout/stderr along the way
			rOut, wOut, err := os.Pipe()
			require.Nil(t, err)
			rErr, wErr, err := os.Pipe()
			require.Nil(t, err)
			// Redirect stdout/err to the pipes temporarily
			origStdout := os.Stdout
			os.Stdout = wOut
			origStderr := os.Stderr
			os.Stderr = wErr

			tErr := RunPreflights(tt.interactive, tt.output, tt.format, tt.args)

			// Stop redirection of stdout/stderr
			bufOut := make([]byte, 1024)
			nOut, err := rOut.Read(bufOut)
			require.Nil(t, err)
			bufErr := make([]byte, 1024)
			// nErr
			_, err = rErr.Read(bufErr)
			require.Nil(t, err)
			os.Stdout = origStdout
			os.Stderr = origStderr

			if tt.wantErr {
				// It's always an error of some form
				assert.Error(t, tErr)

				var exitErr types.ExitError
				// Make sure we can always turn it into an ExitError
				assert.True(t, errors.As(tErr, &exitErr))

				assert.Equal(t, tt.wantExitCode, exitErr.ExitStatus())
				assert.NotEmpty(t, exitErr.Error())
			} else {
				assert.Nil(t, tErr)
			}

			useBufOut := string(bufOut[:nOut])
			//useBufErr := string(bufErr[:nErr])
			//fmt.Printf("stdout: %+v\n", useBufOut)
			//fmt.Printf("stderr: %+v\n", useBufErr)

			if tt.output != "" {
				// Output file is expected, make sure it exists
				assert.FileExists(t, tt.output)
				// If it exists, check contents of output file against expected
				readOutputFile, err := os.ReadFile(tt.output)
				require.NoError(t, err)
				assert.Equal(t, string(readOutputFile), tt.wantOutputContent)
			} else {
				// Expected no output file, make sure it doesn't exist
				assert.NoFileExists(t, tt.output)
				// Check stdout against expected output
				assert.Equal(t, useBufOut, tt.wantOutputContent)
			}

			// Remove the (temp) output file if it exists
			if _, err := os.Stat(tt.output); err == nil {
				err = os.Remove(tt.output)
				require.NoError(t, err)
			}
		})
	}
}

func TestCheckOutcomesToExitCode(t *testing.T) {
	tests := []struct {
		name         string
		results      []*analyzerunner.AnalyzeResult
		wantExitCode int
	}{
		{
			name: "all-pass",
			results: []*analyzerunner.AnalyzeResult{
				&analyzerunner.AnalyzeResult{
					IsPass: true,
					IsFail: false,
					IsWarn: false,
				},
				&analyzerunner.AnalyzeResult{
					IsPass: true,
					IsFail: false,
					IsWarn: false,
				},
			},
			wantExitCode: 0,
		},
		{
			name: "one-warn",
			results: []*analyzerunner.AnalyzeResult{
				&analyzerunner.AnalyzeResult{
					IsPass: true,
					IsFail: false,
					IsWarn: false,
				},
				&analyzerunner.AnalyzeResult{
					IsPass: false,
					IsFail: false,
					IsWarn: true,
				},
			},
			wantExitCode: 4,
		},
		{
			name: "one-fail",
			results: []*analyzerunner.AnalyzeResult{
				&analyzerunner.AnalyzeResult{
					IsPass: true,
					IsFail: false,
					IsWarn: false,
				},
				&analyzerunner.AnalyzeResult{
					IsPass: false,
					IsFail: true,
					IsWarn: false,
				},
			},
			wantExitCode: 3,
		},
		{
			name: "one-warn-one-fail",
			results: []*analyzerunner.AnalyzeResult{
				&analyzerunner.AnalyzeResult{
					IsPass: true,
					IsFail: false,
					IsWarn: false,
				},
				&analyzerunner.AnalyzeResult{
					IsPass: false,
					IsFail: true,
					IsWarn: false,
				},
				&analyzerunner.AnalyzeResult{
					IsPass: false,
					IsFail: false,
					IsWarn: true,
				},
			},
			// A fail is a fail...
			wantExitCode: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotExitCode := checkOutcomesToExitCode(tt.results)

			assert.Equal(t, tt.wantExitCode, gotExitCode)
		})
	}
}

func TestValidatePreflight(t *testing.T) {
	noCollectorsPreflightFile := filepath.Join(testutils.FileDir(), "../../testdata/preflightspec/troubleshoot_v1beta2_preflight_validate_empty_collectors_gotest.yaml")
	noAnalyzersPreflightFile := filepath.Join(testutils.FileDir(), "../../testdata/preflightspec/troubleshoot_v1beta2_preflight_validate_empty_analyzers_gotest.yaml")
	// hostpreflightFile := filepath.Join(testutils.FileDir(), "../../examples/preflight/host/block-devices.yaml")
	tests := []struct {
		name          string
		preflightSpec string
		wantWarning   *types.ExitCodeWarning
	}{
		{
			name:          "empty-preflight",
			preflightSpec: "",
			wantWarning:   types.NewExitCodeWarning("no preflight or host preflight spec was found"),
		},
		{
			name:          "no-collectores",
			preflightSpec: noCollectorsPreflightFile,
			wantWarning:   types.NewExitCodeWarning("No collectors found"),
		},
		{
			name:          "no-analyzers",
			preflightSpec: noAnalyzersPreflightFile,
			wantWarning:   types.NewExitCodeWarning("No analyzers found"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specs := PreflightSpecs{}
			specs.Read([]string{tt.preflightSpec})
			gotWarning := validatePreflight(specs)
			assert.Equal(t, tt.wantWarning.Warning(), gotWarning.Warning())
		})
	}
}
