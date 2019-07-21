package collect

import (
	"errors"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"gopkg.in/yaml.v2"
)

type Collector struct {
	Spec   string
	Redact bool
}

func (c *Collector) RunCollectorSync() error {
	collect, err := parseSpec(c.Spec)
	if err != nil {
		return err
	}

	if collect.ClusterInfo != nil {
		return ClusterInfo()
	}
	if collect.ClusterResources != nil {
		return ClusterResources(c.Redact)
	}
	if collect.Secret != nil {
		return Secret(collect.Secret, c.Redact)
	}
	if collect.Logs != nil {
		return Logs(collect.Logs, c.Redact)
	}
	if collect.Run != nil {
		return Run(collect.Run, c.Redact)
	}
	if collect.Exec != nil {
		return Exec(collect.Exec, c.Redact)
	}
	if collect.Copy != nil {
		return Copy(collect.Copy, c.Redact)
	}
	if collect.HTTP != nil {
		return HTTP(collect.HTTP, c.Redact)
	}

	return errors.New("no spec found to run")
}

func parseSpec(specContents string) (*troubleshootv1beta1.Collect, error) {
	collect := troubleshootv1beta1.Collect{}

	if err := yaml.Unmarshal([]byte(specContents), &collect); err != nil {
		return nil, err
	}

	return &collect, nil
}
