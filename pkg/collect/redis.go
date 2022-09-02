package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-redis/redis/v7"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type CollectRedis struct {
	Collector    *troubleshootv1beta2.Database
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	ctx          context.Context
	RBACErrors   []error
}

func (c *CollectRedis) Title() string {
	return collectorTitleOrDefault(c.Collector.CollectorMeta, "Cluster Info")
}

func (c *CollectRedis) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectRedis) CheckRBAC(ctx context.Context, collector *troubleshootv1beta2.Collect) error {
	exclude, err := c.IsExcluded()
	if err != nil || exclude != true {
		return nil
	}

	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return errors.Wrap(err, "failed to create client from config")
	}

	forbidden := make([]error, 0)

	specs := collector.AccessReviewSpecs(c.Namespace)
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
				DisplayName: c.Title(),
				Namespace:   spec.ResourceAttributes.Namespace,
				Resource:    spec.ResourceAttributes.Resource,
				Verb:        spec.ResourceAttributes.Verb,
			})
		}
	}
	c.RBACErrors = forbidden

	return nil
}

func (c *CollectRedis) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	databaseConnection := DatabaseConnection{}

	opt, err := redis.ParseURL(c.Collector.URI)
	if err != nil {
		databaseConnection.Error = err.Error()
	} else {
		client := redis.NewClient(opt)
		stringResult := client.Info("server")

		if stringResult.Err() != nil {
			databaseConnection.Error = stringResult.Err().Error()
		}

		databaseConnection.IsConnected = stringResult.Err() == nil

		if databaseConnection.Error == "" {
			lines := strings.Split(stringResult.Val(), "\n")
			for _, line := range lines {
				lineParts := strings.Split(line, ":")
				if len(lineParts) == 2 {
					if lineParts[0] == "redis_version" {
						databaseConnection.Version = strings.TrimSpace(lineParts[1])
					}
				}
			}
		}
	}

	b, err := json.Marshal(databaseConnection)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal database connection")
	}

	collectorName := c.Collector.CollectorName
	if collectorName == "" {
		collectorName = "redis"
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, fmt.Sprintf("redis/%s.json", collectorName), bytes.NewBuffer(b))

	return output, nil
}
