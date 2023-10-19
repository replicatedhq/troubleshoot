package collect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type HostRunInfo struct {
	Command   string   `json:"command"`
	ExitCode  string   `json:"exitCode"`
	Error     string   `json:"error"`
	OutputDir string   `json:"outputDir"`
	Input     string   `json:"input"`
	Env       []string `json:"env"`
}

type CollectHostRun struct {
	hostCollector *troubleshootv1beta2.HostRun
	BundlePath    string
}

func (c *CollectHostRun) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "Run Host")
}

func (c *CollectHostRun) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostRun) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	var (
		cmdOutputTempDir         string
		cmdInputTempDir          string
		bundleOutputRelativePath string
	)

	runHostCollector := c.hostCollector
	collectorName := runHostCollector.CollectorName
	if collectorName == "" {
		collectorName = "run-host"
	}

	cmd := exec.Command(runHostCollector.Command, runHostCollector.Args...)

	runInfo := &HostRunInfo{
		Command:  cmd.String(),
		ExitCode: "0",
	}

	err := c.processEnvVars(cmd)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse env variable")
	}

	// if we choose to save result for the command run
	if runHostCollector.OutputDir != "" {
		cmdOutputTempDir, err = os.MkdirTemp("", runHostCollector.OutputDir)
		defer os.RemoveAll(cmdOutputTempDir)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("failed to created temp dir for: %s", runHostCollector.OutputDir))
		}
		cmd.Env = append(cmd.Env,
			fmt.Sprintf("TS_WORKSPACE_DIR=%s", cmdOutputTempDir),
		)
	}

	if runHostCollector.Input != nil {
		cmdInputTempDir, err = os.MkdirTemp("", "input")
		defer os.RemoveAll(cmdInputTempDir)
		if err != nil {
			return nil, errors.New("failed to created temp dir for host run input")
		}
		for inFilename, inFileContent := range runHostCollector.Input {
			if strings.Contains(inFileContent, "/") {
				return nil, errors.New("Input filename contains '/'")
			}
			cmdInputFilePath := filepath.Join(cmdInputTempDir, inFilename)
			err = os.WriteFile(cmdInputFilePath, []byte(inFileContent), 0644)
			if err != nil {
				return nil, errors.Wrap(err, fmt.Sprintf("failed to write input file: %s to temp directory", inFilename))
			}
		}
		cmd.Env = append(cmd.Env,
			fmt.Sprintf("TS_INPUT_DIR=%s", cmdInputTempDir),
		)
	}

	collectorRelativePath := filepath.Join("host-collectors/run-host", collectorName)

	runInfo.Env = cmd.Env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		if werr, ok := err.(*exec.ExitError); ok {
			runInfo.ExitCode = strings.TrimPrefix(werr.Error(), "exit status ")
			runInfo.Error = stderr.String()
		} else {
			return nil, errors.Wrap(err, "failed to run")
		}
	}

	output := NewResult()
	resultInfo := filepath.Join("host-collectors/run-host", collectorName+"-info.json")
	result := filepath.Join("host-collectors/run-host", collectorName+".txt")

	b, err := json.Marshal(runInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal run host result")
	}

	output.SaveResult(c.BundlePath, resultInfo, bytes.NewBuffer(b))
	output.SaveResult(c.BundlePath, result, bytes.NewBuffer(stdout.Bytes()))
	// walkthrough the output directory and save result for each file
	if runHostCollector.OutputDir != "" {
		runInfo.OutputDir = runHostCollector.OutputDir
		bundleOutputRelativePath = filepath.Join(collectorRelativePath, runHostCollector.OutputDir)
		output.SaveResults(c.BundlePath, bundleOutputRelativePath, cmdOutputTempDir)
	}

	return output, nil
}

func (c *CollectHostRun) processEnvVars(cmd *exec.Cmd) error {
	runHostCollector := c.hostCollector

	if runHostCollector.IgnoreParentEnvs {
		// clears the parent env vars
		cmd.Env = []string{}
	} else if runHostCollector.InheritEnvs != nil {
		for _, key := range runHostCollector.InheritEnvs {
			envVal, found := os.LookupEnv(key)
			if !found {
				return errors.New(fmt.Sprintf("inherit env variable is not found: %s", key))
			}
			cmd.Env = append(cmd.Env,
				fmt.Sprintf("%s=%s", key, envVal))
		}
	}

	guaranteedEnvs := []string{"PATH", "KUBECONFIG"}
	for _, key := range guaranteedEnvs {
		guaranteedEnvVal, found := os.LookupEnv(key)
		if found {
			cmd.Env = append(cmd.Env,
				fmt.Sprintf("%s=%s", key, guaranteedEnvVal))
		}
	}

	if runHostCollector.Env != nil {
		for i := range runHostCollector.Env {
			parts := strings.Split(runHostCollector.Env[i], "=")
			if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
				cmd.Env = append(cmd.Env,
					fmt.Sprintf("%s", runHostCollector.Env[i]))
			} else {
				return errors.New(fmt.Sprintf("env variable entry is missing '=' : %s", runHostCollector.Env[i]))
			}
		}
	}

	return nil
}
