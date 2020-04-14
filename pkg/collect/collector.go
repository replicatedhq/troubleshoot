package collect

import (
	"strconv"

	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
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

func (c *Collector) RunCollectorSync(globalRedactors []*troubleshootv1beta1.Redact) (map[string][]byte, error) {
	var unRedacted map[string][]byte
	var isExcludedResult bool
	var err error
	var redactors []*troubleshootv1beta1.Redact
	if c.Collect.ClusterInfo != nil {
		isExcludedResult, err = isExcluded(c.Collect.ClusterInfo.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcludedResult {
			return nil, nil
		}
		unRedacted, err = ClusterInfo(c.GetContext())
		redactors = c.Collect.ClusterInfo.Redactors
	} else if c.Collect.ClusterResources != nil {
		isExcludedResult, err = isExcluded(c.Collect.ClusterResources.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcludedResult {
			return nil, nil
		}
		unRedacted, err = ClusterResources(c.GetContext())
		redactors = c.Collect.ClusterResources.Redactors
	} else if c.Collect.Secret != nil {
		isExcludedResult, err = isExcluded(c.Collect.Secret.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcludedResult {
			return nil, nil
		}
		unRedacted, err = Secret(c.GetContext(), c.Collect.Secret)
		redactors = c.Collect.Secret.Redactors
	} else if c.Collect.Logs != nil {
		isExcludedResult, err = isExcluded(c.Collect.Logs.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcludedResult {
			return nil, nil
		}
		unRedacted, err = Logs(c.GetContext(), c.Collect.Logs)
		redactors = c.Collect.Logs.Redactors
	} else if c.Collect.Run != nil {
		isExcludedResult, err = isExcluded(c.Collect.Run.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcludedResult {
			return nil, nil
		}
		unRedacted, err = Run(c.GetContext(), c.Collect.Run)
		redactors = c.Collect.Run.Redactors
	} else if c.Collect.Exec != nil {
		isExcludedResult, err = isExcluded(c.Collect.Exec.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcludedResult {
			return nil, nil
		}
		unRedacted, err = Exec(c.GetContext(), c.Collect.Exec)
		redactors = c.Collect.Exec.Redactors
	} else if c.Collect.Data != nil {
		isExcludedResult, err = isExcluded(c.Collect.Data.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcludedResult {
			return nil, nil
		}
		unRedacted, err = Data(c.GetContext(), c.Collect.Data)
		redactors = c.Collect.Data.Redactors
	} else if c.Collect.Copy != nil {
		isExcludedResult, err = isExcluded(c.Collect.Copy.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcludedResult {
			return nil, nil
		}
		unRedacted, err = Copy(c.GetContext(), c.Collect.Copy)
		redactors = c.Collect.Copy.Redactors
	} else if c.Collect.HTTP != nil {
		isExcludedResult, err = isExcluded(c.Collect.HTTP.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcludedResult {
			return nil, nil
		}
		unRedacted, err = HTTP(c.GetContext(), c.Collect.HTTP)
		redactors = c.Collect.HTTP.Redactors
	} else if c.Collect.Postgres != nil {
		isExcludedResult, err = isExcluded(c.Collect.Postgres.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcludedResult {
			return nil, nil
		}
		unRedacted, err = Postgres(c.GetContext(), c.Collect.Postgres)
		redactors = c.Collect.Postgres.Redactors
	} else if c.Collect.Mysql != nil {
		isExcludedResult, err = isExcluded(c.Collect.Mysql.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcludedResult {
			return nil, nil
		}
		unRedacted, err = Mysql(c.GetContext(), c.Collect.Mysql)
		redactors = c.Collect.Mysql.Redactors
	} else if c.Collect.Redis != nil {
		isExcludedResult, err = isExcluded(c.Collect.Redis.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcludedResult {
			return nil, nil
		}
		unRedacted, err = Redis(c.GetContext(), c.Collect.Redis)
		redactors = c.Collect.Redis.Redactors
	} else {
		return nil, errors.New("no spec found to run")
	}

	if err != nil {
		return nil, err
	}
	if c.Redact {
		return redactMap(unRedacted, append(redactors, globalRedactors...))
	}
	return unRedacted, nil
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
