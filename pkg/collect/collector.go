package collect

import (
	"errors"
	"fmt"
	"strings"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/rest"
)

type Collector struct {
	Collect      *troubleshootv1beta1.Collect
	Redact       bool
	ClientConfig *rest.Config
	Namespace    string
}

type Context struct {
	Redact       bool
	ClientConfig *rest.Config
	Namespace    string
}

func (c *Collector) RunCollectorSync() ([]byte, error) {
	if c.Collect.ClusterInfo != nil {
		if c.Collect.ClusterInfo.Exclude {
			return nil, nil
		}
		return ClusterInfo(c.GetContext())
	}
	if c.Collect.ClusterResources != nil {
		if c.Collect.ClusterResources.Exclude {
			return nil, nil
		}
		return ClusterResources(c.GetContext())
	}
	if c.Collect.Secret != nil {
		if c.Collect.Secret.Exclude {
			return nil, nil
		}
		return Secret(c.GetContext(), c.Collect.Secret)
	}
	if c.Collect.Logs != nil {
		if c.Collect.Logs.Exclude {
			return nil, nil
		}
		return Logs(c.GetContext(), c.Collect.Logs)
	}
	if c.Collect.Run != nil {
		if c.Collect.Run.Exclude {
			return nil, nil
		}
		return Run(c.GetContext(), c.Collect.Run)
	}
	if c.Collect.Exec != nil {
		if c.Collect.Exec.Exclude {
			return nil, nil
		}
		return Exec(c.GetContext(), c.Collect.Exec)
	}
	if c.Collect.Data != nil {
		if c.Collect.Data.Exclude {
			return nil, nil
		}
		return Data(c.GetContext(), c.Collect.Data)
	}
	if c.Collect.Copy != nil {
		if c.Collect.Copy.Exclude {
			return nil, nil
		}
		return Copy(c.GetContext(), c.Collect.Copy)
	}
	if c.Collect.HTTP != nil {
		if c.Collect.HTTP.Exclude {
			return nil, nil
		}
		return HTTP(c.GetContext(), c.Collect.HTTP)
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

func (c *Collector) GetContext() *Context {
	return &Context{
		Redact:       c.Redact,
		ClientConfig: c.ClientConfig,
		Namespace:    c.Namespace,
	}
}

func ParseSpec(specContents string) (*troubleshootv1beta1.Collect, error) {
	collect := troubleshootv1beta1.Collect{}

	if err := yaml.Unmarshal([]byte(specContents), &collect); err != nil {
		return nil, err
	}

	return &collect, nil
}
