package collect

import (
	"strconv"

	"github.com/pkg/errors"
	"github.com/replicatedhq/kots/kotskinds/multitype"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
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

func isExcluded(excludeVal multitype.BoolOrString) (bool, error) {
	if excludeVal.Type == multitype.Bool {
		return excludeVal.BoolVal, nil
	}

	parsed, err := strconv.ParseBool(excludeVal.StrVal)
	if err != nil {
		return false, errors.Wrap(err, "failed to parse bool string")
	}

	return parsed, nil
}

func (c *Collector) RunCollectorSync() ([]byte, error) {
	if c.Collect.ClusterInfo != nil {
		isExcluded, err := isExcluded(c.Collect.ClusterInfo.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		return ClusterInfo(c.GetContext())
	}
	if c.Collect.ClusterResources != nil {
		isExcluded, err := isExcluded(c.Collect.ClusterResources.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		return ClusterResources(c.GetContext())
	}
	if c.Collect.Rook != nil {
		isExcluded, err := isExcluded(c.Collect.Rook.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		return Rook(c.GetContext(), c.Collect.Rook)
	}
	if c.Collect.Secret != nil {
		isExcluded, err := isExcluded(c.Collect.Secret.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		return Secret(c.GetContext(), c.Collect.Secret)
	}
	if c.Collect.Logs != nil {
		isExcluded, err := isExcluded(c.Collect.Logs.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		return Logs(c.GetContext(), c.Collect.Logs)
	}
	if c.Collect.Run != nil {
		isExcluded, err := isExcluded(c.Collect.Run.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		return Run(c.GetContext(), c.Collect.Run)
	}
	if c.Collect.Exec != nil {
		isExcluded, err := isExcluded(c.Collect.Exec.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		return Exec(c.GetContext(), c.Collect.Exec)
	}
	if c.Collect.Data != nil {
		isExcluded, err := isExcluded(c.Collect.Data.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		return Data(c.GetContext(), c.Collect.Data)
	}
	if c.Collect.Copy != nil {
		isExcluded, err := isExcluded(c.Collect.Copy.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		return Copy(c.GetContext(), c.Collect.Copy)
	}
	if c.Collect.HTTP != nil {
		isExcluded, err := isExcluded(c.Collect.HTTP.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
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
