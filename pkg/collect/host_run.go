package collect

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type HostRunResults struct {
	Command  string `json:"result"`
	ExitCode string `json:"exitCode"`
	Error    string `json:"error"`
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
	runHostCollector := c.hostCollector

	cmd := exec.Command(runHostCollector.Command, runHostCollector.Args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runResult := HostRunResults{
		Command:  cmd.String(),
		ExitCode: "0",
	}

	err := cmd.Run()
	if err != nil {
		if werr, ok := err.(*exec.ExitError); ok {
			runResult.ExitCode = strings.TrimPrefix(werr.Error(), "exit status ")
			runResult.Error = stderr.String()
		} else {
			return nil, errors.Wrap(err, "failed to run")
		}
	}

	collectorName := c.hostCollector.CollectorName
	if collectorName == "" {
		collectorName = "run-host"
	}
	resultInfo := filepath.Join("host-collectors/run-host", collectorName+"-info.json")
	result := filepath.Join("host-collectors/run-host", collectorName+".txt")

	b, err := json.Marshal(runResult)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal run host result")
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, resultInfo, bytes.NewBuffer(b))
	output.SaveResult(c.BundlePath, result, bytes.NewBuffer(stdout.Bytes()))

	runHostOutput := map[string][]byte{
		resultInfo: b,
		result:     stdout.Bytes(),
	}

	return runHostOutput, nil
}
