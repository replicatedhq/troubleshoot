package collect

import (
	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"gopkg.in/yaml.v2"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Collector struct {
	Collect      *troubleshootv1beta1.Collect
	Redact       bool
	RBACErrors   []error
	ClientConfig *rest.Config
	Namespace    string
}

type Collectors []*Collector

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
	return c.Collect.GetName()
}

func (c *Collector) GetContext() *Context {
	return &Context{
		Redact:       c.Redact,
		ClientConfig: c.ClientConfig,
		Namespace:    c.Namespace,
	}
}

func (c *Collector) CheckRBAC() error {
	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return errors.Wrap(err, "failed to create client from config")
	}

	forbidden := make([]error, 0)

	specs := c.Collect.AccessReviewSpecs(c.Namespace)
	for _, spec := range specs {

		sar := &authorizationv1.SelfSubjectAccessReview{
			Spec: spec,
		}

		resp, err := client.AuthorizationV1().SelfSubjectAccessReviews().Create(sar)
		if err != nil {
			return errors.Wrap(err, "failed to run subject review")
		}

		if !resp.Status.Allowed { // all other fields of Status are empty...
			forbidden = append(forbidden, RBACError{
				DisplayName: c.GetDisplayName(),
				Namespace:   spec.ResourceAttributes.Namespace,
				Resource:    spec.ResourceAttributes.Resource,
				Verb:        spec.ResourceAttributes.Verb,
			})
		}
	}
	c.RBACErrors = forbidden

	return nil
}

func (cs Collectors) CheckRBAC() error {
	for _, c := range cs {
		if err := c.CheckRBAC(); err != nil {
			return errors.Wrap(err, "failed to check RBAC")
		}
	}
	return nil
}

func ParseSpec(specContents string) (*troubleshootv1beta1.Collect, error) {
	collect := troubleshootv1beta1.Collect{}

	if err := yaml.Unmarshal([]byte(specContents), &collect); err != nil {
		return nil, err
	}

	return &collect, nil
}
