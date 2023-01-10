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

	expectedOutputContentHuman := `
   --- PASS Compare JSON Example
      --- The collected data matches the value.
--- PASS   go-test-preflight
PASS
`

	expectedOutputContentJson := `{
  "pass": [
    {
      "title": "Compare JSON Example",
      "message": "The collected data matches the value."
    }
  ]
}
`

	expectedOutputContentYaml := `pass:
- title: Compare JSON Example
  message: The collected data matches the value.

`

	tests := []struct {
		interactive bool
		output      string
		format      string
		args        []string
		//
		expectedError error
		// May be in stdout or file, depending on above value
		expectedOutputContent string
	}{
		// TODOLATER: test interactive true as well
		{
			interactive:           false,
			output:                "",
			format:                "human",
			args:                  []string{preflightFile},
			expectedError:         nil,
			expectedOutputContent: expectedOutputContentHuman,
		},
		{
			interactive:           false,
			output:                "",
			format:                "json",
			args:                  []string{preflightFile},
			expectedError:         nil,
			expectedOutputContent: expectedOutputContentJson,
		},
		{
			interactive:           false,
			output:                "",
			format:                "yaml",
			args:                  []string{preflightFile},
			expectedError:         nil,
			expectedOutputContent: expectedOutputContentYaml,
		},
		{
			interactive:           false,
			output:                testutils.TempFilename("preflight_out_test_"),
			format:                "human",
			args:                  []string{preflightFile},
			expectedError:         nil,
			expectedOutputContent: expectedOutputContentHuman,
		},
		{
			interactive:           false,
			output:                testutils.TempFilename("preflight_out_test_"),
			format:                "json",
			args:                  []string{preflightFile},
			expectedError:         nil,
			expectedOutputContent: expectedOutputContentJson,
		},
		{
			interactive:           false,
			output:                testutils.TempFilename("preflight_out_test_"),
			format:                "yaml",
			args:                  []string{preflightFile},
			expectedError:         nil,
			expectedOutputContent: expectedOutputContentYaml,
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
		// Capture stdout/stderr along the way
		rOut, wOut, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		rErr, wErr, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		// Redirect stdout/err to the pipes temporarily
		origStdout := os.Stdout
		os.Stdout = wOut
		origStderr := os.Stderr
		os.Stderr = wErr

		tErr := RunPreflights(tt.interactive, tt.output, tt.format, tt.args)

		// Stop redirection of stdout/stderr
		bufOut := make([]byte, 1024)
		nOut, err := rOut.Read(bufOut)
		if err != nil {
			t.Fatal(err)
		}
		bufErr := make([]byte, 1024)
		// nErr
		_, err = rErr.Read(bufErr)
		if err != nil {
			t.Fatal(err)
		}
		os.Stdout = origStdout
		os.Stderr = origStderr

		if tErr != tt.expectedError {
			t.Error(err)
		}

		useBufOut := string(bufOut[:nOut])
		//useBufErr := string(bufErr[:nErr])
		//fmt.Printf("stdout: %+v\n", useBufOut)
		//fmt.Printf("stderr: %+v\n", useBufErr)

		if tt.output != "" {
			// Output file is expected, make sure it exists
			if _, err := os.Stat(tt.output); errors.Is(err, os.ErrNotExist) {
				t.Errorf("interactive: %t format %s :: Output file %s was expected, does not exist\n", tt.interactive, tt.format, tt.output)
			} else {
				// If it exists, check contents of output file against expected
				readOutputFile, err := os.ReadFile(tt.output)
				if err != nil {
					t.Error(err)
				}
				if string(readOutputFile) != tt.expectedOutputContent {
					t.Errorf("interactive: %t format %s :: Output in file does not match expected. Expected length: %d, stdout buffer: %d\n", tt.interactive, tt.format, len(tt.expectedOutputContent), len(readOutputFile))
				}
			}
		} else {
			// Expected no output file, make sure it doesn't exist
			if _, err := os.Stat(tt.output); err == nil {
				t.Errorf("interactive: %t format %s :: Output file %s was not expected, but one was found\n", tt.interactive, tt.format, tt.output)
			}

			// Check stdout against expected output
			if useBufOut != tt.expectedOutputContent {
				t.Errorf("interactive: %t format %s :: Output to stdout does not match expected. Expected length: %d, stdout buffer length: %d\n\nexpected: ''%s''\n\ngot: ''%s''\n", tt.interactive, tt.format, len(tt.expectedOutputContent), len(useBufOut), tt.expectedOutputContent, useBufOut)
			}
		}

		// Remove the (temp) output file if it exists
		if _, err := os.Stat(tt.output); err == nil {
			err = os.Remove(tt.output)
			if err != nil {
				t.Error(err)
			}
		}
	}
}
