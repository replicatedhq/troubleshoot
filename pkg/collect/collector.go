package collect

import (
	"context"
	"path/filepath"
	"strconv"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Collector struct {
	Collect      *troubleshootv1beta2.Collect
	Redact       bool
	RBACErrors   []error
	ClientConfig *rest.Config
	Namespace    string
	PathPrefix   string
}

type Collectors []*Collector

func isExcluded(excludeVal multitype.BoolOrString) (bool, error) {
	if excludeVal.Type == multitype.Bool {
		return excludeVal.BoolVal, nil
	}

	if excludeVal.StrVal == "" {
		return false, nil
	}

	parsed, err := strconv.ParseBool(excludeVal.StrVal)
	if err != nil {
		return false, errors.Wrap(err, "failed to parse bool string")
	}

	return parsed, nil
}

func (c *Collector) RunCollectorSync(globalRedactors []*troubleshootv1beta2.Redact) (map[string][]byte, error) {
	var unRedacted map[string][]byte
	var isExcludedResult bool
	var err error
	if c.Collect.ClusterInfo != nil {
		isExcludedResult, err = isExcluded(c.Collect.ClusterInfo.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcludedResult {
			return nil, nil
		}
		unRedacted, err = ClusterInfo(c)
	} else if c.Collect.ClusterResources != nil {
		isExcludedResult, err = isExcluded(c.Collect.ClusterResources.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcludedResult {
			return nil, nil
		}
		unRedacted, err = ClusterResources(c)
	} else if c.Collect.Secret != nil {
		isExcludedResult, err = isExcluded(c.Collect.Secret.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcludedResult {
			return nil, nil
		}
		unRedacted, err = Secret(c, c.Collect.Secret)
	} else if c.Collect.Logs != nil {
		isExcludedResult, err = isExcluded(c.Collect.Logs.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcludedResult {
			return nil, nil
		}
		unRedacted, err = Logs(c, c.Collect.Logs)
	} else if c.Collect.Run != nil {
		isExcludedResult, err = isExcluded(c.Collect.Run.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcludedResult {
			return nil, nil
		}
		unRedacted, err = Run(c, c.Collect.Run)
	} else if c.Collect.Exec != nil {
		isExcludedResult, err = isExcluded(c.Collect.Exec.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcludedResult {
			return nil, nil
		}
		unRedacted, err = Exec(c, c.Collect.Exec)
	} else if c.Collect.Data != nil {
		isExcludedResult, err = isExcluded(c.Collect.Data.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcludedResult {
			return nil, nil
		}
		unRedacted, err = Data(c, c.Collect.Data)
	} else if c.Collect.Copy != nil {
		isExcludedResult, err = isExcluded(c.Collect.Copy.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcludedResult {
			return nil, nil
		}
		unRedacted, err = Copy(c, c.Collect.Copy)
	} else if c.Collect.HTTP != nil {
		isExcludedResult, err = isExcluded(c.Collect.HTTP.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcludedResult {
			return nil, nil
		}
		unRedacted, err = HTTP(c, c.Collect.HTTP)
	} else if c.Collect.Postgres != nil {
		isExcludedResult, err = isExcluded(c.Collect.Postgres.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcludedResult {
			return nil, nil
		}
		unRedacted, err = Postgres(c, c.Collect.Postgres)
	} else if c.Collect.Mysql != nil {
		isExcludedResult, err = isExcluded(c.Collect.Mysql.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcludedResult {
			return nil, nil
		}
		unRedacted, err = Mysql(c, c.Collect.Mysql)
	} else if c.Collect.Redis != nil {
		isExcludedResult, err = isExcluded(c.Collect.Redis.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcludedResult {
			return nil, nil
		}
		unRedacted, err = Redis(c, c.Collect.Redis)
	} else {
		return nil, errors.New("no spec found to run")
	}
	if err != nil {
		return nil, err
	}

	if c.PathPrefix != "" {
		// prefix file paths
		prefixed := map[string][]byte{}
		for k, v := range unRedacted {
			prefixed[filepath.Join(c.PathPrefix, k)] = v
		}
		unRedacted = prefixed
	}

	if c.Redact {
		return redactMap(unRedacted, globalRedactors)
	}
	return unRedacted, nil
}

func (c *Collector) GetDisplayName() string {
	return c.Collect.GetName()
}

func (c *Collector) CheckRBAC(ctx context.Context) error {
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

		resp, err := client.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
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

func (cs Collectors) CheckRBAC(ctx context.Context) error {
	for _, c := range cs {
		if err := c.CheckRBAC(ctx); err != nil {
			return errors.Wrap(err, "failed to check RBAC")
		}
	}
	return nil
}
