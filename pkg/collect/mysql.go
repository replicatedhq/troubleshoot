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
	Context      context.Context
	RBACErrors
}

func (c *CollectMysql) Title() string {
	return getCollectorName(c)
}

func (c *CollectMysql) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectMysql) SkipRedaction() bool {
	return c.Collector.SkipRedaction
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
