package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var _ Collector = &CollectClickhouse{}

type CollectClickhouse struct {
	Collector    *troubleshootv1beta2.Database
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectClickhouse) Title() string {
	return getCollectorName(c)
}

func (c *CollectClickhouse) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectClickhouse) createConnectConfig() (*clickhouse.Options, error) {
	if c.Collector.URI == "" {
		return nil, errors.New("clickhouse uri cannot be empty")
	}

	opts, err := clickhouse.ParseDSN(c.Collector.URI)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't parse clickhouse URI")
	}

	if c.Collector.TLS != nil {
		tlsCfg, err := createTLSConfig(c.Context, c.Client, c.Collector.TLS)
		if err != nil {
			return nil, err
		}
		opts.TLS = tlsCfg
	}

	return opts, nil
}

func (c *CollectClickhouse) connect() (driver.Conn, error) {
	config, err := c.createConnectConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create clickhouse connection config")
	}

	conn, err := clickhouse.Open(config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open clickhouse connection")
	}

	return conn, nil
}

func (c *CollectClickhouse) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	databaseConnection := DatabaseConnection{}

	conn, err := c.connect()
	if err != nil {
		databaseConnection.Error = err.Error()
	} else {
		defer conn.Close()

		if err := conn.Ping(c.Context); err != nil {
			databaseConnection.Error = err.Error()
		} else {
			databaseConnection.IsConnected = true

			var version string
			err := conn.QueryRow(c.Context, "SELECT version()").Scan(&version)
			if err != nil {
				databaseConnection.Error = err.Error()
			} else {
				databaseConnection.Version = version
			}
		}
	}

	b, err := json.Marshal(databaseConnection)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal database connection")
	}

	collectorName := c.Collector.CollectorName
	if collectorName == "" {
		collectorName = "clickhouse"
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, fmt.Sprintf("clickhouse/%s.json", collectorName), bytes.NewBuffer(b))
	return output, nil
}
