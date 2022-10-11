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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type CollectPostgres struct {
	Collector    *troubleshootv1beta2.Database
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectPostgres) Title() string {
	return collectorTitleOrDefault(c.Collector.CollectorMeta, "Postgres")
}

func (c *CollectPostgres) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
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
