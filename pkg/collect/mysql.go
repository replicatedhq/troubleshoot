package collect

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
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

func (c *CollectMysql) createConnectConfig() (*mysql.Config, error) {
	if c.Collector.URI == "" {
		return nil, errors.New("mysql uri cannot be empty")
	}

	cfg, err := mysql.ParseDSN(c.Collector.URI)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse mysql config")
	}

	if c.Collector.TLS != nil {
		klog.V(2).Infof("Connecting to mysql with TLS client config")
		tlsConfig, err := createTLSConfig(c.Context, c.Client, c.Collector.TLS)
		if err != nil {
			return nil, err
		}
		cfg.TLS = tlsConfig
	}

	return cfg, nil
}

func (c *CollectMysql) connect() (*sql.DB, error) {
	cfg, err := c.createConnectConfig()
	if err != nil {
		return nil, err
	}

	if c.Collector.TLS != nil {
		connector, err := mysql.NewConnector(cfg)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create mysql connector")
		}

		db := sql.OpenDB(connector)
		return db, nil
	}

	db, err := sql.Open("mysql", c.Collector.URI)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func (c *CollectMysql) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	databaseConnection := DatabaseConnection{}

	db, err := c.connect()
	if err != nil {
		klog.V(2).Infof("MySQL connection error: %s", err.Error())
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
