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
	"k8s.io/klog/v2"
)

var _ Collector = &CollectClickHouse{}

type CollectClickHouse struct {
	Collector    *troubleshootv1beta2.Database
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectClickHouse) Title() string {
	return getCollectorName(c)
}

func (c *CollectClickHouse) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectClickHouse) createConnectConfig() (*clickhouse.Options, error) {
	if c.Collector.URI == "" {
		return nil, errors.New("clickhouse uri cannot be empty")
	}

	opts, err := clickhouse.ParseDSN(c.Collector.URI)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't parse clickhouse URI")
	}

	if c.Collector.TLS != nil {
		klog.V(2).Infof("Connecting to clickhouse with TLS client config")
		tlsCfg, err := createTLSConfig(c.Context, c.Client, c.Collector.TLS)
		if err != nil {
			return nil, err
		}
		opts.TLS = tlsCfg
	}
	klog.V(2).Infof("Successfully parsed clickhouse DSN from URI")

	return opts, nil
}

func (c *CollectClickHouse) connect() (driver.Conn, error) {
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

func (c *CollectClickHouse) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	databaseConnection := DatabaseConnection{}

	conn, err := c.connect()
	if err != nil {
		klog.V(2).Infof("ClickHouse connection error: %s", err.Error())
		databaseConnection.Error = err.Error()
	} else {
		klog.V(2).Infof("Successfully connected to ClickHouse")

		defer conn.Close()

		if err := conn.Ping(c.Context); err != nil {
			klog.V(2).Infof("ClickHouse ping error: %s", err.Error())
			databaseConnection.Error = err.Error()
		} else {
			var version string
			// ClickHouse version query to get major.minor.patch only
			// version() returns a string in the form: major_version.minor_version.patch_version.number_of_commits_since_the_previous_stable_release. This breaks the semver parsing in the analyzer.
			query := `SELECT arrayStringConcat(
					arraySlice(splitByChar('.', version()), 1, 3),
					'.'
				)`
			err := conn.QueryRow(c.Context, query).Scan(&version)
			if err != nil {
				databaseConnection.Version = "Unknown"
				databaseConnection.Error = err.Error()
			} else {
				databaseConnection.IsConnected = true
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
