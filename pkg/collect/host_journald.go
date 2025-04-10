package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/klog/v2"
)

type CollectHostJournald struct {
	hostCollector *troubleshootv1beta2.HostJournald
	BundlePath    string
}

const HostJournaldPath = `host-collectors/journald/`

func (c *CollectHostJournald) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "journald")
}

func (c *CollectHostJournald) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostJournald) SkipRedaction() bool {
	return c.hostCollector.SkipRedaction
}

func (c *CollectHostJournald) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {

	// collector name check
	collectorName := c.hostCollector.CollectorName
	if collectorName == "" {
		return nil, errors.New("collector name is required")
	}

	// timeout check
	timeout, err := getTimeout(c.hostCollector.Timeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse timeout")
	}

	// set timeout context
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// prepare command options
	cmdOptions, err := generateOptions(c.hostCollector)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate journalctl options")
	}

	// run journalctl and capture output
	klog.V(2).Infof("Running journalctl with options: %v", cmdOptions)
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "journalctl", cmdOptions...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	cmdInfo := HostRunInfo{
		Command:  cmd.String(),
		ExitCode: "0",
	}

	if err := cmd.Run(); err != nil {
		klog.V(2).Infof("journalctl command failed: %v", err)
		if err == context.DeadlineExceeded {
			cmdInfo.ExitCode = "124"
			cmdInfo.Error = fmt.Sprintf("command timed out after %s", timeout.String())
		} else if exitError, ok := err.(*exec.ExitError); ok {
			cmdInfo.ExitCode = strconv.Itoa(exitError.ExitCode())
			cmdInfo.Error = stderr.String()
		} else {
			return nil, errors.Wrap(err, "failed to run journalctl")
		}
	}

	output := NewResult()

	// write info file
	infoJsonBytes, err := json.Marshal(cmdInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal journalctl info result")
	}
	infoFileName := getOutputInfoFile(collectorName)
	output.SaveResult(c.BundlePath, infoFileName, bytes.NewBuffer(infoJsonBytes))

	// write actual journalctl output
	outputFileName := getOutputFile(collectorName)
	klog.V(2).Infof("Saving journalctl output to %q in bundle", outputFileName)
	output.SaveResult(c.BundlePath, outputFileName, bytes.NewBuffer(stdout.Bytes()))

	return output, nil
}

func generateOptions(jd *troubleshootv1beta2.HostJournald) ([]string, error) {
	options := []string{}

	if jd.System {
		options = append(options, "--system")
	}

	if jd.Dmesg {
		options = append(options, "--dmesg")
	}

	for _, unit := range jd.Units {
		options = append(options, "-u", unit)
	}

	if jd.Since != "" {
		options = append(options, "--since", jd.Since)
	}

	if jd.Until != "" {
		options = append(options, "--until", jd.Until)
	}

	if jd.Output != "" {
		options = append(options, "--output", jd.Output)
	}

	if jd.Lines > 0 {
		options = append(options, "-n", strconv.Itoa(jd.Lines))
	}

	if jd.Reverse {
		options = append(options, "--reverse")
	}

	if jd.Utc {
		options = append(options, "--utc")
	}

	// opinionated on --no-pager
	options = append(options, "--no-pager")

	return options, nil
}

func getOutputFile(collectorName string) string {
	return filepath.Join(HostJournaldPath, collectorName+".txt")
}

func getOutputInfoFile(collectorName string) string {
	return filepath.Join(HostJournaldPath, collectorName+"-info.json")
}

func getTimeout(timeout string) (time.Duration, error) {
	if timeout == "" {
		return 30 * time.Second, nil
	}

	return time.ParseDuration(timeout)
}

func (c *CollectHostJournald) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}
