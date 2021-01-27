package v1beta2

import (
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
)

type HostCollectorMeta struct {
	CollectorName string `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	// +optional
	Exclude multitype.BoolOrString `json:"exclude,omitempty" yaml:"exclude,omitempty"`
}

type CPU struct {
}

type Memory struct {
}

type HostCollect struct {
	CPU    *CPU    `json:"cpu,omitempty" yaml:"cpu,omitempty"`
	Memory *Memory `json:"memory,omitempty" yaml:"memory,omitempty"`
}

func (c *HostCollect) GetName() string {
	var collector string
	if c.CPU != nil {
		collector = "cpu"
	}
	if c.Memory != nil {
		collector = "memory"
	}

	if collector == "" {
		return "<none>"
	}

	return collector
}
