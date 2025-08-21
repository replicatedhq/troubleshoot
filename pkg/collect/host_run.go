package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/klog/v2"
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

func (c *CollectHostRun) SkipRedaction() bool {
	return c.hostCollector.SkipRedaction
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

	var (
		timeout            time.Duration
		cmd                *exec.Cmd
		errInvalidDuration error
	)

	ctx := context.Background()
	cmdPath := c.attemptToConvertCmdToAbsPath()

	if runHostCollector.Timeout != "" {
		timeout, errInvalidDuration = time.ParseDuration(runHostCollector.Timeout)
		if errInvalidDuration != nil {
			return nil, errors.Wrapf(errInvalidDuration, "failed to parse timeout %q", runHostCollector.Timeout)
		}
	}

	if timeout <= time.Duration(0) {
		cmd = exec.Command(cmdPath, runHostCollector.Args...)
	} else {
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		cmd = exec.CommandContext(timeoutCtx, cmdPath, runHostCollector.Args...)
	}

	klog.V(2).Infof("Run host collector command: %q", cmd.String())
	runInfo := &HostRunInfo{
		Command:  cmd.String(),
		ExitCode: "0",
	}

	err := c.processEnvVars(cmd)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse env variable")
	}

	// Create a working directory for the command
	wkdir, err := os.MkdirTemp("", collectorName)
	defer os.RemoveAll(wkdir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temp dir for host run")
	}
	// Change the working directory for the command to ensure the command
	// does not polute the parent/caller working directory
	cmd.Dir = wkdir

	// if we choose to save result for the command run
	if runHostCollector.OutputDir != "" {
		cmdOutputTempDir = filepath.Join(wkdir, runHostCollector.OutputDir)
		err = os.MkdirAll(cmdOutputTempDir, 0755)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("failed to create dir for: %s", runHostCollector.OutputDir))
		}
		cmd.Env = append(cmd.Env,
			fmt.Sprintf("TS_OUTPUT_DIR=%s", cmdOutputTempDir),
		)
	}

	if runHostCollector.Input != nil {
		cmdInputTempDir = filepath.Join(wkdir, "input")
		err = os.MkdirAll(cmdInputTempDir, 0755)
		if err != nil {
			return nil, errors.New("failed to create temp dir for host run input")
		}
		for inFilename, inFileContent := range runHostCollector.Input {
			if strings.Contains(inFilename, "/") {
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
		} else if err == context.DeadlineExceeded {
			runInfo.ExitCode = "-1"
			runInfo.Error = fmt.Sprintf("command timed out after %s", timeout.String())
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
		klog.V(2).Infof("Saving command output to %q in bundle", bundleOutputRelativePath)
		output.SaveResults(c.BundlePath, bundleOutputRelativePath, cmdOutputTempDir)
	}

	return output, nil
}

func (c *CollectHostRun) processEnvVars(cmd *exec.Cmd) error {
	runHostCollector := c.hostCollector

	if runHostCollector.IgnoreParentEnvs {
		klog.V(2).Info("Not inheriting the environment variables!")
		if runHostCollector.InheritEnvs != nil {
			klog.V(2).Infof("The following environment variables will not be loaded to the command: [%s]",
				strings.Join(runHostCollector.InheritEnvs, ","))
		}
		// clears the parent env vars
		cmd.Env = []string{}
		populateGuaranteedEnvVars(cmd)
	} else if runHostCollector.InheritEnvs != nil {
		for _, key := range runHostCollector.InheritEnvs {
			envVal, found := os.LookupEnv(key)
			if !found {
				return errors.New(fmt.Sprintf("inherit env variable is not found: %s", key))
			}
			cmd.Env = append(cmd.Env,
				fmt.Sprintf("%s=%s", key, envVal))
		}
		populateGuaranteedEnvVars(cmd)
	} else {
		cmd.Env = os.Environ()
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

func populateGuaranteedEnvVars(cmd *exec.Cmd) {
	guaranteedEnvs := []string{"PATH", "KUBECONFIG", "PWD"}
	for _, key := range guaranteedEnvs {
		guaranteedEnvVal, found := os.LookupEnv(key)
		if found {
			cmd.Env = append(cmd.Env,
				fmt.Sprintf("%s=%s", key, guaranteedEnvVal))
		}
	}
}

// attemptToConvertCmdToAbsPath checks if the command is a file path or command name
// If it is a file path, it will return the absolute path else
// it will return the command name as is and leave the resolution to cmd.Run()
// This enables passing commands using relative paths e.g. "./my-command"
// which is not possible with cmd.Run() since the child process runs
// in a different working directory
func (c *CollectHostRun) attemptToConvertCmdToAbsPath() string {
	// Attempt to check if the command is file path or command name
	cmdAbsPath, err := filepath.Abs(c.hostCollector.Command)
	if err != nil {
		return c.hostCollector.Command
	}

	// Check if the file exists
	_, err = os.Stat(cmdAbsPath)
	if err != nil {
		return c.hostCollector.Command
	}

	return cmdAbsPath
}

func (c *CollectHostRun) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}
