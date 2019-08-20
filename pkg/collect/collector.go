package collect

import (
	"errors"
	"fmt"
	"strings"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"gopkg.in/yaml.v2"
)

type Collector struct {
	Collect *troubleshootv1beta1.Collect
	Redact  bool
}

func (c *Collector) RunCollectorSync() ([]byte, error) {
	if c.Collect.ClusterInfo != nil {
		return ClusterInfo()
	}
	if c.Collect.ClusterResources != nil {
		return ClusterResources(c.Redact)
	}
	if c.Collect.Secret != nil {
		return Secret(c.Collect.Secret, c.Redact)
	}
	if c.Collect.Logs != nil {
		return Logs(c.Collect.Logs, c.Redact)
	}
	if c.Collect.Run != nil {
		return Run(c.Collect.Run, c.Redact)
	}
	if c.Collect.Exec != nil {
		return Exec(c.Collect.Exec, c.Redact)
	}
	if c.Collect.Copy != nil {
		return Copy(c.Collect.Copy, c.Redact)
	}
	if c.Collect.HTTP != nil {
		return HTTP(c.Collect.HTTP, c.Redact)
	}

	return nil, errors.New("no spec found to run")
}

func (c *Collector) GetDisplayName() string {
	var collector, name, selector string
	if c.Collect.ClusterInfo != nil {
		collector = "cluster-info"
	}
	if c.Collect.ClusterResources != nil {
		collector = "cluster-resources"
	}
	if c.Collect.Secret != nil {
		collector = "secret"
		name = c.Collect.Secret.CollectorName
	}
	if c.Collect.Logs != nil {
		collector = "logs"
		name = c.Collect.Logs.CollectorName
		selector = strings.Join(c.Collect.Logs.Selector, ",")
	}
	if c.Collect.Run != nil {
		collector = "run"
		name = c.Collect.Run.CollectorName
	}
	if c.Collect.Exec != nil {
		collector = "exec"
		name = c.Collect.Exec.CollectorName
		selector = strings.Join(c.Collect.Exec.Selector, ",")
	}
	if c.Collect.Copy != nil {
		collector = "copy"
		name = c.Collect.Copy.CollectorName
		selector = strings.Join(c.Collect.Copy.Selector, ",")
	}
	if c.Collect.HTTP != nil {
		collector = "http"
		name = c.Collect.HTTP.CollectorName
	}

	if collector == "" {
		return "<none>"
	}
	if name != "" {
		return fmt.Sprintf("%s/%s", collector, name)
	}
	if selector != "" {
		return fmt.Sprintf("%s/%s", collector, selector)
	}
	return collector
}

func ParseSpec(specContents string) (*troubleshootv1beta1.Collect, error) {
	collect := troubleshootv1beta1.Collect{}

	if err := yaml.Unmarshal([]byte(specContents), &collect); err != nil {
		return nil, err
	}

	return &collect, nil
}
