//go:build integration

// NOTE: requires a Kubernetes cluster in place currently, hence hidden behind a tag above
// TODOLATER: get a mocked or ephemeral/Docker based K8s for use? see below some approaches which haven't played out yet
// Test using: go test --tags=integration

package preflight

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	/*
		See TODOLATER below

		"k8s.io/client-go/kubernetes/fake"
		k8sruntime "k8s.io/apimachinery/pkg/runtime"
		discoveryfake "k8s.io/client-go/discovery/fake"
	*/)

func TestRunPreflights(t *testing.T) {
	t.Parallel()

	// A very simple preflight spec (local file)
	_, testFilename, _, status := runtime.Caller(0)
	if status == false {
		t.Fatal("Cannot determine current directory for go test source code, required for relative path of preflight spec YAML")
	}
	preflightFile := fmt.Sprintf("%s/troubleshoot_v1beta2_preflight_gotest.yaml", filepath.ToSlash(filepath.Dir(testFilename)))

	type preflightTests struct {
		Interactive bool
		Output      string
		Format      string
		Args        []string
		//
		ExpectedError      error
		ExpectedOutputFile string
		// May be in stdout or file, depending on above value
		ExpectedOutputContent string
	}

	tests := []preflightTests{}
	// TODO: test interactive true as well
	for _, interactive := range []bool{false} {
		// false = no output file specified, true = specify a (temp) output filename
		for _, outputToFile := range []bool{false, true} {
			for _, format := range []string{"human", "json", "yaml"} {
				thisTest := preflightTests{
					Interactive: interactive,
					Format:      format,
					// TODOLATER: try a URL also, using a local mocked HTTP server for testing? using a local file only for now
					Args:          []string{preflightFile},
					ExpectedError: nil,
				}

				// An output file should be written
				if outputToFile == true {
					randBytes := make([]byte, 16)
					rand.Read(randBytes)
					outputFilename := filepath.Join(os.TempDir(), fmt.Sprintf("preflight_out_test_%s", hex.EncodeToString(randBytes)))
					thisTest.Output = outputFilename
					thisTest.ExpectedOutputFile = outputFilename
				}

				switch format {
				case "human":
					thisTest.ExpectedOutputContent = `
   --- PASS Compare JSON Example
      --- The collected data matches the value.
--- PASS   go-test-preflight
PASS
`
				case "json":
					thisTest.ExpectedOutputContent = `{
  "pass": [
    {
      "title": "Compare JSON Example",
      "message": "The collected data matches the value."
    }
  ]
}
`
				case "yaml":
					thisTest.ExpectedOutputContent = `pass:
- title: Compare JSON Example
  message: The collected data matches the value.

`
				default:
					t.Fatal("Error: unsupported output format in test")
				}

				tests = append(tests, thisTest)
			}
		}
	}

	// Show the full list of permutations
	//fmt.Printf("+v\n", tests)

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

	for _, i := range tests {
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

		tErr := RunPreflights(i.Interactive, i.Output, i.Format, i.Args)

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

		if tErr != i.ExpectedError {
			t.Error(err)
		}

		useBufOut := string(bufOut[:nOut])
		//useBufErr := string(bufErr[:nErr])
		//fmt.Printf("stdout: %+v\n", useBufOut)
		//fmt.Printf("stderr: %+v\n", useBufErr)

		if i.ExpectedOutputFile != "" {
			// Output file is expected, make sure it exists
			if _, err := os.Stat(i.ExpectedOutputFile); errors.Is(err, os.ErrNotExist) {
				t.Errorf("interactive: %t format %s :: Output file %s was expected, does not exist\n", i.Interactive, i.Format, i.ExpectedOutputFile)
			} else {
				// If it exists, check contents of output file against expected
				readOutputFile, err := os.ReadFile(i.ExpectedOutputFile)
				if err != nil {
					t.Error(err)
				}
				if string(readOutputFile) != i.ExpectedOutputContent {
					t.Errorf("interactive: %t format %s :: Output in file does not match expected. Expected length: %d, stdout buffer: %d\n", i.Interactive, i.Format, len(i.ExpectedOutputContent), len(readOutputFile))
				}
			}
		} else {
			// Expected no output file, make sure it doesn't exist
			if _, err := os.Stat(i.ExpectedOutputFile); err == nil {
				t.Errorf("interactive: %t format %s :: Output file %s was not expected, but one was found\n", i.Interactive, i.Format, i.ExpectedOutputFile)
			}

			// Check stdout against expected output
			if useBufOut != i.ExpectedOutputContent {
				t.Errorf("interactive: %t format %s :: Output to stdout does not match expected. Expected length: %d, stdout buffer length: %d\n\nexpected: ''%s''\n\ngot: ''%s''\n", i.Interactive, i.Format, len(i.ExpectedOutputContent), len(useBufOut), i.ExpectedOutputContent, useBufOut)
			}
		}

		// Remove the (temp) output file if it exists
		if _, err := os.Stat(i.ExpectedOutputFile); err == nil {
			err = os.Remove(i.ExpectedOutputFile)
			if err != nil {
				t.Error(err)
			}
		}
	}
}
