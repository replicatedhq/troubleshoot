package collect

import (
	"errors"

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

func ParseSpec(specContents string) (*troubleshootv1beta1.Collect, error) {
	collect := troubleshootv1beta1.Collect{}

	if err := yaml.Unmarshal([]byte(specContents), &collect); err != nil {
		return nil, err
	}

	return &collect, nil
}
