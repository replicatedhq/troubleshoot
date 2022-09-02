package collect

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"

	_ "github.com/lib/pq"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type CollectPostgres struct {
	Collector    *troubleshootv1beta2.Database
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	ctx          context.Context
	RBACErrors   []error
}

func (c *CollectPostgres) Title() string {
	return collectorTitleOrDefault(c.Collector.CollectorMeta, "Postgres")
}

func (c *CollectPostgres) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectPostgres) CheckRBAC(ctx context.Context, collector *troubleshootv1beta2.Collect) error {
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

func (c *CollectPostgres) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	databaseConnection := DatabaseConnection{}

	db, err := sql.Open("postgres", c.Collector.URI)
	if err != nil {
		databaseConnection.Error = err.Error()
	} else {
		query := `select version()`
		row := db.QueryRow(query)
		version := ""
		if err := row.Scan(&version); err != nil {
			databaseConnection.Error = err.Error()
		} else {
			databaseConnection.IsConnected = true

			postgresVersion, err := parsePostgresVersion(version)
			if err != nil {
				databaseConnection.Version = "Unknown"
				databaseConnection.Error = err.Error()
			} else {
				databaseConnection.Version = postgresVersion
			}
		}
	}

	b, err := json.Marshal(databaseConnection)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal database connection")
	}

	collectorName := c.Collector.CollectorName
	if collectorName == "" {
		collectorName = "postgres"
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, fmt.Sprintf("postgres/%s.json", collectorName), bytes.NewBuffer(b))

	return output, nil
}

func parsePostgresVersion(postgresVersion string) (string, error) {
	re := regexp.MustCompile("PostgreSQL ([0-9.]*)")
	matches := re.FindStringSubmatch(postgresVersion)
	if len(matches) < 2 {
		return "", errors.Errorf("postgres version did not match regex: %q", postgresVersion)
	}

	return matches[1], nil

}
