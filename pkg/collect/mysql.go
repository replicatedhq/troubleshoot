package collect

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type CollectMysql struct {
	Collector    *troubleshootv1beta2.Database
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	ctx          context.Context
	RBACErrors   []error
}

func (c *CollectMysql) Title() string {
	return collectorTitleOrDefault(c.Collector.CollectorMeta, "Mysql")
}

func (c *CollectMysql) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectMysql) GetRBACErrors() []error {
	return c.RBACErrors
}

func (c *CollectMysql) HasRBACErrors() bool {
	return len(c.RBACErrors) > 0
}

func (c *CollectMysql) CheckRBAC(ctx context.Context, collector *troubleshootv1beta2.Collect) error {
	exclude, err := c.IsExcluded()
	if err != nil || exclude != true {
		return nil
	}

	rbacErrors, err := checkRBAC(ctx, c.ClientConfig, c.Namespace, c.Title(), collector)
	if err != nil {
		return err
	}

	c.RBACErrors = rbacErrors

	return nil
}

func (c *CollectMysql) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	databaseConnection := DatabaseConnection{}

	db, err := sql.Open("mysql", c.Collector.URI)
	if err != nil {
		databaseConnection.Error = err.Error()
	} else {
		defer db.Close()
		query := `select version()`
		row := db.QueryRow(query)

		version := ""
		if err := row.Scan(&version); err != nil {
			databaseConnection.Error = err.Error()
		} else {
			databaseConnection.IsConnected = true
			databaseConnection.Version = version
		}

		requestedParameters := c.Collector.Parameters
		if len(requestedParameters) > 0 {
			rows, err := db.Query("SHOW VARIABLES")

			if err != nil {
				databaseConnection.Error = err.Error()
			} else {
				defer rows.Close()

				variables := map[string]string{}
				for rows.Next() {
					var key, value string
					err = rows.Scan(&key, &value)
					if err != nil {
						databaseConnection.Error = err.Error()
						break
					}
					variables[key] = value
				}
				filteredVariables := map[string]string{}

				for _, key := range requestedParameters {
					if value, ok := variables[key]; ok {
						filteredVariables[key] = value
					}

				}
				databaseConnection.Variables = filteredVariables
			}
		}

	}

	b, err := json.Marshal(databaseConnection)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal database connection")
	}

	collectorName := c.Collector.CollectorName
	if collectorName == "" {
		collectorName = "mysql"
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, fmt.Sprintf("mysql/%s.json", collectorName), bytes.NewBuffer(b))

	return output, nil
}
