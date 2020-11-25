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

// checks if a given collector has a spec with 'exclude' that evaluates to true.
func (c *Collector) IsExcluded() bool {
	if c.Collect.ClusterInfo != nil {
		isExcludedResult, err := isExcluded(c.Collect.ClusterInfo.Exclude)
		if err != nil {
			return true
		}
		if isExcludedResult {
			return true
		}
	} else if c.Collect.ClusterResources != nil {
		isExcludedResult, err := isExcluded(c.Collect.ClusterResources.Exclude)
		if err != nil {
			return true
		}
		if isExcludedResult {
			return true
		}
	} else if c.Collect.Secret != nil {
		isExcludedResult, err := isExcluded(c.Collect.Secret.Exclude)
		if err != nil {
			return true
		}
		if isExcludedResult {
			return true
		}
	} else if c.Collect.Logs != nil {
		isExcludedResult, err := isExcluded(c.Collect.Logs.Exclude)
		if err != nil {
			return true
		}
		if isExcludedResult {
			return true
		}
	} else if c.Collect.Run != nil {
		isExcludedResult, err := isExcluded(c.Collect.Run.Exclude)
		if err != nil {
			return true
		}
		if isExcludedResult {
			return true
		}
	} else if c.Collect.Exec != nil {
		isExcludedResult, err := isExcluded(c.Collect.Exec.Exclude)
		if err != nil {
			return true
		}
		if isExcludedResult {
			return true
		}
	} else if c.Collect.Data != nil {
		isExcludedResult, err := isExcluded(c.Collect.Data.Exclude)
		if err != nil {
			return true
		}
		if isExcludedResult {
			return true
		}
	} else if c.Collect.Copy != nil {
		isExcludedResult, err := isExcluded(c.Collect.Copy.Exclude)
		if err != nil {
			return true
		}
		if isExcludedResult {
			return true
		}
	} else if c.Collect.HTTP != nil {
		isExcludedResult, err := isExcluded(c.Collect.HTTP.Exclude)
		if err != nil {
			return true
		}
		if isExcludedResult {
			return true
		}
	} else if c.Collect.Postgres != nil {
		isExcludedResult, err := isExcluded(c.Collect.Postgres.Exclude)
		if err != nil {
			return true
		}
		if isExcludedResult {
			return true
		}
	} else if c.Collect.Mysql != nil {
		isExcludedResult, err := isExcluded(c.Collect.Mysql.Exclude)
		if err != nil {
			return true
		}
		if isExcludedResult {
			return true
		}
	} else if c.Collect.Redis != nil {
		isExcludedResult, err := isExcluded(c.Collect.Redis.Exclude)
		if err != nil {
			return true
		}
		if isExcludedResult {
			return true
		}
	} else if c.Collect.Collectd != nil {
		// TODO: see if redaction breaks these
		isExcludedResult, err := isExcluded(c.Collect.Collectd.Exclude)
		if err != nil {
			return true
		}
		if isExcludedResult {
			return true
		}
	} else if c.Collect.Ceph != nil {
		isExcludedResult, err := isExcluded(c.Collect.Ceph.Exclude)
		if err != nil {
			return true
		}
		if isExcludedResult {
			return true
		}
	}
	return false
}

func (c *Collector) RunCollectorSync(globalRedactors []*troubleshootv1beta2.Redact, analyzers []*troubleshootv1beta2.Analyze, progressChan chan interface{}) (result map[string][]byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.Errorf("recovered from panic: %v", r)
		}
	}()

	if c.IsExcluded() {
		return
	}

	if c.Collect.ClusterInfo != nil {
		result, err = ClusterInfo(c)
	} else if c.Collect.ClusterResources != nil {
		errs := make(map[string]error)
		result, errs = ClusterResources(c, analyzers)
		if errs != nil {
			collector, isRequired := isCollectorRequired(errs, analyzers)
			if isRequired {
				return result, errs[collector]
			}
			for collector, err := range errs {
				progressChan <- errors.Errorf("default collector %s failed: %v", collector, err)
			}
		}
	} else if c.Collect.Secret != nil {
		result, err = Secret(c, c.Collect.Secret)
	} else if c.Collect.Logs != nil {
		result, err = Logs(c, c.Collect.Logs)
	} else if c.Collect.Run != nil {
		result, err = Run(c, c.Collect.Run)
	} else if c.Collect.Exec != nil {
		result, err = Exec(c, c.Collect.Exec)
	} else if c.Collect.Data != nil {
		result, err = Data(c, c.Collect.Data)
	} else if c.Collect.Copy != nil {
		result, err = Copy(c, c.Collect.Copy)
	} else if c.Collect.HTTP != nil {
		result, err = HTTP(c, c.Collect.HTTP)
	} else if c.Collect.Postgres != nil {
		result, err = Postgres(c, c.Collect.Postgres)
	} else if c.Collect.Mysql != nil {
		result, err = Mysql(c, c.Collect.Mysql)
	} else if c.Collect.Redis != nil {
		result, err = Redis(c, c.Collect.Redis)
	} else if c.Collect.Collectd != nil {
		// TODO: see if redaction breaks these
		result, err = Collectd(c, c.Collect.Collectd)
	} else if c.Collect.Ceph != nil {
		result, err = Ceph(c, c.Collect.Ceph)
	} else {
		err = errors.New("no spec found to run")
		return
	}
	if err != nil {
		return
	}

	if c.PathPrefix != "" {
		// prefix file paths
		prefixed := map[string][]byte{}
		for k, v := range result {
			prefixed[filepath.Join(c.PathPrefix, k)] = v
		}
		result = prefixed
	}

	if c.Redact {
		result, err = redactMap(result, globalRedactors)
	}

	return
}

func (c *Collector) GetDisplayName() string {
	return c.Collect.GetName()
}

func (c *Collector) CheckRBAC(ctx context.Context) error {
	if c.IsExcluded() {
		return nil // excluded collectors require no permissions
	}

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

func isCollectorRequired(errs map[string]error, analyzers []*troubleshootv1beta2.Analyze) (string, bool) {
	if _, ok := errs["client"]; ok {
		return "client", true
	}
	if analyzers != nil {
		for _, analyzer := range analyzers {
			if _, ok := errs["namespace"]; ok && analyzer.ContainerRuntime != nil {
				return "namespace", true
			} else if _, ok := errs["deployments"]; ok && analyzer.DeploymentStatus != nil {
				return "deployments", true
			} else if _, ok := errs["statefulsets"]; ok && analyzer.StatefulsetStatus != nil {
				return "statefulsets", true
			} else if _, ok := errs["ingress"]; ok && analyzer.Ingress != nil {
				return "ingress", true
			} else if _, ok := errs["storageclasses"]; ok && analyzer.StorageClass != nil {
				return "storageclasses", true
			} else if _, ok := errs["crds"]; ok && analyzer.CustomResourceDefinition != nil {
				return "crds", true
			} else if _, ok := errs["imagepullsecrets"]; ok && analyzer.ImagePullSecret != nil {
				return "imagepullsecrets", true
			} else if _, ok := errs["nodes"]; ok && (analyzer.NodeResources != nil || analyzer.Distribution != nil) {
				return "nodes", true
			} else if _, ok := errs["apiresources"]; ok && analyzer.Distribution != nil {
				return "apiresources", true
			}
		}
	}
	return "", false
}
